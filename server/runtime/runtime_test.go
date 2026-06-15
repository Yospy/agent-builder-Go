package runtime_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-builder/brain"
	"agent-builder/logx"
	"agent-builder/platformtools"
	"agent-builder/runtime"
	"agent-builder/store"
	"agent-builder/tools"
)

// --- test harness ---

type harness struct {
	server  *runtime.Server
	srv     *httptest.Server
	store   *store.Store
	reg     *tools.Registry
	saved   *bool // set when the consequential "save" tool runs
	logsDir string
}

type fakeTitleSummarizer struct {
	title string
	err   error
}

func (f fakeTitleSummarizer) SummarizeTitle(ctx context.Context, prompt string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.title, nil
}

func newHarness(t *testing.T, b brain.Brain) *harness {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	saved := new(bool)
	reg := tools.NewRegistry()
	reg.Register(tools.Calculator())
	reg.Register(tools.Tool{
		Name: "save", Description: "consequential save", Consequential: true,
		Parameters: json.RawMessage(`{"type":"object"}`),
		Execute:    func(ctx context.Context, a json.RawMessage) (string, error) { *saved = true; return "saved", nil },
	})
	platformtools.Register(reg, st)

	logsDir := t.TempDir()
	s := runtime.NewServer(st, reg, b, logsDir, "", logx.Discard())
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return &harness{server: s, srv: ts, store: st, reg: reg, saved: saved, logsDir: logsDir}
}

func (h *harness) seedAgent(t *testing.T, tools []string) store.Agent {
	t.Helper()
	a, err := h.store.InsertAgent(context.Background(), store.Agent{Name: "T", Instructions: "do", Tools: tools})
	if err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	return a
}

func (h *harness) post(t *testing.T, path, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(h.srv.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func (h *harness) delete(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, h.srv.URL+path, nil)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	return resp
}

func (h *harness) patch(t *testing.T, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, h.srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("PATCH %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", path, err)
	}
	return resp
}

type sseEvent map[string]any

// readSSE consumes the stream, invoking onEvent for each event, and returns all events.
// It stops at a terminal event (done/error/aborted) or when the stream closes.
func readSSE(t *testing.T, body io.Reader, onEvent func(sseEvent)) []sseEvent {
	t.Helper()
	var events []sseEvent
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev sseEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			t.Fatalf("bad SSE json: %v (%q)", err, line)
		}
		events = append(events, ev)
		if onEvent != nil {
			onEvent(ev)
		}
		switch ev["type"] {
		case "done", "error", "aborted":
			return events
		}
	}
	return events
}

func types(events []sseEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i], _ = e["type"].(string)
	}
	return out
}

func nonStatusTypes(events []sseEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		typ, _ := e["type"].(string)
		if typ != "status" {
			out = append(out, typ)
		}
	}
	return out
}

func waitSessionMessages(t *testing.T, st *store.Store, sessionID string, want int) store.Session {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		sess, err := st.GetSession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("load session: %v", err)
		}
		if len(sess.Messages) == want {
			return sess
		}
		if time.Now().After(deadline) {
			t.Fatalf("persisted history len = %d, want %d", len(sess.Messages), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// --- tests ---

func TestFullTurn(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("c1", "calculator", `{"expression":"2+2"}`),
		brain.TextStep("It's 4."),
	}}
	h := newHarness(t, fb)
	agent := h.seedAgent(t, []string{"calculator"})

	// mint a session
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create session: %d", resp.StatusCode)
	}
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	// run a turn
	runResp := h.post(t, "/api/sessions/"+cs.SessionID+"/run", `{"message":"what is 2+2?"}`)
	defer runResp.Body.Close()
	if ct := runResp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected SSE, got %q", ct)
	}
	events := readSSE(t, runResp.Body, nil)

	got := nonStatusTypes(events)
	want := []string{"tool_use", "tool_result", "done"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("events = %v, want %v", got, want)
	}
	if last := events[len(events)-1]; last["text"] != "It's 4." {
		t.Errorf("done.text = %v, want 'It's 4.'", last["text"])
	}

	// history persisted: user + assistant(toolreq) + tool + assistant(final) = 4
	waitSessionMessages(t, h.store, cs.SessionID, 4)
}

