package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// OpenAIBrain is the real Brain, backed by OpenAI chat-completions. It translates our
// neutral, persistable Message/ToolDef types into the provider's params on every call,
// and normalizes the response back into a Step. All OpenAI-specific shapes live HERE
// and nowhere else — that is the whole point of the Brain seam.
//
// The real implementation streams provider text deltas through the Brain callback
// while still returning one normalized Step after the provider response completes.
type OpenAIBrain struct {
	client openai.Client
	model  string
	log    *slog.Logger
	// timeout bounds one provider call. Zero means "use the caller's context as-is".
	timeout time.Duration
}

// NewOpenAIBrain builds a brain. The API key and model are injected (the brain never
// reads the environment itself — that is the orchestrator's job). Extra request options
// (e.g. a tracing middleware) are appended, keeping this package free of any
// observability dependency.
func NewOpenAIBrain(apiKey, model string, opts ...option.RequestOption) *OpenAIBrain {
	clientOpts := append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)
	return &OpenAIBrain{
		client: openai.NewClient(clientOpts...),
		model:  model,
		log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// SetLogger attaches structured diagnostics to provider calls.
func (o *OpenAIBrain) SetLogger(log *slog.Logger) {
	if log != nil {
		o.log = log
	}
}

// SetTimeout bounds a single provider call. Pass zero to disable the brain-local
// timeout and rely only on the request context.
func (o *OpenAIBrain) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// Next runs one assistant turn.
func (o *OpenAIBrain) Next(ctx context.Context, system string, tools []ToolDef, history []Message, onTextDelta func(string)) (Step, error) {
	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(o.model),
		Messages: toParamMessages(system, history),
	}
	if defs := toToolParams(tools); len(defs) > 0 {
		params.Tools = defs
	}

	callCtx := ctx
	var cancel context.CancelFunc
	if o.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, o.timeout)
		defer cancel()
	}

	start := time.Now()
	log := o.log
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	log.Info("openai.chat start",
		"model", o.model,
		"timeout_ms", o.timeout.Milliseconds(),
		"tools", len(tools),
		"history_len", len(history),
		"messages", len(params.Messages),
	)
	stream := o.client.Chat.Completions.NewStreaming(callCtx, params)
	defer stream.Close()
	var acc openai.ChatCompletionAccumulator
	textStreamed := false
	for stream.Next() {
		chunk := stream.Current()
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" && onTextDelta != nil {
				onTextDelta(choice.Delta.Content)
				textStreamed = true
			}
		}
		if !acc.AddChunk(chunk) {
			dur := time.Since(start).Milliseconds()
			log.Error("openai.chat stream accumulation error", "duration_ms", dur)
			return Step{}, fmt.Errorf("openai: could not accumulate streaming response")
		}
	}
	err := stream.Err()
	dur := time.Since(start).Milliseconds()
	if err != nil {
		log.Error("openai.chat stream error", "duration_ms", dur, "err", err)
		return Step{}, fmt.Errorf("openai: %w", err)
	}
	completion := &acc.ChatCompletion
	if len(completion.Choices) == 0 {
		log.Error("openai.chat empty choices", "duration_ms", dur)
		return Step{}, fmt.Errorf("openai: response had no choices")
	}
	msg := completion.Choices[0].Message

	var calls []ToolCall
	for _, tc := range msg.ToolCalls {
		// Narrow the tool-call union: only function calls are executable here.
		// (Mirrors the "function" guard; a "custom" tool variant is skipped.)
		if tc.Type != "function" {
			continue
		}
		args := tc.Function.Arguments
		if args == "" {
			args = "{}" // a no-arg call still needs valid JSON for the executor
		}
		calls = append(calls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: json.RawMessage(args),
		})
	}
	log.Info("openai.chat done",
		"duration_ms", dur,
		"choices", len(completion.Choices),
		"finish_reason", completion.Choices[0].FinishReason,
		"tool_calls", len(calls),
		"content_len", len(msg.Content),
		"text_streamed", textStreamed,
	)
	if len(calls) > 0 {
		return Step{Kind: StepToolCalls, Text: msg.Content, TextStreamed: textStreamed, ToolCalls: calls}, nil
	}
	return Step{Kind: StepText, Text: msg.Content, TextStreamed: textStreamed}, nil
}

// toToolParams converts our neutral ToolDefs into OpenAI function tools with strict
// schema adherence on (first line of tool-arg validation; the executor's typed
// unmarshal is the second).
func toToolParams(tools []ToolDef) []openai.ChatCompletionToolUnionParam {
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, d := range tools {
		var schema map[string]any
		if len(d.Parameters) > 0 {
			_ = json.Unmarshal(d.Parameters, &schema)
		}
		out = append(out, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        d.Name,
			Description: openai.String(d.Description),
			Parameters:  openai.FunctionParameters(schema),
			Strict:      openai.Bool(true),
		}))
	}
	return out
}

// toParamMessages rebuilds the provider message list from the system prompt + our
// neutral history on every call (the engine owns history; the brain is stateless).
func toParamMessages(system string, history []Message) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(history)+1)
	if system != "" {
		out = append(out, openai.SystemMessage(system))
	}
	for _, m := range history {
		switch m.Role {
		case RoleUser:
			out = append(out, openai.UserMessage(m.Content))
		case RoleTool:
			out = append(out, openai.ToolMessage(m.Content, m.ToolCallID))
		case RoleAssistant:
			if len(m.ToolCalls) == 0 {
				out = append(out, openai.AssistantMessage(m.Content))
				continue
			}
			am := openai.ChatCompletionAssistantMessageParam{}
			if m.Content != "" {
				am.Content.OfString = openai.String(m.Content)
			}
			for _, tc := range m.ToolCalls {
				am.ToolCalls = append(am.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: string(tc.Args),
						},
					},
				})
			}
			out = append(out, openai.ChatCompletionMessageParamUnion{OfAssistant: &am})
		}
	}
	return out
}
