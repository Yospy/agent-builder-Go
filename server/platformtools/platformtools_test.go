package platformtools_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-builder/brain"
	"agent-builder/engine"
	"agent-builder/platformtools"
	"agent-builder/store"
	"agent-builder/tools"
)

// fullRegistry wires the capability tools + the platform tools against a store, exactly
// as the server will at boot.
func fullRegistry(t *testing.T) (*tools.Registry, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	reg := tools.NewRegistry()
	reg.Register(tools.Calculator())
	reg.Register(tools.FetchURL())
	platformtools.Register(reg, st)
	return reg, st
}

func call(t *testing.T, reg *tools.Registry, name, args string) (string, error) {
	t.Helper()
	return callWithContext(t, reg, context.Background(), name, args)
}

func callWithContext(t *testing.T, reg *tools.Registry, ctx context.Context, name, args string) (string, error) {
	t.Helper()
	tool, ok := reg.Get(name)
	if !ok {
		t.Fatalf("tool %q not registered", name)
	}
	return tool.Execute(ctx, json.RawMessage(args))
}

func TestSeedBuilderIdempotent(t *testing.T) {
	_, st := fullRegistry(t)
	ctx := context.Background()
	if err := platformtools.SeedBuilder(ctx, st); err != nil {
		t.Fatalf("seed 1: %v", err)
	}
	if err := platformtools.SeedBuilder(ctx, st); err != nil {
		t.Fatalf("seed 2 (should be idempotent): %v", err)
	}
	b, err := st.GetAgent(ctx, platformtools.BuilderID)
	if err != nil {
		t.Fatalf("builder not seeded: %v", err)
	}
	if len(b.Tools) != 5 {
		t.Errorf("builder should have 5 platform tools, got %v", b.Tools)
	}
	if !contains(b.Tools, "ask_user_question") {
		t.Errorf("builder tools should include ask_user_question, got %v", b.Tools)
	}
	agents, _ := st.ListAgents(ctx, store.DefaultUser)
	if len(agents) != 1 {
		t.Errorf("expected exactly one builder row after double seed, got %d", len(agents))
	}
}

func TestListToolsHidesPlatformTools(t *testing.T) {
	reg, _ := fullRegistry(t)
	out, err := call(t, reg, "list_tools", `{}`)
	if err != nil {
		t.Fatalf("list_tools: %v", err)
	}
	for _, hidden := range []string{"create_agent", "update_agent", "list_tools", "list_agents", "ask_user_question"} {
		if strings.Contains(out, `"`+hidden+`"`) {
			t.Errorf("list_tools must hide platform tool %q; got %s", hidden, out)
		}
	}
	for _, shown := range []string{"calculator", "fetch_url"} {
		if !strings.Contains(out, shown) {
			t.Errorf("list_tools should show capability tool %q; got %s", shown, out)
		}
	}
}

func TestCreateAgentValidatesTools(t *testing.T) {
	reg, st := fullRegistry(t)

	// unknown tool -> rejected, no row written
	if _, err := call(t, reg, "create_agent",
		`{"name":"X","instructions":"do","tools":["nope"]}`); err == nil {
		t.Error("create_agent should reject an unknown tool")
	}
	// granting a platform tool -> rejected (least privilege)
	if _, err := call(t, reg, "create_agent",
		`{"name":"X","instructions":"do","tools":["create_agent"]}`); err == nil {
		t.Error("create_agent should refuse to grant a platform tool")
	}
	if _, err := call(t, reg, "create_agent",
		`{"name":"X","instructions":"do","tools":["ask_user_question"]}`); err == nil {
		t.Error("create_agent should refuse to grant ask_user_question")
	}
	agents, _ := st.ListAgents(context.Background(), store.DefaultUser)
	if len(agents) != 0 {
		t.Errorf("no agent should have been created on validation failure, got %d", len(agents))
	}

	// valid -> row written
	out, err := call(t, reg, "create_agent",
		`{"name":"Research","instructions":"search the web","tools":["fetch_url"]}`)
	if err != nil {
		t.Fatalf("valid create_agent failed: %v", err)
	}
	if !strings.Contains(out, "created agent") {
		t.Errorf("unexpected output: %s", out)
	}
	agents, _ = st.ListAgents(context.Background(), store.DefaultUser)
	if len(agents) != 1 || agents[0].Name != "Research" {
		t.Errorf("expected the Research agent to be persisted, got %+v", agents)
	}
}

