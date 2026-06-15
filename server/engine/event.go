package engine

import "encoding/json"

// EventType is the normalized event vocabulary the engine emits. It is OURS, not the
// provider's — the brain layer translates OpenAI's deltas/tool_calls into these, so
// nothing downstream (orchestrator, UI, logs) ever sees a provider-specific shape.
type EventType string

const (
	EventLLMText      EventType = "llm_text"
	EventStatus       EventType = "status"
	EventToolUse      EventType = "tool_use"
	EventToolResult   EventType = "tool_result"
	EventConfirm      EventType = "confirm" // emitted for consequential tools (gated in Phase 3)
	EventUserQuestion EventType = "user_question"
	EventDone         EventType = "done"
	EventError        EventType = "error"
	EventAborted      EventType = "aborted"
)

type QuestionOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value"`
}

type QuestionProgress struct {
	Label   string `json:"label,omitempty"`
	Current int    `json:"current,omitempty"`
	Total   int    `json:"total,omitempty"`
}

// Event is one normalized event. Fields are optional by type; see 00-CONTEXT.md §7.
// The same JSON shape is what gets streamed over SSE and appended to logs/*.jsonl.
type Event struct {
	Type              EventType         `json:"type"`
	Seq               int               `json:"seq"`
	Text              string            `json:"text,omitempty"`
	CallID            string            `json:"call_id,omitempty"`
	Name              string            `json:"name,omitempty"`
	Args              json.RawMessage   `json:"args,omitempty"`
	OK                *bool             `json:"ok,omitempty"`
	Data              string            `json:"data,omitempty"`
	Message           string            `json:"message,omitempty"`
	Field             string            `json:"field,omitempty"`
	Question          string            `json:"question,omitempty"`
	Options           []QuestionOption  `json:"options,omitempty"`
	AllowCustom       bool              `json:"allow_custom,omitempty"`
	CustomPlaceholder string            `json:"custom_placeholder,omitempty"`
	Progress          *QuestionProgress `json:"progress,omitempty"`
}

// Emit receives events as the engine produces them. In Phase 1 tests collect them; in
// Phase 3 the orchestrator relays them to SSE and taps them for the audit log.
type Emit func(Event)

// boolPtr is a tiny helper so OK can distinguish true/false/absent.
func boolPtr(b bool) *bool { return &b }
