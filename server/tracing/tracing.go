// Package tracing wires Braintrust LLM observability via OpenTelemetry. It is entirely
// optional: if BRAINTRUST_API_KEY is unset, Init is a no-op and the app behaves exactly
// as before. When enabled, every OpenAI call is traced (via the contrib middleware) and
// the orchestrator wraps each run in a root span so a turn's LLM calls nest into one
// trace. Tracing is world-touching infra, so it lives on the orchestrator side — the
// engine stays pure.
package tracing

import (
	"context"
	"fmt"
	"os"

	"github.com/braintrustdata/braintrust-sdk-go"
	traceopenai "github.com/braintrustdata/braintrust-sdk-go/trace/contrib/openai"
	"github.com/openai/openai-go/v3/option"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Init configures Braintrust tracing for the given project if BRAINTRUST_API_KEY is set
// (read by the SDK from the environment). It returns a shutdown func that flushes spans,
// and whether tracing was enabled. With the key absent it returns a no-op shutdown and
// enabled=false — the caller then simply skips the OpenAI middleware.
func Init(project string) (shutdown func(context.Context) error, enabled bool, err error) {
	noop := func(context.Context) error { return nil }
	if os.Getenv("BRAINTRUST_API_KEY") == "" {
		return noop, false, nil
	}
	tp := sdktrace.NewTracerProvider()
	if _, err := braintrust.New(tp, braintrust.WithProject(project)); err != nil {
		return noop, false, fmt.Errorf("tracing: braintrust init: %w", err)
	}
	otel.SetTracerProvider(tp)
	return tp.Shutdown, true, nil
}

// OpenAIMiddleware returns a request option that traces every OpenAI call to Braintrust.
// Pass it into brain.NewOpenAIBrain only when Init reported enabled. The contrib
// middleware is built against openai-go v1, but its option.Middleware/MiddlewareNext are
// type aliases identical to v3's, so it plugs into the v3 client directly.
func OpenAIMiddleware() option.RequestOption {
	return option.WithMiddleware(traceopenai.NewMiddleware())
}