func TestChatTitleUsesSummarizerAndCapsWords(t *testing.T) {
	h := newHarness(t, &brain.FakeBrain{})
	h.server.SetTitleSummarizer(fakeTitleSummarizer{title: `"Build Article Fetcher Agent Now"`})

	resp, err := http.Post(h.srv.URL+"/api/chat-title", "application/json", strings.NewReader(
		`{"prompt":"Build an article fetcher agent that helps me"}`,
	))
	if err != nil {
		t.Fatalf("POST chat-title: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Title != "Build Article Fetcher" {
		t.Fatalf("title = %q, want compact model title", body.Title)
	}
}

func TestChatTitleFallsBackWhenSummarizerFails(t *testing.T) {
	h := newHarness(t, &brain.FakeBrain{})
	h.server.SetTitleSummarizer(fakeTitleSummarizer{err: context.Canceled})

	resp, err := http.Post(h.srv.URL+"/api/chat-title", "application/json", strings.NewReader(
		`{"prompt":"Build an article fetcher agent that helps me"}`,
	))
	if err != nil {
		t.Fatalf("POST chat-title: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Title != "Build an article" {
		t.Fatalf("title = %q, want fallback first three words", body.Title)
	}
}

func TestConfirmApproveFlow(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("c1", "save", `{}`),
		brain.TextStep("done saving."),
	}}
	h := newHarness(t, fb)
	agent := h.seedAgent(t, []string{"save"})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	runResp := h.post(t, "/api/sessions/"+cs.SessionID+"/run", `{"message":"save it"}`)
	defer runResp.Body.Close()

	// When the confirm event arrives, approve on a separate request, then keep reading.
	events := readSSE(t, runResp.Body, func(ev sseEvent) {
		if ev["type"] == "confirm" {
			callID, _ := ev["call_id"].(string)
			ar := h.post(t, "/api/sessions/"+cs.SessionID+"/approve",
				`{"call_id":"`+callID+`","decision":"approve"}`)
			if ar.StatusCode != http.StatusAccepted {
				t.Errorf("approve status = %d, want 202", ar.StatusCode)
			}
			ar.Body.Close()
		}
	})

	if !*h.saved {
		t.Error("approved consequential tool should have run")
	}
	got := nonStatusTypes(events)
	want := []string{"tool_use", "confirm", "tool_result", "done"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("events = %v, want %v", got, want)
	}
}

func TestConfirmDenyFlow(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("c1", "save", `{}`),
		brain.TextStep("ok, didn't save."),
	}}
	h := newHarness(t, fb)
	agent := h.seedAgent(t, []string{"save"})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	runResp := h.post(t, "/api/sessions/"+cs.SessionID+"/run", `{"message":"save it"}`)
	defer runResp.Body.Close()

	var deniedResult bool
	events := readSSE(t, runResp.Body, func(ev sseEvent) {
		if ev["type"] == "confirm" {
			callID, _ := ev["call_id"].(string)
			ar := h.post(t, "/api/sessions/"+cs.SessionID+"/approve",
				`{"call_id":"`+callID+`","decision":"deny"}`)
			ar.Body.Close()
		}
		if ev["type"] == "tool_result" && ev["data"] == "denied by user" {
			deniedResult = true
		}
	})

	if *h.saved {
		t.Error("denied consequential tool must NOT run")
	}
	if !deniedResult {
		t.Errorf("expected a 'denied by user' tool_result; events=%v", types(events))
	}
	// the model still produces a final answer after denial
	if last := events[len(events)-1]; last["type"] != "done" {
		t.Errorf("run should still complete with done; got %v", last["type"])
	}
}

