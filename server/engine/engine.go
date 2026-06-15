// Package engine is the agent loop — the stateless core. It composes the prompt from
// a spec, resolves tool names into definitions (for the model) + executors (for the
// harness), then runs reason->act until the brain stops. It touches no HTTP, DB, or
// secrets; everything is injected. It emits a stream of normalized events.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"agent-builder/brain"
	"agent-builder/logx"
	"agent-builder/tools"
)

// MaxIterations caps the reason->act loop so a misbehaving model cannot spin forever
// (00-CONTEXT.md §11: runaway tool loop).
const MaxIterations = 10

// Spec is the part of an agent row the engine needs to arrange a prompt. (The full
// row lives in the store; the engine only ever sees this.)
type Spec struct {
	Persona      string
	Instructions string
	Tools        []string // tool names; resolved against the registry
}

// Input bundles everything a run needs. All dependencies are injected — the engine
// constructs nothing that touches the world.
type Input struct {
	InvocationID string // audit/trace key for this turn (minted by the caller)
	Brain        brain.Brain
	Registry     *tools.Registry
	Spec         Spec
	History      []brain.Message // prior turns (the resume); may be empty
	Message      string          // the new user message
	Emit         Emit            // optional; nil-safe
	Logger       *slog.Logger    // optional; nil -> no logging
	// Confirm, if set, gates consequential tools. It is called BEFORE the confirm
	// event is emitted and returns a wait func that the engine calls AFTER emitting,
	// to block for the decision. This two-phase shape lets the implementation register
	// its wait state before the event reaches the client — so an approval can never
	// race ahead of registration. A false decision means denied (the engine feeds a
	// "denied" result back and continues). If nil, consequential tools run ungated.
	Confirm func(ctx context.Context, call brain.ToolCall) (wait func() (bool, error), err error)
	// AskUser, if set, gates tools that request structured user input. The runtime
	// registers the waiter and returns the question event payload plus a wait func.
	AskUser func(ctx context.Context, call brain.ToolCall) (question Event, wait func() (string, error), err error)
}

// Output is the result of a completed turn.
type Output struct {
	History   []brain.Message // History + this turn's messages, ready to persist
	FinalText string          // the assistant's final answer
}

