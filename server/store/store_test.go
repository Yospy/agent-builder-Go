package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"agent-builder/brain"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestAgentInsertGet(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()

	a, err := st.InsertAgent(ctx, Agent{Name: "Research", Instructions: "search", Tools: []string{"fetch_url"}, Sources: []string{"https://example.com"}})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected a generated id")
	}
	if a.UserID != DefaultUser || a.Model != "gpt-5.1" {
		t.Errorf("defaults not applied: %+v", a)
	}

	got, err := st.GetAgent(ctx, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Research" || len(got.Tools) != 1 || got.Tools[0] != "fetch_url" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if len(got.Sources) != 1 || got.Sources[0] != "https://example.com" {
		t.Errorf("sources round-trip mismatch: %+v", got.Sources)
	}
}

func TestAgentGetNotFound(t *testing.T) {
	st := openTest(t)
	if _, err := st.GetAgent(context.Background(), "nope"); err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestListAgents(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	st.InsertAgent(ctx, Agent{Name: "A", Tools: []string{}})
	st.InsertAgent(ctx, Agent{Name: "B", Tools: []string{}})
	st.InsertAgent(ctx, Agent{Name: "other", UserID: "someone-else", Tools: []string{}})

	got, err := st.ListAgents(ctx, DefaultUser)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 agents for default user, got %d", len(got))
	}
}

