package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agent-builder/brain"
	"agent-builder/tools"
)

// testRegistry builds a registry with the calculator plus a controllable "echo" tool
// and a "boom" tool that always errors — enough to drive every loop path.
func testRegistry() *tools.Registry {
	r := tools.NewRegistry()
	r.Register(tools.Calculator())
	r.Register(tools.Tool{
		Name: "echo", Description: "echo back", Parameters: json.RawMessage(`{"type":"object"}`),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) { return string(args), nil },
	})
	r.Register(tools.Tool{
		Name: "boom", Description: "always fails", Parameters: json.RawMessage(`{"type":"object"}`),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", errors.New("kaboom")
		},
	})
	return r
}

func collect() (Emit, *[]Event) {
	var got []Event
	return func(e Event) { got = append(got, e) }, &got
}

func typesOf(events []Event) []EventType {
	out := make([]EventType, len(events))
	for i, e := range events {
		out[i] = e.Type
	}
	return out
}

func nonStatusTypes(events []Event) []EventType {
	out := make([]EventType, 0, len(events))
	for _, e := range events {
		if e.Type != EventStatus {
			out = append(out, e.Type)
		}
	}
	return out
}

func statusMessages(events []Event) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		if e.Type == EventStatus {
			out = append(out, e.Message)
		}
	}
	return out
}

// The happy path: brain asks for a tool, gets the result, then answers.
func TestRun_ToolThenAnswer(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("c1", "calculator", `{"expression":"2+2"}`),
		brain.TextStep("The answer is 4."),
	}}
	emit, got := collect()
	out, err := Run(context.Background(), Input{
		InvocationID: "inv-1",
		Brain:        fb,
		Registry:     testRegistry(),
		Spec:         Spec{Instructions: "be terse", Tools: []string{"calculator"}},
		Message:      "what is 2+2?",
		Emit:         emit,
	})
	if err != nil {
		t.Fatalf("Run errored: %v", err)
	}
	if out.FinalText != "The answer is 4." {
		t.Errorf("FinalText = %q", out.FinalText)
	}
	// user, assistant(tool req), tool(result), assistant(final) = 4 messages
	if len(out.History) != 4 {
		t.Fatalf("History len = %d, want 4: %+v", len(out.History), out.History)
	}
	// The assistant's tool-request message MUST carry ToolCalls into history — this is
	// what the real OpenAI replay (toParamMessages) depends on. FakeBrain ignores
	// history shape, so without this assertion the test would pass for the wrong reason.
	if out.History[1].Role != brain.RoleAssistant || len(out.History[1].ToolCalls) != 1 {
		t.Errorf("assistant tool-request message must carry ToolCalls: %+v", out.History[1])
	}
	if out.History[1].ToolCalls[0].Name != "calculator" {
		t.Errorf("tool call name = %q, want calculator", out.History[1].ToolCalls[0].Name)
	}
	if out.History[2].Role != brain.RoleTool || out.History[2].Content != "4" || out.History[2].ToolCallID != "c1" {
		t.Errorf("tool message wrong: %+v", out.History[2])
	}
	// done.Text is authoritative; no separate final llm_text (00-CONTEXT §7).
	want := []EventType{EventToolUse, EventToolResult, EventDone}
	if !equalTypes(nonStatusTypes(*got), want) {
		t.Errorf("events = %v, want %v", typesOf(*got), want)
	}
	wantStatuses := []string{
		"Preparing response",
		"Checking available tools",
		"Thinking",
		"Calling calculator",
		"Reading tool result",
		"Drafting final response",
	}
	if strings.Join(statusMessages(*got), "|") != strings.Join(wantStatuses, "|") {
		t.Errorf("statuses = %v, want %v", statusMessages(*got), wantStatuses)
	}
	assertSeqMonotonic(t, *got)
	// the tool_result must be ok=true
	for _, e := range *got {
		if e.Type == EventToolResult && (e.OK == nil || !*e.OK) {
			t.Errorf("expected ok=true tool_result, got %+v", e)
		}
	}
}

