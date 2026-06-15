// Package platformtools provides the tools that build and manage agents: list_tools,
// list_agents, create_agent, update_agent. Their executors write rows, so they need the
// store — which is why they live here, separate from the dependency-light capability
// tools in package tools. The Agent Builder is just an agent whose row names these.
//
// They are still ordinary tools.Tool values resolved through the same registry, so the
// Builder runs the exact same engine loop as any agent it creates.
package platformtools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"agent-builder/store"
	"agent-builder/tools"
)

// BuilderID is the id of the seeded Agent Builder row.
const BuilderID = "builder"

// platformNames are the tools an agent may NOT be granted via create_agent — they are
// the build/manage tools, reserved for the Builder. Keeping them off created agents is
// least-privilege: a research agent should never be able to mint agents.
var platformNames = map[string]bool{
	"list_tools":        true,
	"list_agents":       true,
	"ask_user_question": true,
	"create_agent":      true,
	"update_agent":      true,
}

// Register adds the four platform tools to reg, wired to st.
func Register(reg *tools.Registry, st *store.Store) {
	reg.Register(listToolsTool(reg))
	reg.Register(listAgentsTool(st))
	reg.Register(askUserQuestionTool())
	reg.Register(createAgentTool(reg, st))
	reg.Register(updateAgentTool(reg, st))
}

// SeedBuilder inserts the Agent Builder row if it does not already exist (idempotent).
func SeedBuilder(ctx context.Context, st *store.Store) error {
	if existing, err := st.GetAgent(ctx, BuilderID); err == nil {
		existing.Name = "Agent Builder"
		existing.Persona = "A friendly assistant that helps design new agents."
		existing.Instructions = builderInstructions()
		existing.Tools = builderTools()
		return st.UpdateAgent(ctx, existing)
	} else if err != store.ErrNotFound {
		return err
	}
	_, err := st.InsertAgent(ctx, store.Agent{
		ID:           BuilderID,
		Name:         "Agent Builder",
		Persona:      "A friendly assistant that helps design new agents.",
		Instructions: builderInstructions(),
		Tools:        builderTools(),
	})
	return err
}

func builderTools() []string {
	return []string{"list_tools", "list_agents", "ask_user_question", "create_agent", "update_agent"}
}

func builderInstructions() string {
	return "You design agents by interviewing the user, then BUILDING the agent. " +
		"Work in three phases:\n\n" +
		"1. ASK. When a required detail is missing, call the ask_user_question tool " +
		"with one clear question, 2-4 options, and a custom option. One question per turn. " +
		"NEVER write a question as plain text — every question MUST go through the " +
		"ask_user_question tool, or the user cannot answer it.\n\n" +
		"2. STOP ASKING. You need only five things: name, purpose, behavior, tools, and " +
		"key constraints. As soon as you can reasonably infer these (defaults are fine for " +
		"the rest), STOP. Ask at most 6 questions total. Do not ask for nice-to-have details.\n\n" +
		"3. BUILD. Call list_tools to pick capabilities, then immediately call create_agent. " +
		"After list_tools your next action is ALWAYS create_agent unless a genuinely blocking " +
		"detail is still unknown. Only write plain text AFTER create_agent succeeds, to confirm " +
		"what you built. Never end your turn with a summary or a question instead of building."
}

// --- list_tools ---

func listToolsTool(reg *tools.Registry) tools.Tool {
	return tools.Tool{
		Name:        "list_tools",
		Description: "List the capability tools available to grant to a new agent.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		Execute: func(ctx context.Context, _ json.RawMessage) (string, error) {
			type entry struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			var out []entry
			for _, name := range reg.Names() {
				if platformNames[name] {
					continue // hide build/manage tools from the grantable catalog
				}
				t, _ := reg.Get(name)
				out = append(out, entry{Name: t.Name, Description: t.Description})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
			return toJSON(out)
		},
	}
}

// --- list_agents ---

