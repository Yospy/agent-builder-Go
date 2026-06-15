package runtime

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"agent-builder/brain"
	"agent-builder/engine"
	"agent-builder/store"
)

type runReq struct {
	Message string `json:"message" validate:"required,max=10000"`
}

// handleRun is the turn endpoint: gather -> engine.Run -> stream events -> persist.
// All validation and 404/409 checks happen BEFORE the SSE headers go out, because once
// streaming starts the status code is fixed at 200.
func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	var body runReq
	if !s.decode(w, r, &body) {
		return
	}
	s.log.Info("run request received", "session", sessionID, "message_len", len(body.Message))

	// One run per session at a time.
	if !s.inflight.acquire(sessionID) {
		s.writeErr(w, http.StatusConflict, "a run is already in flight for this session")
		return
	}
	defer s.inflight.release(sessionID)

	ctx := store.WithUser(r.Context(), store.DefaultUser)

	sess, err := s.store.GetSession(ctx, sessionID)
	if errors.Is(err, store.ErrNotFound) {
		s.writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not load session")
		return
	}
	agent, err := s.store.GetAgent(ctx, sess.AgentID)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not load agent")
		return
	}
	s.log.Info("run context loaded", "session", sessionID, "agent", agent.ID, "tools", agent.Tools, "history_len", len(sess.Messages))
	runSpec := engine.Spec{Persona: agent.Persona, Instructions: agent.Instructions, Tools: agent.Tools}
	if sess.Kind == store.SessionKindAgentEdit {
		ctx = store.WithEditSession(ctx, sess.ID, agent.ID)
		runSpec = engine.Spec{
			Persona:      "A precise agent editor that updates one existing agent.",
			Instructions: agentEditorInstructions(agent),
			Tools:        []string{"list_tools", "ask_user_question", "update_agent"},
		}
	}

	// From here on we stream — no more JSON error responses.
	sse, err := newSSE(w)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	invocationID := genID("inv")
	log := s.log.With("session", sessionID, "agent", agent.ID, "invocation", invocationID)
	log.Info("sse opened")

	audit, err := openAuditLog(s.logsDir, agent.ID, sessionID, invocationID)
	if err != nil {
		log.Warn("audit log unavailable; continuing without it", "err", err)
		audit = nil
	}

	emit := func(e engine.Event) {
		if audit != nil {
			if werr := audit.write(time.Now().Unix(), e); werr != nil {
				log.Warn("audit write failed", "err", werr)
			}
		}
		if err := sse.send(e); err != nil {
			log.Warn("sse send failed", "type", e.Type, "seq", e.Seq, "err", err)
			return
		}
		log.Info("sse sent", "type", e.Type, "seq", e.Seq, "call_id", e.CallID, "name", e.Name)
	}

	confirm := func(ctx context.Context, call brain.ToolCall) (func() (bool, error), error) {
		// Register the wait channel NOW — before the engine emits the confirm event —
		// so a fast /approve can never beat the registration and spuriously 404.
		key := sessionID + ":" + call.ID
		ch := s.confirms.register(key)
		log.Info("confirm registered", "call_id", call.ID, "tool", call.Name)
		wait := func() (bool, error) {
			defer s.confirms.drop(key)
			select {
			case <-ctx.Done():
				log.Warn("confirm wait cancelled", "call_id", call.ID, "tool", call.Name, "err", ctx.Err())
				return false, ctx.Err()
			case approved := <-ch:
				log.Info("confirm delivered", "call_id", call.ID, "tool", call.Name, "approved", approved)
				return approved, nil
			}
		}
		return wait, nil
	}

	builderState := parseBuilderState(sess.BuilderStateJSON)

	// Root span for the whole turn: the OpenAI tracing middleware nests each LLM call
	// under it, so one rollout reads as one trace. No-op when tracing is disabled
	// (global provider is a no-op), so this is always safe to wrap.
	ctx, span := otel.Tracer("agent-builder").Start(ctx, "agent.run",
		trace.WithAttributes(
			attribute.String("agent.id", agent.ID),
			attribute.String("session.id", sessionID),
			attribute.String("invocation.id", invocationID),
		))
	defer span.End()

	askUser := func(ctx context.Context, call brain.ToolCall) (engine.Event, func() (string, error), error) {
		args, err := parseQuestionArgs(call.Args)
		if err != nil {
			return engine.Event{}, nil, err
		}
		key := sessionID + ":" + call.ID
		ch := s.questions.register(key, args)
		log.Info("question registered", "call_id", call.ID, "field", args.Field, "options", len(args.Options), "allow_custom", args.AllowCustom)
		builderState.Questions = append(builderState.Questions, builderQuestion{
			CallID:   call.ID,
			Field:    args.Field,
			Question: args.Question,
		})
		wait := func() (string, error) {
			defer s.questions.drop(key)
			select {
			case <-ctx.Done():
				log.Warn("question wait cancelled", "call_id", call.ID, "field", args.Field, "err", ctx.Err())
				return "", ctx.Err()
			case answer := <-ch:
				log.Info("question delivered", "call_id", call.ID, "field", answer.Field, "answer_len", len(answer.Answer))
				builderState.recordAnswer(call.ID, answer.Field, answer.Question, answer.Answer, answer.Value)
				return answerJSON(answer), nil
			}
		}
		return questionEvent(args), wait, nil
	}

	out, runErr := engine.Run(ctx, engine.Input{
		InvocationID: invocationID,
		Brain:        s.brain,
		Registry:     s.reg,
		Spec:         runSpec,
		History:      sess.Messages,
		Message:      body.Message,
		Emit:         emit,
		Confirm:      confirm,
		AskUser:      askUser,
		Logger:       s.log,
	})
	if runErr != nil {
		span.RecordError(runErr)
	}
	log.Info("engine returned", "err", runErr, "history_len", len(out.History), "final_text_len", len(out.FinalText))
	if audit != nil {
		_ = audit.close()
	}

	if runErr != nil {
		// The engine already emitted aborted/error. Do NOT persist the partial turn
		// (00-CONTEXT §9): a cancelled/failed turn never happened for continuity.
		log.Warn("run ended without completion", "err", runErr)
		return
	}

	// Persist only the new messages this turn added (user + assistant + tool messages).
	// Use a context detached from the request so a client disconnect in the instant
	// between completion and commit can't drop a COMPLETED turn (00-CONTEXT §9).
	newMsgs := out.History[len(sess.Messages):]
	builderStateJSON := ""
	if agent.ID == builderAgentID {
		builderStateJSON = builderState.json()
	}
	log.Info("persist start", "new_messages", len(newMsgs), "builder_state_bytes", len(builderStateJSON))
	if err := s.store.AppendMessagesAndBuilderState(context.WithoutCancel(ctx), sessionID, newMsgs, builderStateJSON); err != nil {
		log.Error("persist failed", "err", err)
		return
	}
	log.Info("persist done", "new_messages", len(newMsgs))
}

