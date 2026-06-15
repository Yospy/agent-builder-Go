package brain

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

// TitleSummarizer turns a user's first prompt into a compact sidebar label.
type TitleSummarizer interface {
	SummarizeTitle(ctx context.Context, prompt string) (string, error)
}

type OpenAITitleSummarizer struct {
	client openai.Client
	model  string
}

func NewOpenAITitleSummarizer(apiKey, model string, opts ...option.RequestOption) *OpenAITitleSummarizer {
	clientOpts := append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)
	return &OpenAITitleSummarizer{
		client: openai.NewClient(clientOpts...),
		model:  model,
	}
}

func (o *OpenAITitleSummarizer) SummarizeTitle(ctx context.Context, prompt string) (string, error) {
	resp, err := o.client.Responses.New(ctx, responses.ResponseNewParams{
		Model: shared.ResponsesModel(o.model),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(prompt),
		},
		Instructions: openai.String(strings.TrimSpace(`
Create a concise chat title from the user's prompt.
Return only 2 to 3 plain words.
No punctuation, quotes, emoji, labels, or trailing period.
Prefer concrete nouns and verbs over generic words.
`)),
		MaxOutputTokens: openai.Int(18),
		Reasoning: shared.ReasoningParam{
			Effort: shared.ReasoningEffortNone,
		},
		Store:       openai.Bool(false),
		Temperature: openai.Float(0.2),
		Text: responses.ResponseTextConfigParam{
			Verbosity: responses.ResponseTextConfigVerbosityLow,
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai title: %w", err)
	}
	title := strings.TrimSpace(resp.OutputText())
	if title == "" {
		return "", fmt.Errorf("openai title: empty response")
	}
	return title, nil
}