func listAgentsTool(st *store.Store) tools.Tool {
	return tools.Tool{
		Name:        "list_agents",
		Description: "List the existing agents (id, name, tools).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		Execute: func(ctx context.Context, _ json.RawMessage) (string, error) {
			agents, err := st.ListAgents(ctx, store.UserFrom(ctx))
			if err != nil {
				return "", err
			}
			type entry struct {
				ID    string   `json:"id"`
				Name  string   `json:"name"`
				Tools []string `json:"tools"`
			}
			out := make([]entry, 0, len(agents))
			for _, a := range agents {
				out = append(out, entry{ID: a.ID, Name: a.Name, Tools: a.Tools})
			}
			return toJSON(out)
		},
	}
}

// --- ask_user_question ---

func askUserQuestionTool() tools.Tool {
	return tools.Tool{
		Name:        "ask_user_question",
		Description: "Ask the user one structured setup question with options and an optional custom answer.",
		UserInput:   true,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"field":              { "type": "string", "description": "stable draft field this answer fills" },
				"question":           { "type": "string", "description": "short user-facing question" },
				"options": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"id":          { "type": "string" },
							"label":       { "type": "string" },
							"description": { "type": ["string", "null"] },
							"value":       { "type": "string" }
						},
						"required": ["id", "label", "description", "value"],
						"additionalProperties": false
					},
					"minItems": 1,
					"maxItems": 4
				},
				"allow_custom":       { "type": "boolean" },
				"custom_placeholder": { "type": ["string", "null"] },
				"progress_label":     { "type": ["string", "null"] },
				"progress_current":   { "type": ["number", "null"] },
				"progress_total":     { "type": ["number", "null"] }
			},
			"required": ["field", "question", "options", "allow_custom", "custom_placeholder", "progress_label", "progress_current", "progress_total"],
			"additionalProperties": false
		}`),
		Execute: func(ctx context.Context, raw json.RawMessage) (string, error) {
			return "", fmt.Errorf("ask_user_question must be handled by the runtime")
		},
	}
}

// --- create_agent ---

type createAgentArgs struct {
	Name         string   `json:"name"`
	Persona      string   `json:"persona"`
	Instructions string   `json:"instructions"`
	Model        string   `json:"model"`
	Tools        []string `json:"tools"`
	Sources      []string `json:"sources"`
}

func createAgentTool(reg *tools.Registry, st *store.Store) tools.Tool {
	return tools.Tool{
		Name:          "create_agent",
		Description:   "Create a new agent. tools must be names returned by list_tools.",
		Consequential: true,
		// OpenAI strict mode requires every property to be in "required"; optional
		// fields are expressed as nullable. A null persona/model decodes to "" in Go.
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name":         { "type": "string", "description": "short display name" },
				"persona":      { "type": ["string", "null"], "description": "who the agent is" },
				"instructions": { "type": "string", "description": "what the agent should do" },
				"model":        { "type": ["string", "null"], "description": "model id; null defaults to gpt-5.1" },
				"tools":        { "type": "array", "items": { "type": "string" }, "description": "capability tool names" },
				"sources":      { "type": ["array", "null"], "items": { "type": "string" }, "description": "URLs or file references the agent should use" }
			},
			"required": ["name", "persona", "instructions", "model", "tools", "sources"],
			"additionalProperties": false
		}`),
		Execute: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var a createAgentArgs
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if strings.TrimSpace(a.Name) == "" {
				return "", fmt.Errorf("name is required")
			}
			if err := validateGrantable(reg, a.Tools); err != nil {
				return "", err
			}
			agent, err := st.InsertAgent(ctx, store.Agent{
				UserID:       store.UserFrom(ctx),
				Name:         a.Name,
				Persona:      a.Persona,
				Instructions: a.Instructions,
				Model:        a.Model,
				Tools:        a.Tools,
				Sources:      a.Sources,
			})
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("created agent %s (%q) with tools %v", agent.ID, agent.Name, agent.Tools), nil
		},
	}
}

// --- update_agent ---

type updateAgentArgs struct {
	ID           string    `json:"id"`
	Name         *string   `json:"name"`
	Persona      *string   `json:"persona"`
	Instructions *string   `json:"instructions"`
	Model        *string   `json:"model"`
	Tools        *[]string `json:"tools"`
	Sources      *[]string `json:"sources"`
}