func TestAgentEditSessionApproveUpdatesAgentAndVersion(t *testing.T) {
	agent := store.Agent{}
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("u1", "update_agent", `{"id":"placeholder","name":"Edited","sources":["https://a16z.com"],"tools":["calculator"]}`),
		brain.TextStep("Applied."),
	}}
	h := newHarness(t, fb)
	agent = h.seedAgent(t, []string{"calculator"})
	fb.Steps[0] = brain.ToolStep("u1", "update_agent", `{"id":"`+agent.ID+`","name":"Edited","sources":["https://a16z.com"],"tools":["calculator"]}`)

	resp := h.post(t, "/api/agents/"+agent.ID+"/edit-sessions", `{}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create edit session = %d", resp.StatusCode)
	}
	var edit struct {
		ID      string `json:"id"`
		AgentID string `json:"agent_id"`
		Kind    string `json:"kind"`
	}
	json.NewDecoder(resp.Body).Decode(&edit)
	resp.Body.Close()
	if edit.AgentID != agent.ID || edit.Kind != store.SessionKindAgentEdit {
		t.Fatalf("bad edit session response: %+v", edit)
	}

	runResp := h.post(t, "/api/sessions/"+edit.ID+"/run", `{"message":"add the source"}`)
	defer runResp.Body.Close()
	readSSE(t, runResp.Body, func(ev sseEvent) {
		if ev["type"] == "confirm" {
			callID, _ := ev["call_id"].(string)
			ar := h.post(t, "/api/sessions/"+edit.ID+"/approve",
				`{"call_id":"`+callID+`","decision":"approve"}`)
			if ar.StatusCode != http.StatusAccepted {
				t.Errorf("approve status = %d, want 202", ar.StatusCode)
			}
			ar.Body.Close()
		}
	})

	got, err := h.store.GetAgent(context.Background(), agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Edited" || len(got.Sources) != 1 || got.Sources[0] != "https://a16z.com" {
		t.Fatalf("agent not updated from edit chat: %+v", got)
	}
	versions, err := h.store.ListAgentVersions(context.Background(), agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0].SessionID != edit.ID || versions[0].VersionNumber != 1 {
		t.Fatalf("bad versions: %+v", versions)
	}
}

func TestAgentEditSessionDenyDoesNotUpdateAgentOrVersion(t *testing.T) {
	fb := &brain.FakeBrain{}
	h := newHarness(t, fb)
	agent := h.seedAgent(t, []string{"calculator"})
	fb.Steps = []brain.Step{
		brain.ToolStep("u1", "update_agent", `{"id":"`+agent.ID+`","name":"Denied"}`),
		brain.TextStep("Not applied."),
	}

	resp := h.post(t, "/api/agents/"+agent.ID+"/edit-sessions", `{}`)
	var edit struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&edit)
	resp.Body.Close()

	runResp := h.post(t, "/api/sessions/"+edit.ID+"/run", `{"message":"rename it"}`)
	defer runResp.Body.Close()
	readSSE(t, runResp.Body, func(ev sseEvent) {
		if ev["type"] == "confirm" {
			callID, _ := ev["call_id"].(string)
			ar := h.post(t, "/api/sessions/"+edit.ID+"/approve",
				`{"call_id":"`+callID+`","decision":"deny"}`)
			ar.Body.Close()
		}
	})

	got, _ := h.store.GetAgent(context.Background(), agent.ID)
	if got.Name == "Denied" {
		t.Fatalf("denied edit should not update agent: %+v", got)
	}
	versions, _ := h.store.ListAgentVersions(context.Background(), agent.ID)
	if len(versions) != 0 {
		t.Fatalf("denied edit should not write versions: %+v", versions)
	}
}

func TestUserQuestionAnswerFlow(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("q1", "ask_user_question", `{
			"field":"fetch_mode",
			"question":"How should this agent get articles?",
			"options":[
				{"id":"paste","label":"Paste URLs","description":null,"value":"user_provided"},
				{"id":"both","label":"Both","description":"Most flexible","value":"both"}
			],
			"allow_custom":true,
			"custom_placeholder":"Describe the source workflow",
			"progress_label":"Agent setup",
			"progress_current":2,
			"progress_total":5
		}`),
		brain.TextStep("got it."),
	}}
	h := newHarness(t, fb)
	agent, err := h.store.InsertAgent(context.Background(), store.Agent{
		ID: "builder", Name: "Agent Builder", Instructions: "build", Tools: []string{"ask_user_question"},
	})
	if err != nil {
		t.Fatalf("seed builder: %v", err)
	}

	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	runResp := h.post(t, "/api/sessions/"+cs.SessionID+"/run", `{"message":"build"}`)
	defer runResp.Body.Close()

	events := readSSE(t, runResp.Body, func(ev sseEvent) {
		if ev["type"] == "user_question" {
			if ev["question"] != "How should this agent get articles?" {
				t.Errorf("question = %v", ev["question"])
			}
			callID, _ := ev["call_id"].(string)
			ar := h.post(t, "/api/sessions/"+cs.SessionID+"/answer",
				`{"call_id":"`+callID+`","option_id":"both"}`)
			if ar.StatusCode != http.StatusAccepted {
				t.Errorf("answer status = %d, want 202", ar.StatusCode)
			}
			ar.Body.Close()
		}
	})

	got := nonStatusTypes(events)
	want := []string{"user_question", "tool_result", "done"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v, want %v", got, want)
	}
	sess := waitSessionMessages(t, h.store, cs.SessionID, 4)
	if !strings.Contains(sess.BuilderStateJSON, `"fetch_mode":"both"`) {
		t.Fatalf("builder state not updated: %s", sess.BuilderStateJSON)
	}
	if !strings.Contains(sess.Messages[2].Content, `"option_id":"both"`) {
		t.Fatalf("answer tool result not persisted: %+v", sess.Messages)
	}
}

func TestAnswerValidationAndUnknownCall(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("q1", "ask_user_question", `{
			"field":"tone",
			"question":"Choose tone",
			"options":[{"id":"sharp","label":"Sharp","description":null,"value":"sharp"}],
			"allow_custom":false,
			"custom_placeholder":null,
			"progress_label":null,
			"progress_current":null,
			"progress_total":null
		}`),
		brain.TextStep("done."),
	}}
	h := newHarness(t, fb)
	agent, _ := h.store.InsertAgent(context.Background(), store.Agent{
		ID: "builder", Name: "Agent Builder", Instructions: "build", Tools: []string{"ask_user_question"},
	})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	unknown := h.post(t, "/api/sessions/"+cs.SessionID+"/answer", `{"call_id":"ghost","option_id":"sharp"}`)
	if unknown.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown answer = %d, want 404", unknown.StatusCode)
	}
	unknown.Body.Close()

	runResp := h.post(t, "/api/sessions/"+cs.SessionID+"/run", `{"message":"build"}`)
	defer runResp.Body.Close()
	events := readSSE(t, runResp.Body, func(ev sseEvent) {
		if ev["type"] == "user_question" {
			callID, _ := ev["call_id"].(string)
			bad := h.post(t, "/api/sessions/"+cs.SessionID+"/answer",
				`{"call_id":"`+callID+`","custom_text":"friendly"}`)
			if bad.StatusCode != http.StatusBadRequest {
				t.Errorf("invalid custom answer = %d, want 400", bad.StatusCode)
			}
			bad.Body.Close()
			ok := h.post(t, "/api/sessions/"+cs.SessionID+"/answer",
				`{"call_id":"`+callID+`","option_id":"sharp"}`)
			if ok.StatusCode != http.StatusAccepted {
				t.Errorf("valid answer = %d, want 202", ok.StatusCode)
			}
			ok.Body.Close()
		}
	})
	if last := events[len(events)-1]; last["type"] != "done" {
		t.Fatalf("run should complete after valid answer, got %v", last["type"])
	}
}

func TestCancelWhileWaitingForAnswerDoesNotPersist(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("q1", "ask_user_question", `{
			"field":"scope",
			"question":"Choose scope",
			"options":[{"id":"official","label":"Official only","description":null,"value":"official"}],
			"allow_custom":true,
			"custom_placeholder":null,
			"progress_label":null,
			"progress_current":null,
			"progress_total":null
		}`),
		brain.TextStep("unreachable."),
	}}
	h := newHarness(t, fb)
	agent, _ := h.store.InsertAgent(context.Background(), store.Agent{
		ID: "builder", Name: "Agent Builder", Instructions: "build", Tools: []string{"ask_user_question"},
	})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		h.srv.URL+"/api/sessions/"+cs.SessionID+"/run", strings.NewReader(`{"message":"build"}`))
	req.Header.Set("Content-Type", "application/json")

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("run request: %v", err)
	}
	sc := bufio.NewScanner(r.Body)
	for sc.Scan() {
		if strings.Contains(sc.Text(), `"type":"user_question"`) {
			cancel()
			break
		}
	}
	r.Body.Close()
	time.Sleep(50 * time.Millisecond)

	sess, _ := h.store.GetSession(context.Background(), cs.SessionID)
	if len(sess.Messages) != 0 {
		t.Fatalf("cancelled question turn should not persist messages: %+v", sess.Messages)
	}
	if sess.BuilderStateJSON != "{}" {
		t.Fatalf("cancelled question turn should not persist state: %s", sess.BuilderStateJSON)
	}
}

func TestAuditLogWritten(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("c1", "calculator", `{"expression":"6*7"}`),
		brain.TextStep("42."),
	}}
	h := newHarness(t, fb)
	agent := h.seedAgent(t, []string{"calculator"})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	runResp := h.post(t, "/api/sessions/"+cs.SessionID+"/run", `{"message":"6*7?"}`)
	readSSE(t, runResp.Body, nil)
	runResp.Body.Close()

	// find the single jsonl audit file under logsDir/agents/<agentID>/<session>/
	var file string
	filepath.Walk(h.logsDir, func(p string, info os.FileInfo, err error) error {
		if err == nil && strings.HasSuffix(p, ".jsonl") {
			file = p
		}
		return nil
	})
	if file == "" {
		t.Fatal("no audit jsonl file written")
	}
	if !strings.Contains(file, filepath.Join("agents", agent.ID, cs.SessionID)) {
		t.Errorf("audit path not isolated by agent/session: %s", file)
	}
	data, _ := os.ReadFile(file)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var sawTS, sawToolUse, sawDone bool
	for _, ln := range lines {
		var e map[string]any
		if err := json.Unmarshal([]byte(ln), &e); err != nil {
			t.Fatalf("audit line not valid JSON: %v (%q)", err, ln)
		}
		if _, ok := e["ts"]; ok {
			sawTS = true
		}
		switch e["type"] {
		case "tool_use":
			sawToolUse = true
		case "done":
			sawDone = true
		}
	}
	if !sawTS || !sawToolUse || !sawDone {
		t.Errorf("audit log incomplete: ts=%v tool_use=%v done=%v", sawTS, sawToolUse, sawDone)
	}
}

func TestApproveUnknownCallID(t *testing.T) {
	h := newHarness(t, &brain.FakeBrain{})
	agent := h.seedAgent(t, []string{})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	ar := h.post(t, "/api/sessions/"+cs.SessionID+"/approve", `{"call_id":"ghost","decision":"approve"}`)
	defer ar.Body.Close()
	if ar.StatusCode != http.StatusNotFound {
		t.Errorf("approve unknown call = %d, want 404", ar.StatusCode)
	}
}

func TestDeleteSession(t *testing.T) {
	h := newHarness(t, &brain.FakeBrain{})
	agent := h.seedAgent(t, []string{})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	del := h.delete(t, "/api/sessions/"+cs.SessionID)
	if del.StatusCode != http.StatusNoContent {
		t.Fatalf("delete session = %d, want 204", del.StatusCode)
	}
	del.Body.Close()
	if _, err := h.store.GetSession(context.Background(), cs.SessionID); err != store.ErrNotFound {
		t.Errorf("deleted session should be gone, got %v", err)
	}

	missing := h.delete(t, "/api/sessions/"+cs.SessionID)
	if missing.StatusCode != http.StatusNotFound {
		t.Errorf("delete missing session = %d, want 404", missing.StatusCode)
	}
	missing.Body.Close()
}

func TestDeleteAgentEditSession(t *testing.T) {
	h := newHarness(t, &brain.FakeBrain{})
	agent := h.seedAgent(t, []string{})
	resp := h.post(t, "/api/agents/"+agent.ID+"/edit-sessions", `{}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create edit session = %d, want 201", resp.StatusCode)
	}
	var edit struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&edit)
	resp.Body.Close()

	del := h.delete(t, "/api/sessions/"+edit.ID)
	if del.StatusCode != http.StatusNoContent {
		t.Fatalf("delete edit session = %d, want 204", del.StatusCode)
	}
	del.Body.Close()

	list, err := http.Get(h.srv.URL + "/api/agent-edit-sessions")
	if err != nil {
		t.Fatalf("GET edit sessions: %v", err)
	}
	defer list.Body.Close()
	var edits []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(list.Body).Decode(&edits); err != nil {
		t.Fatalf("decode edit sessions: %v", err)
	}
	for _, got := range edits {
		if got.ID == edit.ID {
			t.Fatalf("deleted edit session still listed: %+v", edits)
		}
	}
}

