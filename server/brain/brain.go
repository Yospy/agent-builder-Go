// Package brain is the LLM boundary. The engine talks to a Brain; the brain decides
// (text or tool calls). Concrete brains (OpenAI, a fake for tests) live behind the
// interface so the engine never depends on a provider — proving "the brain is a
// swappable dialect."
package brain

import (
	"context"
	"encoding/json"
)

// Roles for a Message. These mirror the chat-completions roles.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message is one entry in the conversation history (the resume state).
type Message struct {
	Role string `json:"role"`
	// Content is the text body. For an assistant message that only requested tools
	// it may be empty; for a tool message it is the tool's result.
	Content string `json:"content,omitempty"`
	// ToolCalls is set on an assistant message that requested tool execution.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID + Name are set on a tool-result message (Role == RoleTool).
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

// ToolCall is the brain's request to run one tool. Args is raw JSON so the engine
// can hand it to the executor without the brain package knowing any tool's shape.
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// ToolDef is the brain-facing description of a tool (the "A" side of resolution):
// what the model is told it can call. The executor (the "B" side) never appears here.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema for the arguments
}

// StepKind says what the brain produced this turn.
type StepKind int

const (
	// StepText: the brain returned a final assistant answer (no tool calls).
	StepText StepKind = iota
	// StepToolCalls: the brain wants one or more tools run before it can continue.
	StepToolCalls
)

// Step is the result of a single brain turn. When Kind == StepToolCalls, ToolCalls
// is non-empty; Text may still carry any assistant preamble the model emitted.
type Step struct {
	Kind         StepKind
	Text         string
	TextStreamed bool
	ToolCalls    []ToolCall
}

// Brain is the LLM. One Next call == one assistant turn. It is pure with respect to
// our system: it holds no conversation state — the engine passes the full history in.
type Brain interface {
	Next(ctx context.Context, system string, tools []ToolDef, history []Message, onTextDelta func(string)) (Step, error)
}