// assertSeqMonotonic checks events carry seq 0,1,2,… with no gaps or repeats.
func assertSeqMonotonic(t *testing.T, events []Event) {
	t.Helper()
	for i, e := range events {
		if e.Seq != i {
			t.Errorf("event[%d] has seq %d, want %d (%+v)", i, e.Seq, i, e)
		}
	}
}

// A tool that errors must NOT crash the loop; the model gets a failed result and can
// still produce a final answer.
func TestRun_ToolErrorRecovers(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("c1", "boom", `{}`),
		brain.TextStep("ok, that failed, moving on."),
	}}
	emit, got := collect()
	out, err := Run(context.Background(), Input{
		InvocationID: "inv-2", Brain: fb, Registry: testRegistry(),
		Spec: Spec{Tools: []string{"boom"}}, Message: "go", Emit: emit,
	})
	if err != nil {
		t.Fatalf("tool error should be recoverable, got: %v", err)
	}
	if out.FinalText == "" {
		t.Error("expected a final answer after the tool error")
	}
	var sawFail bool
	for _, e := range *got {
		if e.Type == EventToolResult && e.OK != nil && !*e.OK {
			sawFail = true
		}
	}
	if !sawFail {
		t.Error("expected a failed tool_result event")
	}
}

// Malformed tool arguments are handled by the tool (unmarshal guard) -> failed result.
func TestRun_MalformedArgs(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("c1", "calculator", `{"expression":`), // truncated JSON
		brain.TextStep("recovered."),
	}}
	_, err := Run(context.Background(), Input{
		InvocationID: "inv-3", Brain: fb, Registry: testRegistry(),
		Spec: Spec{Tools: []string{"calculator"}}, Message: "go", Emit: func(Event) {},
	})
	if err != nil {
		t.Fatalf("malformed args should not be fatal, got: %v", err)
	}
}

// The loop cap must trigger if the brain keeps asking for tools forever.
func TestRun_LoopCap(t *testing.T) {
	steps := make([]brain.Step, MaxIterations+2)
	for i := range steps {
		steps[i] = brain.ToolStep("c", "echo", `{}`) // never finishes
	}
	fb := &brain.FakeBrain{Steps: steps}
	_, err := Run(context.Background(), Input{
		InvocationID: "inv-4", Brain: fb, Registry: testRegistry(),
		Spec: Spec{Tools: []string{"echo"}}, Message: "go", Emit: func(Event) {},
	})
	if err == nil {
		t.Fatal("expected an error when the loop cap is hit")
	}
}

// An unknown tool name in the spec fails the run up front (resolution error).
func TestRun_UnknownToolInSpec(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{brain.TextStep("hi")}}
	emit, got := collect()
	_, err := Run(context.Background(), Input{
		InvocationID: "inv-5", Brain: fb, Registry: testRegistry(),
		Spec: Spec{Tools: []string{"does_not_exist"}}, Message: "go", Emit: emit,
	})
	if err == nil {
		t.Fatal("expected resolution error for unknown tool")
	}
	if len(*got) == 0 || (*got)[len(*got)-1].Type != EventError {
		t.Errorf("expected an error event, got %v", typesOf(*got))
	}
}

// Cancelling the context aborts the run and emits an aborted event.
func TestRun_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the first iteration
	fb := &brain.FakeBrain{Steps: []brain.Step{brain.TextStep("should not reach")}}
	emit, got := collect()
	_, err := Run(ctx, Input{
		InvocationID: "inv-6", Brain: fb, Registry: testRegistry(),
		Spec: Spec{Tools: []string{"echo"}}, Message: "go", Emit: emit,
	})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	var sawAborted bool
	for _, e := range *got {
		if e.Type == EventAborted {
			sawAborted = true
		}
	}
	if !sawAborted {
		t.Error("expected an aborted event")
	}
}

// A plain-text first step (no tools) finishes immediately.
func TestRun_ImmediateAnswer(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{brain.TextStep("hello")}}
	out, err := Run(context.Background(), Input{
		InvocationID: "inv-7", Brain: fb, Registry: testRegistry(),
		Spec: Spec{}, Message: "hi", Emit: func(Event) {},
	})
	if err != nil {
		t.Fatalf("Run errored: %v", err)
	}
	if out.FinalText != "hello" || len(out.History) != 2 {
		t.Errorf("unexpected: final=%q history=%d", out.FinalText, len(out.History))
	}
}