func TestDeleteAgentDeletesSessions(t *testing.T) {
	h := newHarness(t, &brain.FakeBrain{})
	agent := h.seedAgent(t, []string{})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	del := h.delete(t, "/api/agents/"+agent.ID)
	if del.StatusCode != http.StatusNoContent {
		t.Fatalf("delete agent = %d, want 204", del.StatusCode)
	}
	del.Body.Close()
	if _, err := h.store.GetAgent(context.Background(), agent.ID); err != store.ErrNotFound {
		t.Errorf("deleted agent should be gone, got %v", err)
	}
	if _, err := h.store.GetSession(context.Background(), cs.SessionID); err != store.ErrNotFound {
		t.Errorf("agent session should be deleted, got %v", err)
	}
}

func TestDeleteBuilderRejected(t *testing.T) {
	h := newHarness(t, &brain.FakeBrain{})
	if _, err := h.store.InsertAgent(context.Background(), store.Agent{
		ID:           "builder",
		Name:         "Agent Builder",
		Instructions: "build agents",
		Tools:        []string{},
	}); err != nil {
		t.Fatalf("seed builder: %v", err)
	}

	del := h.delete(t, "/api/agents/builder")
	if del.StatusCode != http.StatusBadRequest {
		t.Fatalf("delete builder = %d, want 400", del.StatusCode)
	}
	del.Body.Close()
	if _, err := h.store.GetAgent(context.Background(), "builder"); err != nil {
		t.Errorf("builder should remain after rejected delete: %v", err)
	}
}