func updateAgentTool(reg *tools.Registry, st *store.Store) tools.Tool {
	return tools.Tool{
		Name:          "update_agent",
		Description:   "Update an existing agent's fields. Only provided fields change.",
		Consequential: true,
		// Strict mode: all keys required, optionals nullable. A null field means
		// "leave unchanged" (decodes to a nil pointer in Go); a present value updates it.
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id":           { "type": "string" },
				"name":         { "type": ["string", "null"] },
				"persona":      { "type": ["string", "null"] },
				"instructions": { "type": ["string", "null"] },
				"model":        { "type": ["string", "null"] },
				"tools":        { "type": ["array", "null"], "items": { "type": "string" } },
				"sources":      { "type": ["array", "null"], "items": { "type": "string" } }
			},
			"required": ["id", "name", "persona", "instructions", "model", "tools", "sources"],
			"additionalProperties": false
		}`),
		Execute: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var a updateAgentArgs
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if a.ID == "" {
				return "", fmt.Errorf("id is required")
			}
			edit, isEdit := store.EditSessionFrom(ctx)
			if isEdit && a.ID != edit.AgentID {
				return "", fmt.Errorf("agent edit session can only update agent %s", edit.AgentID)
			}
			agent, err := st.GetAgent(ctx, a.ID)
			if err != nil {
				return "", err
			}
			if a.Name != nil {
				agent.Name = *a.Name
			}
			if a.Persona != nil {
				agent.Persona = *a.Persona
			}
			if a.Instructions != nil {
				agent.Instructions = *a.Instructions
			}
			if a.Model != nil {
				agent.Model = *a.Model
			}
			if a.Tools != nil {
				if err := validateGrantable(reg, *a.Tools); err != nil {
					return "", err
				}
				agent.Tools = *a.Tools
			}
			if a.Sources != nil {
				agent.Sources = normalizeSources(*a.Sources)
			}
			var updateErr error
			if isEdit {
				updateErr = st.UpdateAgentWithVersion(ctx, agent, edit.SessionID, updateAgentSummary(a))
			} else {
				updateErr = st.UpdateAgent(ctx, agent)
			}
			if updateErr != nil {
				return "", updateErr
			}
			return fmt.Sprintf("updated agent %s", agent.ID), nil
		},
	}
}

func updateAgentSummary(a updateAgentArgs) string {
	var fields []string
	if a.Name != nil {
		fields = append(fields, "name")
	}
	if a.Persona != nil {
		fields = append(fields, "persona")
	}
	if a.Instructions != nil {
		fields = append(fields, "instructions")
	}
	if a.Model != nil {
		fields = append(fields, "model")
	}
	if a.Tools != nil {
		fields = append(fields, "tools")
	}
	if a.Sources != nil {
		fields = append(fields, "sources")
	}
	if len(fields) == 0 {
		return "No-op agent update"
	}
	return "Updated " + strings.Join(fields, ", ")
}

func normalizeSources(sources []string) []string {
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source != "" {
			out = append(out, source)
		}
	}
	return out
}

// validateGrantable rejects empty, duplicate, unknown, and platform tool names
// (least privilege). It does not mutate names; the caller stores them as given.
func validateGrantable(reg *tools.Registry, names []string) error {
	var unknown, reserved []string
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		if strings.TrimSpace(n) == "" {
			return fmt.Errorf("tool names must not be empty")
		}
		if seen[n] {
			return fmt.Errorf("duplicate tool: %s", n)
		}
		seen[n] = true
		switch {
		case platformNames[n]:
			reserved = append(reserved, n)
		case !reg.Has(n):
			unknown = append(unknown, n)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("unknown tool(s): %s", strings.Join(unknown, ", "))
	}
	if len(reserved) > 0 {
		return fmt.Errorf("cannot grant build/manage tool(s) to an agent: %s", strings.Join(reserved, ", "))
	}
	return nil
}

func toJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