// Two tool calls in one step must both run, in order, and both land in history.
func TestRun_MultipleToolCalls(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		{Kind: brain.StepToolCalls, ToolCalls: []brain.ToolCall{
			{ID: "c1", Name: "echo", Args: []byte(`{"x":1}`)},
			{ID: "c2", Name: "calculator", Args: []byte(`{"expression":"1+1"}`)},
		}},
		brain.TextStep("did both"),
	}}
	emit, got := collect()
	out, err := Run(context.Background(), Input{
		InvocationID: "inv-multi", Brain: fb, Registry: testRegistry(),
		Spec: Spec{Tools: []string{"echo", "calculator"}}, Message: "go", Emit: emit,
	})
	if err != nil {
		t.Fatalf("Run errored: %v", err)
	}
	// user, assistant(2 calls), tool(c1), tool(c2), assistant(final) = 5
	if len(out.History) != 5 {
		t.Fatalf("History len = %d, want 5: %+v", len(out.History), out.History)
	}
	if len(out.History[1].ToolCalls) != 2 {
		t.Errorf("assistant message should carry 2 tool calls: %+v", out.History[1])
	}
	want := []EventType{EventToolUse, EventToolResult, EventToolUse, EventToolResult, EventDone}
	if !equalTypes(nonStatusTypes(*got), want) {
		t.Errorf("events = %v, want %v", typesOf(*got), want)
	}
	assertSeqMonotonic(t, *got)
}

func TestRun_StreamedTextDeltas(t *testing.T) {
	fb := &brain.FakeBrain{
		Steps:      []brain.Step{brain.TextStep("Hello world.")},
		TextDeltas: [][]string{{"Hello", " world."}},
	}
	emit, got := collect()
	out, err := Run(context.Background(), Input{
		InvocationID: "inv-stream", Brain: fb, Registry: testRegistry(),
		Spec: Spec{}, Message: "hi", Emit: emit,
	})
	if err != nil {
		t.Fatalf("Run errored: %v", err)
	}
	if out.FinalText != "Hello world." {
		t.Fatalf("final = %q", out.FinalText)
	}
	want := []EventType{EventLLMText, EventLLMText, EventDone}
	if !equalTypes(nonStatusTypes(*got), want) {
		t.Fatalf("events = %v, want %v", typesOf(*got), want)
	}
	if (*got)[3].Text != "Hello" || (*got)[4].Text != " world." {
		t.Fatalf("stream deltas not preserved: %+v", *got)
	}
	assertSeqMonotonic(t, *got)
}

func TestRun_StreamedPreToolTextNotDuplicated(t *testing.T) {
	fb := &brain.FakeBrain{
		Steps: []brain.Step{
			{
				Kind: brain.StepToolCalls,
				Text: "Let me calculate.",
				ToolCalls: []brain.ToolCall{{
					ID: "c1", Name: "calculator", Args: []byte(`{"expression":"2+2"}`),
				}},
			},
			brain.TextStep("The answer is 4."),
		},
		TextDeltas: [][]string{{"Let me ", "calculate."}},
	}
	emit, got := collect()
	_, err := Run(context.Background(), Input{
		InvocationID: "inv-stream-tool", Brain: fb, Registry: testRegistry(),
		Spec: Spec{Tools: []string{"calculator"}}, Message: "go", Emit: emit,
	})
	if err != nil {
		t.Fatalf("Run errored: %v", err)
	}
	want := []EventType{EventLLMText, EventLLMText, EventToolUse, EventToolResult, EventDone}
	if !equalTypes(nonStatusTypes(*got), want) {
		t.Fatalf("events = %v, want %v", typesOf(*got), want)
	}
	var fullPreambleCount int
	for _, e := range *got {
		if e.Type == EventLLMText && e.Text == "Let me calculate." {
			fullPreambleCount++
		}
	}
	if fullPreambleCount != 0 {
		t.Fatalf("streamed pre-tool text was duplicated as a full preamble: %+v", *got)
	}
	assertSeqMonotonic(t, *got)
}