// A blocking brain holds a run in flight so we can test 409 and cancellation.
type blockingBrain struct {
	entered chan struct{}
	release chan struct{}
}

func (b *blockingBrain) Next(ctx context.Context, system string, tools []brain.ToolDef, history []brain.Message, onTextDelta func(string)) (brain.Step, error) {
	select {
	case b.entered <- struct{}{}:
	default:
	}
	select {
	case <-b.release:
		return brain.TextStep("done"), nil
	case <-ctx.Done():
		return brain.Step{}, ctx.Err()
	}
}

func TestDoubleSubmit409(t *testing.T) {
	bb := &blockingBrain{entered: make(chan struct{}, 1), release: make(chan struct{})}
	h := newHarness(t, bb)
	agent := h.seedAgent(t, []string{})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	// Start run 1 in the background; it will block inside the brain.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r := h.post(t, "/api/sessions/"+cs.SessionID+"/run", `{"message":"hi"}`)
		readSSE(t, r.Body, nil)
		r.Body.Close()
	}()

	<-bb.entered // run 1 is now in flight (lock held)

	// Run 2 on the same session must be rejected.
	r2 := h.post(t, "/api/sessions/"+cs.SessionID+"/run", `{"message":"again"}`)
	if r2.StatusCode != http.StatusConflict {
		t.Errorf("second concurrent run = %d, want 409", r2.StatusCode)
	}
	r2.Body.Close()

	close(bb.release) // let run 1 finish
	wg.Wait()
}