// Run executes one turn. It returns when the brain produces a final answer, the loop
// cap is hit, the context is cancelled, or an unrecoverable error occurs. Tool
// execution errors are NOT unrecoverable — they are fed back to the model.
func Run(ctx context.Context, in Input) (Output, error) {
	log := in.Logger
	if log == nil {
		log = logx.Discard()
	}
	log = log.With("invocation_id", in.InvocationID)
	emit := in.Emit
	if emit == nil {
		emit = func(Event) {}
	}

	seq := 0
	next := func(e Event) Event { e.Seq = seq; seq++; return e }
	isBuilderRun := hasTool(in.Spec.Tools, "create_agent")
	status := func(message string) {
		log.Info("engine.status", "message", message)
		emit(next(Event{Type: EventStatus, Message: message}))
	}
	runStatus := func(normalMessage, builderMessage string) {
		if isBuilderRun && builderMessage != "" {
			status(builderMessage)
			return
		}
		status(normalMessage)
	}
	normalStatus := func(message string) {
		if !isBuilderRun {
			status(message)
		}
	}

	// (1) compose prompt — stack the spec layers in stable order.
	runStatus("Preparing response", "Preparing agent brief")
	system := composePrompt(in.Spec)

	// (2) resolve tools — names -> definitions (to the model) + executors (kept here).
	status("Checking available tools")
	defs, execs, err := resolve(in.Registry, in.Spec.Tools)
	if err != nil {
		emit(next(Event{Type: EventError, Message: err.Error()}))
		return Output{}, err
	}
	log.Info("engine.tools resolved", "tools", toolNames(defs))

	history := append([]brain.Message(nil), in.History...)
	history = append(history, brain.Message{Role: brain.RoleUser, Content: in.Message})

	start := time.Now()
	log.Info("engine.run start", "tools", len(defs), "history_len", len(in.History), "message_len", len(in.Message))

	// (3) the reason->act loop.
	for iter := 1; ; iter++ {
		if err := ctx.Err(); err != nil {
			log.Warn("engine.run cancelled", "iter", iter)
			emit(next(Event{Type: EventAborted}))
			return Output{History: history}, err
		}
		if iter > MaxIterations {
			msg := fmt.Sprintf("exceeded max iterations (%d)", MaxIterations)
			log.Warn("engine.run loop cap hit", "max", MaxIterations)
			emit(next(Event{Type: EventError, Message: msg}))
			return Output{History: history}, fmt.Errorf("engine: %s", msg)
		}

		if iter == 1 {
			runStatus("Thinking", "Drafting agent spec")
		} else {
			runStatus("Drafting final response", "Drafting agent spec")
		}
		brainStart := time.Now()
		log.Info("engine.brain start", "iter", iter, "history_len", len(history), "tools", len(defs))
		step, err := in.Brain.Next(ctx, system, defs, history, func(text string) {
			if text != "" {
				emit(next(Event{Type: EventLLMText, Text: text}))
			}
		})
		brainDur := time.Since(brainStart).Milliseconds()
		if err != nil {
			if ctx.Err() != nil {
				log.Warn("engine.run cancelled during brain call", "iter", iter, "duration_ms", brainDur)
				emit(next(Event{Type: EventAborted}))
				return Output{History: history}, ctx.Err()
			}
			log.Error("engine.run brain error", "iter", iter, "duration_ms", brainDur, "err", err)
			emit(next(Event{Type: EventError, Message: err.Error()}))
			return Output{History: history}, fmt.Errorf("engine: brain: %w", err)
		}
		log.Info("engine.brain done",
			"iter", iter,
			"duration_ms", brainDur,
			"step", stepKindName(step.Kind),
			"text_len", len(step.Text),
			"tool_calls", toolCallNames(step.ToolCalls),
		)

		// Final answer: no tool calls. The brain may already have streamed llm_text
		// deltas; done.Text remains the authoritative final text (00-CONTEXT §7).
		if step.Kind == brain.StepText || len(step.ToolCalls) == 0 {
			history = append(history, brain.Message{Role: brain.RoleAssistant, Content: step.Text})
			emit(next(Event{Type: EventDone, Text: step.Text}))
			log.Info("engine.run done", "iters", iter, "duration_ms", time.Since(start).Milliseconds())
			return Output{History: history, FinalText: step.Text}, nil
		}

		// Tool turn: record the assistant's request, then run each tool.
		if step.Text != "" && !step.TextStreamed {
			emit(next(Event{Type: EventLLMText, Text: step.Text}))
		}
		history = append(history, brain.Message{
			Role:      brain.RoleAssistant,
			Content:   step.Text,
			ToolCalls: step.ToolCalls,
		})

		for _, call := range step.ToolCalls {
			log.Info("engine.tool requested", "tool", call.Name, "call_id", call.ID, "args_bytes", len(call.Args))
			tool, found := execs[call.Name]
			if !found {
				// Should not happen (resolve validated names), but be defensive.
				log.Warn("engine.tool unknown", "tool", call.Name, "call_id", call.ID)
				appendResult(emit, next, &history, call, "error: unknown tool "+call.Name, false)
				continue
			}

			if tool.UserInput {
				if in.AskUser == nil {
					log.Warn("engine.user_question unavailable", "tool", call.Name, "call_id", call.ID)
					appendResult(emit, next, &history, call, "error: user input unavailable", false)
					continue
				}
				status("Waiting for answer")
				event, wait, err := in.AskUser(ctx, call)
				if err != nil {
					log.Error("engine.user_question register failed", "tool", call.Name, "call_id", call.ID, "err", err)
					emit(next(Event{Type: EventError, Message: err.Error()}))
					return Output{History: history}, err
				}
				event.Type = EventUserQuestion
				event.CallID = call.ID
				event.Name = call.Name
				log.Info("engine.user_question emit", "tool", call.Name, "call_id", call.ID, "field", event.Field, "options", len(event.Options), "allow_custom", event.AllowCustom)
				emit(next(event))
				log.Info("engine.user_question wait", "tool", call.Name, "call_id", call.ID)
				result, err := wait()
				if err != nil {
					log.Warn("engine.run cancelled awaiting user answer", "tool", call.Name, "call_id", call.ID)
					emit(next(Event{Type: EventAborted}))
					return Output{History: history}, err
				}
				log.Info("engine.user_question answered", "tool", call.Name, "call_id", call.ID, "bytes", len(result))
				appendResult(emit, next, &history, call, result, true)
				normalStatus("Reading tool result")
				continue
			}

			if !isBuilderRun || call.Name != "create_agent" {
				status("Calling " + call.Name)
			}
			emit(next(Event{Type: EventToolUse, CallID: call.ID, Name: call.Name, Args: call.Args}))

			// Confirmation gate for consequential tools (00-CONTEXT §8). Register the
			// wait state FIRST, then emit the confirm event, then block — so an
			// approval can never arrive before the waiter is registered.
			if tool.Consequential && in.Confirm != nil {
				wait, err := in.Confirm(ctx, call)
				if err != nil {
					emit(next(Event{Type: EventError, Message: err.Error()}))
					return Output{History: history}, err
				}
				status("Waiting for approval")
				emit(next(Event{Type: EventConfirm, CallID: call.ID, Name: call.Name, Args: call.Args}))
				approved, err := wait()
				if err != nil {
					log.Warn("engine.run cancelled awaiting confirmation", "tool", call.Name, "call_id", call.ID)
					emit(next(Event{Type: EventAborted}))
					return Output{History: history}, err
				}
				log.Info("engine.confirm answered", "tool", call.Name, "call_id", call.ID, "approved", approved)
				if !approved {
					log.Info("engine.tool denied", "tool", call.Name, "call_id", call.ID)
					appendResult(emit, next, &history, call, "denied by user", false)
					normalStatus("Reading tool result")
					continue
				}
			}

			if call.Name == "create_agent" {
				status("Creating agent")
			}
			result, ok := executeTool(ctx, tool, call, log)
			if call.Name == "create_agent" && ok {
				status("Saving agent")
			}
			appendResult(emit, next, &history, call, result, ok)
			normalStatus("Reading tool result")
		}
	}
}