// Cancelling the context DURING a tool's execution must abort at the next loop check.
func TestRun_CancelDuringToolExec(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reg := tools.NewRegistry()
	reg.Register(tools.Tool{
		Name: "canceler", Description: "cancels the run", Parameters: json.RawMessage(`{"type":"object"}`),
		Execute: func(c context.Context, a json.RawMessage) (string, error) { cancel(); return "ok", nil },
	})
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("c1", "canceler", `{}`),
		brain.TextStep("should be unreachable"),
	}}
	emit, got := collect()
	out, err := Run(ctx, Input{
		InvocationID: "inv-cancel2", Brain: fb, Registry: reg,
		Spec: Spec{Tools: []string{"canceler"}}, Message: "go", Emit: emit,
	})
	if err == nil {
		t.Fatal("expected cancellation error after the tool cancelled the context")
	}
	if out.FinalText != "" {
		t.Errorf("should not have reached a final answer, got %q", out.FinalText)
	}
	var sawAborted bool
	for _, e := range *got {
		if e.Type == EventAborted {
			sawAborted = true
		}
	}
	if !sawAborted {
		t.Errorf("expected an aborted event, got %v", typesOf(*got))
	}
}

// regWithConsequential returns a registry plus a flag-pointer that records whether the
// consequential "save" tool actually executed.
func regWithConsequential() (*tools.Registry, *bool) {
	ran := new(bool)
	r := tools.NewRegistry()
	r.Register(tools.Tool{
		Name: "save", Description: "consequential save", Consequential: true,
		Parameters: json.RawMessage(`{"type":"object"}`),
		Execute:    func(ctx context.Context, a json.RawMessage) (string, error) { *ran = true; return "saved", nil },
	})
	return r, ran
}

func regWithCreateAgent() (*tools.Registry, *bool) {
	ran := new(bool)
	r := tools.NewRegistry()
	r.Register(tools.Tool{
		Name: "create_agent", Description: "create agent", Consequential: true,
		Parameters: json.RawMessage(`{"type":"object"}`),
		Execute:    func(ctx context.Context, a json.RawMessage) (string, error) { *ran = true; return "created", nil },
	})
	return r, ran
}

func regWithQuestionTool() *tools.Registry {
	r := tools.NewRegistry()
	r.Register(tools.Tool{
		Name: "ask_user_question", Description: "ask", UserInput: true,
		Parameters: json.RawMessage(`{"type":"object"}`),
		Execute: func(ctx context.Context, a json.RawMessage) (string, error) {
			return "", errors.New("should not execute")
		},
	})
	return r
}

func TestRun_BuilderStatusEvents(t *testing.T) {
	reg, ran := regWithCreateAgent()
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("c1", "create_agent", `{"name":"A"}`),
		brain.TextStep("created it."),
	}}
	emit, got := collect()
	_, err := Run(context.Background(), Input{
		InvocationID: "inv-status", Brain: fb, Registry: reg,
		Spec: Spec{Tools: []string{"create_agent"}}, Message: "build", Emit: emit,
		Confirm: func(context.Context, brain.ToolCall) (func() (bool, error), error) {
			return func() (bool, error) { return true, nil }, nil
		},
	})
	if err != nil {
		t.Fatalf("Run errored: %v", err)
	}
	if !*ran {
		t.Fatal("approved create_agent should execute")
	}
	statuses := statusMessages(*got)
	want := []string{
		"Preparing agent brief",
		"Checking available tools",
		"Drafting agent spec",
		"Waiting for approval",
		"Creating agent",
		"Saving agent",
		"Drafting agent spec",
	}
	if strings.Join(statuses, "|") != strings.Join(want, "|") {
		t.Fatalf("statuses = %v, want %v; events=%v", statuses, want, typesOf(*got))
	}
	assertSeqMonotonic(t, *got)
}