func TestDeleteSessionInFlight409(t *testing.T) {
	bb := &blockingBrain{entered: make(chan struct{}, 1), release: make(chan struct{})}
	h := newHarness(t, bb)
	agent := h.seedAgent(t, []string{})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r := h.post(t, "/api/sessions/"+cs.SessionID+"/run", `{"message":"hi"}`)
		readSSE(t, r.Body, nil)
		r.Body.Close()
	}()

	<-bb.entered
	del := h.delete(t, "/api/sessions/"+cs.SessionID)
	if del.StatusCode != http.StatusConflict {
		t.Errorf("delete in-flight session = %d, want 409", del.StatusCode)
	}
	del.Body.Close()

	close(bb.release)
	wg.Wait()
}

func TestDeleteAgentInFlight409(t *testing.T) {
	bb := &blockingBrain{entered: make(chan struct{}, 1), release: make(chan struct{})}
	h := newHarness(t, bb)
	agent := h.seedAgent(t, []string{})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r := h.post(t, "/api/sessions/"+cs.SessionID+"/run", `{"message":"hi"}`)
		readSSE(t, r.Body, nil)
		r.Body.Close()
	}()

	<-bb.entered
	del := h.delete(t, "/api/agents/"+agent.ID)
	if del.StatusCode != http.StatusConflict {
		t.Errorf("delete agent with in-flight session = %d, want 409", del.StatusCode)
	}
	del.Body.Close()
	if _, err := h.store.GetAgent(context.Background(), agent.ID); err != nil {
		t.Errorf("agent should remain after 409: %v", err)
	}
	if _, err := h.store.GetSession(context.Background(), cs.SessionID); err != nil {
		t.Errorf("session should remain after 409: %v", err)
	}

	close(bb.release)
	wg.Wait()
}