func TestUpdateAgent(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	a, _ := st.InsertAgent(ctx, Agent{Name: "old", Tools: []string{"calculator"}})

	a.Name = "new"
	a.Tools = []string{"fetch_url", "web_search"}
	if err := st.UpdateAgent(ctx, a); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := st.GetAgent(ctx, a.ID)
	if got.Name != "new" || len(got.Tools) != 2 {
		t.Errorf("update not persisted: %+v", got)
	}

	// updating a missing agent is ErrNotFound
	if err := st.UpdateAgent(ctx, Agent{ID: "ghost"}); err != ErrNotFound {
		t.Errorf("want ErrNotFound updating missing agent, got %v", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	agent, _ := st.InsertAgent(ctx, Agent{Name: "A", Tools: []string{}})

	sess, err := st.CreateSession(ctx, DefaultUser, agent.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.AgentID != agent.ID || sess.ID == "" {
		t.Errorf("bad session: %+v", sess)
	}
	if sess.Kind != SessionKindNormal || sess.Title != "New chat" {
		t.Errorf("session defaults wrong: %+v", sess)
	}
	if sess.BuilderStateJSON != "{}" {
		t.Errorf("builder state default = %q, want {}", sess.BuilderStateJSON)
	}

	// binding to a missing agent must fail
	if _, err := st.CreateSession(ctx, DefaultUser, "missing"); err != ErrNotFound {
		t.Errorf("want ErrNotFound for missing agent, got %v", err)
	}

	// append two batches; history accumulates in order
	if err := st.AppendMessages(ctx, sess.ID, []brain.Message{{Role: brain.RoleUser, Content: "hi"}}); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := st.AppendMessages(ctx, sess.ID, []brain.Message{{Role: brain.RoleAssistant, Content: "hello"}}); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	got, err := st.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(got.Messages) != 2 || got.Messages[0].Content != "hi" || got.Messages[1].Role != brain.RoleAssistant {
		t.Errorf("history wrong: %+v", got.Messages)
	}

	if err := st.AppendMessagesAndBuilderState(ctx, sess.ID, nil, `{"version":1,"draft":{"name":"A"}}`); err != nil {
		t.Fatalf("append builder state: %v", err)
	}
	got, err = st.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get session after state: %v", err)
	}
	if got.BuilderStateJSON != `{"version":1,"draft":{"name":"A"}}` {
		t.Errorf("builder state not persisted: %s", got.BuilderStateJSON)
	}

	// appending to a missing session is ErrNotFound
	if err := st.AppendMessages(ctx, "ghost", []brain.Message{{Role: brain.RoleUser, Content: "x"}}); err != ErrNotFound {
		t.Errorf("want ErrNotFound appending to missing session, got %v", err)
	}
}

func TestAgentEditSessionsAndVersions(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	agent, _ := st.InsertAgent(ctx, Agent{Name: "Editable", Tools: []string{"calculator"}})
	other, _ := st.InsertAgent(ctx, Agent{Name: "Other", Tools: []string{}})

	edit, err := st.CreateAgentEditSession(ctx, DefaultUser, agent.ID)
	if err != nil {
		t.Fatalf("create edit session: %v", err)
	}
	if edit.Kind != SessionKindAgentEdit || edit.AgentID != agent.ID {
		t.Fatalf("bad edit session: %+v", edit)
	}
	if _, err := st.CreateAgentEditSession(ctx, DefaultUser, "missing"); err != ErrNotFound {
		t.Fatalf("missing agent edit session = %v, want ErrNotFound", err)
	}

	edits, err := st.ListAgentEditSessions(ctx, DefaultUser)
	if err != nil {
		t.Fatalf("list edit sessions: %v", err)
	}
	if len(edits) != 1 || edits[0].ID != edit.ID {
		t.Fatalf("edit session list = %+v, want only %s", edits, edit.ID)
	}
	if _, err := st.CreateSession(ctx, DefaultUser, other.ID); err != nil {
		t.Fatalf("create normal session: %v", err)
	}
	edits, _ = st.ListAgentEditSessions(ctx, DefaultUser)
	if len(edits) != 1 {
		t.Fatalf("normal sessions should not appear in edit list: %+v", edits)
	}

	agent.Name = "Edited"
	agent.Sources = []string{"https://a16z.com"}
	if err := st.UpdateAgentWithVersion(ctx, agent, edit.ID, "Updated name and sources"); err != nil {
		t.Fatalf("versioned update: %v", err)
	}
	got, _ := st.GetAgent(ctx, agent.ID)
	if got.Name != "Edited" || len(got.Sources) != 1 {
		t.Fatalf("agent not updated: %+v", got)
	}
	versions, err := st.ListAgentVersions(ctx, agent.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 1 || versions[0].VersionNumber != 1 || versions[0].SessionID != edit.ID {
		t.Fatalf("bad versions: %+v", versions)
	}
	if versions[0].ChangeSummary != "Updated name and sources" {
		t.Fatalf("summary = %q", versions[0].ChangeSummary)
	}
}

func TestDeleteSession(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	agent, _ := st.InsertAgent(ctx, Agent{Name: "A", Tools: []string{}})
	sess, _ := st.CreateSession(ctx, DefaultUser, agent.ID)

	if err := st.DeleteSession(ctx, sess.ID); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, err := st.GetSession(ctx, sess.ID); err != ErrNotFound {
		t.Errorf("deleted session should be gone, got %v", err)
	}
	if err := st.DeleteSession(ctx, sess.ID); err != ErrNotFound {
		t.Errorf("deleting missing session = %v, want ErrNotFound", err)
	}

	edit, _ := st.CreateAgentEditSession(ctx, DefaultUser, agent.ID)
	agent.Name = "Edited"
	if err := st.UpdateAgentWithVersion(ctx, agent, edit.ID, "renamed"); err != nil {
		t.Fatalf("versioned update: %v", err)
	}
	if err := st.DeleteSession(ctx, edit.ID); err != nil {
		t.Fatalf("delete edit session: %v", err)
	}
	edits, err := st.ListAgentEditSessions(ctx, DefaultUser)
	if err != nil {
		t.Fatalf("list edit sessions: %v", err)
	}
	if len(edits) != 0 {
		t.Fatalf("deleted edit session still listed: %+v", edits)
	}
	versions, err := st.ListAgentVersions(ctx, agent.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 1 || versions[0].SessionID != edit.ID {
		t.Fatalf("delete edit session should keep applied version audit: %+v", versions)
	}
}

func TestDeleteAgentDeletesSessions(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	agent, _ := st.InsertAgent(ctx, Agent{Name: "A", Tools: []string{}})
	sess1, _ := st.CreateSession(ctx, DefaultUser, agent.ID)
	sess2, _ := st.CreateSession(ctx, DefaultUser, agent.ID)
	edit, _ := st.CreateAgentEditSession(ctx, DefaultUser, agent.ID)
	agent.Name = "changed"
	if err := st.UpdateAgentWithVersion(ctx, agent, edit.ID, "changed"); err != nil {
		t.Fatalf("versioned update: %v", err)
	}

	ids, err := st.ListSessionIDsForAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("list session ids: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("session ids before delete = %v, want 3 ids", ids)
	}

	if err := st.DeleteAgent(ctx, agent.ID); err != nil {
		t.Fatalf("delete agent: %v", err)
	}
	if _, err := st.GetAgent(ctx, agent.ID); err != ErrNotFound {
		t.Errorf("deleted agent should be gone, got %v", err)
	}
	if _, err := st.GetSession(ctx, sess1.ID); err != ErrNotFound {
		t.Errorf("first agent session should be gone, got %v", err)
	}
	if _, err := st.GetSession(ctx, sess2.ID); err != ErrNotFound {
		t.Errorf("second agent session should be gone, got %v", err)
	}
	if _, err := st.GetSession(ctx, edit.ID); err != ErrNotFound {
		t.Errorf("edit session should be gone, got %v", err)
	}
	versions, err := st.ListAgentVersions(ctx, agent.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("versions should be gone after agent delete, got %+v", versions)
	}
	if err := st.DeleteAgent(ctx, agent.ID); err != ErrNotFound {
		t.Errorf("deleting missing agent = %v, want ErrNotFound", err)
	}
}

// Concurrent appends must not lose updates: with MaxOpenConns(1) serializing the
// read-modify-write transaction, N goroutines each appending once must yield N messages.
func TestAppendMessagesConcurrent(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	agent, _ := st.InsertAgent(ctx, Agent{Name: "A", Tools: []string{}})
	sess, _ := st.CreateSession(ctx, DefaultUser, agent.ID)

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- st.AppendMessages(ctx, sess.ID, []brain.Message{{Role: brain.RoleUser, Content: "m"}})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent append: %v", err)
		}
	}
	got, _ := st.GetSession(ctx, sess.ID)
	if len(got.Messages) != n {
		t.Errorf("lost updates: got %d messages, want %d", len(got.Messages), n)
	}
}

func TestReopenAcrossConnections(t *testing.T) {
	// Data must survive closing and reopening the same file (crash-recovery property).
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.db")

	st1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := st1.InsertAgent(context.Background(), Agent{Name: "persist", Tools: []string{}})
	sess, _ := st1.CreateSession(context.Background(), DefaultUser, a.ID)
	if err := st1.AppendMessagesAndBuilderState(context.Background(), sess.ID, nil, `{"version":1,"draft":{"purpose":"research"}}`); err != nil {
		t.Fatal(err)
	}
	st1.Close()

	st2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	if _, err := st2.GetAgent(context.Background(), a.ID); err != nil {
		t.Errorf("agent did not survive reopen: %v", err)
	}
	got, err := st2.GetSession(context.Background(), sess.ID)
	if err != nil {
		t.Errorf("session did not survive reopen: %v", err)
	}
	if got.BuilderStateJSON != `{"version":1,"draft":{"purpose":"research"}}` {
		t.Errorf("builder state did not survive reopen: %s", got.BuilderStateJSON)
	}
}
