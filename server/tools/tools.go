// Package tools is the registry: tool name -> { definition (told to the LLM),
// executor (run in the harness), consequential flag }. It deliberately does NOT
// import brain — a tool knows nothing about the model or agents. The engine is the
// only place names get resolved into both halves.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool is one capability. Definition fields (Name/Description/Parameters) are the
// "A" side shown to the model; Execute is the "B" side that never reaches the model.
type Tool struct {
	Name        string
	Description string
	// Parameters is the JSON Schema for the arguments (sent to the model).
	Parameters json.RawMessage
	// Consequential tools (create/update/write) pause the loop for confirmation in
	// Phase 3. The engine reads this flag; in Phase 1 it is recorded but not gated.
	Consequential bool
	// UserInput tools pause the loop to ask the human for structured input. They are
	// not approvals and must be handled by a separate runtime broker.
	UserInput bool
	// Execute runs the tool. args is the raw JSON the model produced. Returning an
	// error is normal control flow — the engine surfaces it as a failed tool_result
	// and lets the model recover; it must never crash the loop.
	Execute func(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry holds tools by name. Built once at startup; read-only during a run.
type Registry struct {
	m map[string]Tool
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{m: make(map[string]Tool)} }

// Register adds a tool. It panics on a duplicate or missing name/executor because
// those are wiring bugs that must fail at startup, not at request time.
func (r *Registry) Register(t Tool) {
	if t.Name == "" {
		panic("tools: cannot register a tool with an empty name")
	}
	if t.Execute == nil {
		panic(fmt.Sprintf("tools: tool %q has no Execute func", t.Name))
	}
	if _, dup := r.m[t.Name]; dup {
		panic(fmt.Sprintf("tools: tool %q registered twice", t.Name))
	}
	r.m[t.Name] = t
}

// Get returns the tool and whether it exists.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.m[name]
	return t, ok
}

// Has reports whether a tool name is known. Used to validate an agent spec's tool
// list before it is accepted (a spec may only name tools that exist).
func (r *Registry) Has(name string) bool {
	_, ok := r.m[name]
	return ok
}

// Names returns all registered tool names (unordered). Backs the list_tools tool.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.m))
	for name := range r.m {
		out = append(out, name)
	}
	return out
}