func TestRun_UserQuestionContinuesWithAnswer(t *testing.T) {
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("q1", "ask_user_question", `{"field":"fetch_mode"}`),
		brain.TextStep("ready to build."),
	}}
	emit, got := collect()
	out, err := Run(context.Background(), Input{
		InvocationID: "inv-question",
		Brain:        fb,
		Registry:     regWithQuestionTool(),
		Spec:         Spec{Tools: []string{"ask_user_question"}},
		Message:      "build",
		Emit:         emit,
		AskUser: func(ctx context.Context, call brain.ToolCall) (Event, func() (string, error), error) {
			return Event{
					Field:       "fetch_mode",
					Question:    "How should it fetch?",
					Options:     []QuestionOption{{ID: "both", Label: "Both", Value: "both"}},
					AllowCustom: true,
				},
				func() (string, error) {
					return `{"field":"fetch_mode","answer":"Both","value":"both","option_id":"both","custom":false}`, nil
				}, nil
		},
	})
	if err != nil {
		t.Fatalf("Run errored: %v", err)
	}
	if out.FinalText != "ready to build." {
		t.Fatalf("final = %q", out.FinalText)
	}
	want := []EventType{EventUserQuestion, EventToolResult, EventDone}
	if !equalTypes(nonStatusTypes(*got), want) {
		t.Fatalf("events = %v, want %v", typesOf(*got), want)
	}
	var question Event
	for _, e := range *got {
		if e.Type == EventUserQuestion {
			question = e
			break
		}
	}
	if question.Question != "How should it fetch?" {
		t.Fatalf("question event = %+v", question)
	}
	if out.History[2].Role != brain.RoleTool || !strings.Contains(out.History[2].Content, `"value":"both"`) {
		t.Fatalf("answer should be recorded as tool result: %+v", out.History)
	}
	assertSeqMonotonic(t, *got)
}

// A consequential tool must be gated: denial means it never runs and the model gets a
// "denied" result and continues.
func TestRun_ConfirmDeny(t *testing.T) {
	reg, ran := regWithConsequential()
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("c1", "save", `{}`),
		brain.TextStep("ok, not saving."),
	}}
	emit, got := collect()
	out, err := Run(context.Background(), Input{
		InvocationID: "inv-deny", Brain: fb, Registry: reg,
		Spec: Spec{Tools: []string{"save"}}, Message: "go", Emit: emit,
		Confirm: func(context.Context, brain.ToolCall) (func() (bool, error), error) {
			return func() (bool, error) { return false, nil }, nil
		},
	})
	if err != nil {
		t.Fatalf("Run errored: %v", err)
	}
	if *ran {
		t.Error("denied consequential tool must NOT execute")
	}
	if out.FinalText != "ok, not saving." {
		t.Errorf("expected recovery answer, got %q", out.FinalText)
	}
	var sawConfirm, sawDenied bool
	for _, e := range *got {
		if e.Type == EventConfirm {
			sawConfirm = true
		}
		if e.Type == EventToolResult && e.Data == "denied by user" {
			sawDenied = true
		}
	}
	if !sawConfirm || !sawDenied {
		t.Errorf("expected confirm + denied result; events=%v", typesOf(*got))
	}
}

// Approval runs the consequential tool.
func TestRun_ConfirmApprove(t *testing.T) {
	reg, ran := regWithConsequential()
	fb := &brain.FakeBrain{Steps: []brain.Step{
		brain.ToolStep("c1", "save", `{}`),
		brain.TextStep("saved it."),
	}}
	emit, got := collect()
	_, err := Run(context.Background(), Input{
		InvocationID: "inv-appr", Brain: fb, Registry: reg,
		Spec: Spec{Tools: []string{"save"}}, Message: "go", Emit: emit,
		Confirm: func(context.Context, brain.ToolCall) (func() (bool, error), error) {
			return func() (bool, error) { return true, nil }, nil
		},
	})
	if err != nil {
		t.Fatalf("Run errored: %v", err)
	}
	if !*ran {
		t.Error("approved consequential tool must execute")
	}
	assertSeqMonotonic(t, *got)
}

func equalTypes(a, b []EventType) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