// appendResult emits a tool_result event and records the matching tool message.
func appendResult(emit Emit, next func(Event) Event, history *[]brain.Message, call brain.ToolCall, result string, ok bool) {
	emit(next(Event{Type: EventToolResult, CallID: call.ID, Name: call.Name, OK: boolPtr(ok), Data: result}))
	*history = append(*history, brain.Message{
		Role:       brain.RoleTool,
		ToolCallID: call.ID,
		Name:       call.Name,
		Content:    result,
	})
}

// executeTool runs one tool. An executor error is returned as a failed result
// (ok=false) so the loop continues and the model can recover — never fatal.
func executeTool(ctx context.Context, tool tools.Tool, call brain.ToolCall, log *slog.Logger) (result string, ok bool) {
	start := time.Now()
	out, err := safeExecute(ctx, tool, call.Args)
	dur := time.Since(start).Milliseconds()
	if err != nil {
		log.Warn("engine.tool error", "tool", call.Name, "call_id", call.ID, "duration_ms", dur, "err", err)
		return "error: " + err.Error(), false
	}
	log.Info("engine.tool ok", "tool", call.Name, "call_id", call.ID, "duration_ms", dur, "bytes", len(out))
	return out, true
}

// safeExecute runs the executor and converts a panic into an error, so one buggy tool
// cannot take down the whole run.
func safeExecute(ctx context.Context, tool tools.Tool, args []byte) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("tool panicked: %v", r)
		}
	}()
	return tool.Execute(ctx, args)
}

// composePrompt stacks the spec layers in a fixed, stable order (static-first so a
// future cache prefix is stable). Phase 1 is intentionally simple.
func composePrompt(s Spec) string {
	var b strings.Builder
	if s.Persona != "" {
		b.WriteString(s.Persona)
		b.WriteString("\n\n")
	}
	if s.Instructions != "" {
		b.WriteString(s.Instructions)
		b.WriteString("\n\n")
	}
	if len(s.Tools) > 0 {
		b.WriteString("You may call the provided tools when they help. Use a tool only when needed.")
	}
	return strings.TrimSpace(b.String())
}

// resolve turns tool names into the model-facing definitions and the harness-side
// executors. An unknown name is a spec/wiring error and fails the run up front.
func resolve(reg *tools.Registry, names []string) ([]brain.ToolDef, map[string]tools.Tool, error) {
	defs := make([]brain.ToolDef, 0, len(names))
	execs := make(map[string]tools.Tool, len(names))
	for _, name := range names {
		t, found := reg.Get(name)
		if !found {
			return nil, nil, fmt.Errorf("engine: spec names unknown tool %q", name)
		}
		defs = append(defs, brain.ToolDef{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
		execs[name] = t
	}
	return defs, execs, nil
}

func hasTool(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func toolNames(defs []brain.ToolDef) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	return names
}

func toolCallNames(calls []brain.ToolCall) []string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		names = append(names, call.Name)
	}
	return names
}

func stepKindName(kind brain.StepKind) string {
	switch kind {
	case brain.StepText:
		return "text"
	case brain.StepToolCalls:
		return "tool_calls"
	default:
		return fmt.Sprintf("unknown:%d", kind)
	}
}
