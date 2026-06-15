package brain_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"agent-builder/brain"
	"agent-builder/engine"
	"agent-builder/tools"
)

// TestOpenAILive drives a real OpenAI turn end-to-end through the engine. It is gated
// behind LIVE_OPENAI=1 so the default `go test` stays offline and free.
//
//	LIVE_OPENAI=1 OPENAI_API_KEY=... OPENAI_MODEL=gpt-5.1 go test ./brain/ -run Live -v
func TestOpenAILive(t *testing.T) {
	if os.Getenv("LIVE_OPENAI") == "" {
		t.Skip("set LIVE_OPENAI=1 (and OPENAI_API_KEY) to run the live smoke test")
	}
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Fatal("OPENAI_API_KEY is required for the live test")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5.1"
	}

	reg := tools.NewRegistry()
	reg.Register(tools.Calculator())

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var sawToolUse bool
	out, err := engine.Run(ctx, engine.Input{
		InvocationID: "live-1",
		Brain:        brain.NewOpenAIBrain(key, model),
		Registry:     reg,
		Spec: engine.Spec{
			Instructions: "You are a precise assistant. Use the calculator tool for any arithmetic.",
			Tools:        []string{"calculator"},
		},
		Message: "What is 137 * 19? Use the calculator.",
		Emit: func(e engine.Event) {
			if e.Type == engine.EventToolUse {
				sawToolUse = true
			}
			t.Logf("event: %s %s", e.Type, e.Name)
		},
	})
	if err != nil {
		t.Fatalf("live run failed: %v", err)
	}
	if out.FinalText == "" {
		t.Error("expected a non-empty final answer")
	}
	if !sawToolUse {
		t.Error("expected the model to call the calculator tool")
	}
	if !strings.Contains(out.FinalText, "2603") {
		t.Logf("note: final answer did not contain 2603 (137*19): %q", out.FinalText)
	}
	t.Logf("final: %s", out.FinalText)
}