func TestCancelDoesNotPersist(t *testing.T) {
	bb := &blockingBrain{entered: make(chan struct{}, 1), release: make(chan struct{})}
	h := newHarness(t, bb)
	agent := h.seedAgent(t, []string{})
	resp := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp.Body).Decode(&cs)
	resp.Body.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		h.srv.URL+"/api/sessions/"+cs.SessionID+"/run", strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("Content-Type", "application/json")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r, err := http.DefaultClient.Do(req)
		if err == nil {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}
	}()

	<-bb.entered // run is in flight inside the brain
	cancel()     // abort mid-run
	wg.Wait()

	// Give the server a moment to finish its (non-)persist path.
	time.Sleep(50 * time.Millisecond)
	sess, _ := h.store.GetSession(context.Background(), cs.SessionID)
	if len(sess.Messages) != 0 {
		t.Errorf("cancelled turn must not persist; got %d messages", len(sess.Messages))
	}
}

func TestErrorCases(t *testing.T) {
	h := newHarness(t, &brain.FakeBrain{})
	agent := h.seedAgent(t, []string{})

	// GET /api/agents returns [] (not null) when filtered to a fresh user is empty —
	// here at least the seeded agent exists, so assert it's a JSON array.
	resp, _ := http.Get(h.srv.URL + "/api/agents")
	var list []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Errorf("GET /api/agents should be a JSON array: %v", err)
	}
	resp.Body.Close()

	// unknown agent -> 404
	resp, _ = http.Get(h.srv.URL + "/api/agents/nope")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET unknown agent = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()

	// session for missing agent -> 404
	r := h.post(t, "/api/sessions", `{"agent_id":"ghost"}`)
	if r.StatusCode != http.StatusNotFound {
		t.Errorf("create session for missing agent = %d, want 404", r.StatusCode)
	}
	r.Body.Close()

	// run on missing session -> 404
	r = h.post(t, "/api/sessions/ghost/run", `{"message":"hi"}`)
	if r.StatusCode != http.StatusNotFound {
		t.Errorf("run on missing session = %d, want 404", r.StatusCode)
	}
	r.Body.Close()

	// empty message -> 400
	resp2 := h.post(t, "/api/sessions", `{"agent_id":"`+agent.ID+`"}`)
	var cs struct {
		SessionID string `json:"session_id"`
	}
	json.NewDecoder(resp2.Body).Decode(&cs)
	resp2.Body.Close()
	r = h.post(t, "/api/sessions/"+cs.SessionID+"/run", `{"message":""}`)
	if r.StatusCode != http.StatusBadRequest {
		t.Errorf("empty message = %d, want 400", r.StatusCode)
	}
	r.Body.Close()
}