func TestUpdateAgentTool(t *testing.T) {
	reg, st := fullRegistry(t)
	ctx := context.Background()

	out, _ := call(t, reg, "create_agent", `{"name":"X","instructions":"do","tools":["calculator"]}`)
	// pull the id back out of the store (only one agent)
	agents, _ := st.ListAgents(ctx, store.DefaultUser)
	id := agents[0].ID
	_ = out

	// update name + tools via pointer fields; persona/instructions left absent (unchanged)
	if _, err := call(t, reg, "update_agent",
		`{"id":"`+id+`","name":"Renamed","tools":["fetch_url"]}`); err != nil {
		t.Fatalf("update_agent: %v", err)
	}
	got, _ := st.GetAgent(ctx, id)
	if got.Name != "Renamed" {
		t.Errorf("name not updated: %q", got.Name)
	}
	if len(got.Tools) != 1 || got.Tools[0] != "fetch_url" {
		t.Errorf("tools not updated: %v", got.Tools)
	}
	if _, err := call(t, reg, "update_agent",
		`{"id":"`+id+`","sources":[" https://a16z.com ",""]}`); err != nil {
		t.Fatalf("update sources: %v", err)
	}
	got, _ = st.GetAgent(ctx, id)
	if len(got.Sources) != 1 || got.Sources[0] != "https://a16z.com" {
		t.Errorf("sources not normalized/updated: %v", got.Sources)
	}
	if got.Instructions != "do" {
		t.Errorf("absent field should be unchanged, got instructions=%q", got.Instructions)
	}

	// update_agent on a missing id -> error
	if _, err := call(t, reg, "update_agent", `{"id":"ghost","name":"x"}`); err == nil {
		t.Error("update_agent should error on a missing id")
	}
	// update_agent granting a platform tool -> rejected
	if _, err := call(t, reg, "update_agent", `{"id":"`+id+`","tools":["create_agent"]}`); err == nil {
		t.Error("update_agent should refuse to grant a platform tool")
	}
}

func TestUpdateAgentToolInEditSessionIsScopedAndVersioned(t *testing.T) {
	reg, st := fullRegistry(t)
	ctx := context.Background()
	agent, _ := st.InsertAgent(ctx, store.Agent{Name: "Target", Tools: []string{"calculator"}})
	other, _ := st.InsertAgent(ctx, store.Agent{Name: "Other", Tools: []string{}})
	edit, _ := st.CreateAgentEditSession(ctx, store.DefaultUser, agent.ID)
	editCtx := store.WithEditSession(ctx, edit.ID, agent.ID)

	if _, err := callWithContext(t, reg, editCtx, "update_agent",
		`{"id":"`+other.ID+`","name":"bad"}`); err == nil {
		t.Fatal("edit session should not update a different agent")
	}
	if _, err := callWithContext(t, reg, editCtx, "update_agent",
		`{"id":"`+agent.ID+`","name":"Updated","sources":["https://example.com"]}`); err != nil {
		t.Fatalf("scoped update_agent: %v", err)
	}
	got, _ := st.GetAgent(ctx, agent.ID)
	if got.Name != "Updated" || len(got.Sources) != 1 {
		t.Fatalf("agent not updated: %+v", got)
	}
	versions, _ := st.ListAgentVersions(ctx, agent.ID)
	if len(versions) != 1 || versions[0].SessionID != edit.ID {
		t.Fatalf("expected one version linked to edit session, got %+v", versions)
	}
}

func TestToolsRespectContextUser(t *testing.T) {
	reg, st := fullRegistry(t)
	// create_agent under user "alice"
	alice := store.WithUser(context.Background(), "alice")
	ct, _ := reg.Get("create_agent")
	if _, err := ct.Execute(alice, json.RawMessage(`{"name":"AlicesAgent","instructions":"do","tools":["calculator"]}`)); err != nil {
		t.Fatalf("create under alice: %v", err)
	}
	// the row is owned by alice, not the default user
	if got, _ := st.ListAgents(context.Background(), store.DefaultUser); len(got) != 0 {
		t.Errorf("default user should see no agents, got %d", len(got))
	}
	if got, _ := st.ListAgents(context.Background(), "alice"); len(got) != 1 {
		t.Errorf("alice should own 1 agent, got %d", len(got))
	}
	// list_agents under alice's context sees her agent
	lt, _ := reg.Get("list_agents")
	out, _ := lt.Execute(alice, json.RawMessage(`{}`))
	if !strings.Contains(out, "AlicesAgent") {
		t.Errorf("list_agents under alice should show her agent: %s", out)
	}
}

// The Phase-2 headline gate: ONE engine, N rows — behavior differs by row only. We
// create two agents (via create_agent), then run each through the SAME engine with the
// SAME FakeBrain, and assert the tools exposed to the brain come straight from the row.
func TestOneEngineManyRows(t *testing.T) {
	reg, st := fullRegistry(t)
	ctx := context.Background()

	if _, err := call(t, reg, "create_agent",
		`{"name":"Calc","instructions":"do math","tools":["calculator"]}`); err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, reg, "create_agent",
		`{"name":"Web","instructions":"browse","tools":["fetch_url"]}`); err != nil {
		t.Fatal(err)
	}
	agents, _ := st.ListAgents(ctx, store.DefaultUser)
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	for _, a := range agents {
		fb := &brain.FakeBrain{Steps: []brain.Step{brain.TextStep("ok")}}
		_, err := engine.Run(ctx, engine.Input{
			InvocationID: "run-" + a.ID,
			Brain:        fb,
			Registry:     reg,
			Spec:         engine.Spec{Persona: a.Persona, Instructions: a.Instructions, Tools: a.Tools},
			Message:      "go",
			Emit:         func(engine.Event) {},
		})
		if err != nil {
			t.Fatalf("run %s: %v", a.Name, err)
		}
		// Same engine, same brain — only the row differs. The tool the brain was shown
		// must be exactly the one named in that agent's row.
		seen := fb.Calls[0].Tools
		if len(seen) != 1 || seen[0].Name != a.Tools[0] {
			t.Errorf("agent %q exposed tools %v, want row tools %v", a.Name, toolNames(seen), a.Tools)
		}
	}
}

func toolNames(defs []brain.ToolDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