func agentEditorInstructions(agent store.Agent) string {
	sources := agent.Sources
	if sources == nil {
		sources = []string{}
	}
	return strings.TrimSpace(`You edit exactly one existing agent.

Target agent id: ` + agent.ID + `
Target agent name: ` + agent.Name + `
Current model: ` + agent.Model + `
Current tools: ` + strings.Join(agent.Tools, ", ") + `
Current sources: ` + strings.Join(sources, ", ") + `
Current persona:
` + agent.Persona + `

Current system instructions:
` + agent.Instructions + `

When the user asks for a change, propose and apply it by calling update_agent with id "` + agent.ID + `".
Only include fields that should change; use null for unchanged fields when required by the tool schema.
Use list_tools before changing tools if you need the available tool names.
Use ask_user_question only when a required detail is genuinely missing.
Never create a new agent from this chat. Never update any other agent.`)
}

type approveReq struct {
	CallID   string `json:"call_id" validate:"required"`
	Decision string `json:"decision" validate:"required,oneof=approve deny"`
}

// handleApprove delivers an approve/deny decision to a paused consequential tool. The
// continuation streams on the original /run connection; this just unblocks it.
func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	var body approveReq
	if !s.decode(w, r, &body) {
		return
	}
	key := sessionID + ":" + body.CallID
	if !s.confirms.deliver(key, body.Decision == "approve") {
		s.log.Warn("approve rejected; no pending confirmation", "session", sessionID, "call_id", body.CallID)
		s.writeErr(w, http.StatusNotFound, "no pending confirmation for this call_id")
		return
	}
	s.log.Info("approve accepted", "session", sessionID, "call_id", body.CallID, "decision", body.Decision)
	s.writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

// handleAnswer delivers a structured answer to a paused ask_user_question tool. The
// continuation streams on the original /run connection; this just unblocks it.
func (s *Server) handleAnswer(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	var body answerReq
	if !s.decode(w, r, &body) {
		return
	}
	key := sessionID + ":" + body.CallID
	found, status, msg := s.questions.deliver(key, body)
	if !found {
		s.log.Warn("answer rejected; no pending question", "session", sessionID, "call_id", body.CallID)
		s.writeErr(w, http.StatusNotFound, "no pending question for this call_id")
		return
	}
	if status != 0 {
		s.log.Warn("answer rejected", "session", sessionID, "call_id", body.CallID, "status", status, "msg", msg)
		s.writeErr(w, status, msg)
		return
	}
	s.log.Info("answer accepted", "session", sessionID, "call_id", body.CallID)
	s.writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}
