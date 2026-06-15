package brain

import (
	"context"
	"fmt"
)

// FakeBrain is a scripted Brain for deterministic tests — no network, no API key.
// It returns the next queued Step on each Next call. This is what lets us prove the
// whole engine loop offline (and is itself proof the Brain interface is swappable).
type FakeBrain struct {
	// Steps are returned in order, one per Next call.
	Steps []Step
	// TextDeltas, when present for a step index, are synchronously emitted through
	// the text-delta callback before that step returns.
	TextDeltas [][]string
	// Calls records every Next invocation so tests can assert what the engine sent.
	Calls []FakeCall

	idx int
}

// FakeCall captures the arguments of one Next call for assertions.
type FakeCall struct {
	System  string
	Tools   []ToolDef
	History []Message
}

// Next returns the next scripted Step. If the script runs dry it errors loudly so a
// test failure points at the script, not a nil panic.
func (f *FakeBrain) Next(ctx context.Context, system string, tools []ToolDef, history []Message, onTextDelta func(string)) (Step, error) {
	if err := ctx.Err(); err != nil {
		return Step{}, err
	}
	f.Calls = append(f.Calls, FakeCall{System: system, Tools: tools, History: cloneHistory(history)})
	if f.idx >= len(f.Steps) {
		return Step{}, fmt.Errorf("fakebrain: script exhausted after %d steps", len(f.Steps))
	}
	step := f.Steps[f.idx]
	if f.idx < len(f.TextDeltas) {
		for _, delta := range f.TextDeltas[f.idx] {
			if delta != "" && onTextDelta != nil {
				onTextDelta(delta)
				step.TextStreamed = true
			}
		}
	}
	f.idx++
	return step, nil
}

func cloneHistory(h []Message) []Message {
	out := make([]Message, len(h))
	copy(out, h)
	return out
}

// --- script helpers (keep tests readable) ---

// TextStep scripts a final assistant answer.
func TextStep(text string) Step { return Step{Kind: StepText, Text: text} }

// ToolStep scripts one tool call.
func ToolStep(id, name, argsJSON string) Step {
	return Step{Kind: StepToolCalls, ToolCalls: []ToolCall{{ID: id, Name: name, Args: []byte(argsJSON)}}}
}
