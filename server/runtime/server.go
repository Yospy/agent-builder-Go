// Package runtime is the orchestrator: the only thing with an inbound port. It gathers
// (load session->agent->history, build deps), drives the stateless engine, relays its
// events to SSE, and persists results. It never composes prompts or runs the loop —
// that is the engine's job (00-CONTEXT §15, invariant 2).
package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-playground/validator/v10"

	"agent-builder/brain"
	"agent-builder/store"
	"agent-builder/tools"
)

// builderAgentID mirrors platformtools.BuilderID without importing it (avoids a cycle:
// platformtools imports tools/store; runtime imports those too, but keeping the literal
// here keeps runtime independent of platformtools).
const builderAgentID = "builder"

// Server holds the orchestrator's dependencies.
type Server struct {
	store           *store.Store
	reg             *tools.Registry
	brain           brain.Brain
	logsDir         string
	webDir          string
	log             *slog.Logger
	validate        *validator.Validate
	confirms        *confirms
	questions       *questions
	inflight        *inflight
	titleSummarizer titleSummarizer
}

// NewServer wires the orchestrator. All dependencies are injected.
func NewServer(st *store.Store, reg *tools.Registry, b brain.Brain, logsDir, webDir string, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		store:     st,
		reg:       reg,
		brain:     b,
		logsDir:   logsDir,
		webDir:    webDir,
		log:       log,
		validate:  validator.New(),
		confirms:  newConfirms(),
		questions: newQuestions(),
		inflight:  newInflight(),
	}
}

// Handler returns the HTTP router (Go 1.22 method+wildcard patterns; no external router).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/agents", s.handleListAgents)
	mux.HandleFunc("GET /api/agents/{id}", s.handleGetAgent)
	mux.HandleFunc("GET /api/agents/{id}/versions", s.handleListAgentVersions)
	mux.HandleFunc("POST /api/agents/{id}/edit-sessions", s.handleCreateAgentEditSession)
	mux.HandleFunc("GET /api/agent-edit-sessions", s.handleListAgentEditSessions)
	mux.HandleFunc("DELETE /api/agents/{id}", s.handleDeleteAgent)
	mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	mux.HandleFunc("PATCH /api/sessions/{id}", s.handleUpdateSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("POST /api/sessions/{id}/run", s.handleRun)
	mux.HandleFunc("POST /api/sessions/{id}/approve", s.handleApprove)
	mux.HandleFunc("POST /api/sessions/{id}/answer", s.handleAnswer)
	mux.HandleFunc("POST /api/chat-title", s.handleChatTitle)
	mux.Handle("GET /", s.staticHandler())
	return mux
}

// staticHandler serves the UI from webDir if it exists; otherwise a tiny placeholder so
// the root doesn't 404 before Phase 4 lands.
func (s *Server) staticHandler() http.Handler {
	if s.webDir != "" {
		if info, err := os.Stat(s.webDir); err == nil && info.IsDir() {
			return http.FileServer(http.Dir(s.webDir))
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("agent-builder API is running. UI lands in Phase 4.\n"))
	})
}

// --- DTOs ---

type agentDTO struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Persona      string   `json:"persona"`
	Instructions string   `json:"instructions"`
	Model        string   `json:"model"`
	Tools        []string `json:"tools"`
	Sources      []string `json:"sources"`
	CreatedAt    int64    `json:"created_at"`
	UpdatedAt    int64    `json:"updated_at"`
}

func toAgentDTO(a store.Agent) agentDTO {
	tools := a.Tools
	if tools == nil {
		tools = []string{}
	}
	sources := a.Sources
	if sources == nil {
		sources = []string{}
	}
	return agentDTO{
		ID: a.ID, Name: a.Name, Persona: a.Persona, Instructions: a.Instructions,
		Model: a.Model, Tools: tools, Sources: sources, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

type sessionDTO struct {
	ID        string          `json:"id"`
	AgentID   string          `json:"agent_id"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Messages  []brain.Message `json:"messages"`
	CreatedAt int64           `json:"created_at"`
	UpdatedAt int64           `json:"updated_at"`
}

func toSessionDTO(sess store.Session, includeMessages bool) sessionDTO {
	msgs := sess.Messages
	if msgs == nil {
		msgs = []brain.Message{}
	}
	out := sessionDTO{
		ID: sess.ID, AgentID: sess.AgentID, Kind: sess.Kind, Title: sess.Title,
		Messages: msgs, CreatedAt: sess.CreatedAt, UpdatedAt: sess.UpdatedAt,
	}
	if !includeMessages {
		out.Messages = []brain.Message{}
	}
	return out
}

type agentVersionDTO struct {
	ID            string `json:"id"`
	AgentID       string `json:"agent_id"`
	SessionID     string `json:"session_id"`
	VersionNumber int    `json:"version_number"`
	SnapshotJSON  string `json:"snapshot_json"`
	ChangeSummary string `json:"change_summary"`
	CreatedAt     int64  `json:"created_at"`
}

func toAgentVersionDTO(v store.AgentVersion) agentVersionDTO {
	return agentVersionDTO{
		ID: v.ID, AgentID: v.AgentID, SessionID: v.SessionID,
		VersionNumber: v.VersionNumber, SnapshotJSON: v.SnapshotJSON,
		ChangeSummary: v.ChangeSummary, CreatedAt: v.CreatedAt,
	}
}

// --- agent handlers ---

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	ctx := store.WithUser(r.Context(), store.DefaultUser)
	agents, err := s.store.ListAgents(ctx, store.DefaultUser)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not list agents")
		return
	}
	out := make([]agentDTO, 0, len(agents))
	for _, a := range agents {
		out = append(out, toAgentDTO(a))
	}
	s.writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	a, err := s.store.GetAgent(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		s.writeErr(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not load agent")
		return
	}
	s.writeJSON(w, http.StatusOK, toAgentDTO(a))
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == builderAgentID {
		s.writeErr(w, http.StatusBadRequest, "agent builder cannot be deleted")
		return
	}
	sessionIDs, err := s.store.ListSessionIDsForAgent(r.Context(), id)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not delete agent")
		return
	}
	locked := make([]string, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if !s.inflight.acquire(sessionID) {
			for _, acquired := range locked {
				s.inflight.release(acquired)
			}
			s.writeErr(w, http.StatusConflict, "a run is already in flight for this agent")
			return
		}
		locked = append(locked, sessionID)
	}
	defer func() {
		for _, sessionID := range locked {
			s.inflight.release(sessionID)
		}
	}()

	if err := s.store.DeleteAgent(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		s.writeErr(w, http.StatusNotFound, "agent not found")
		return
	} else if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not delete agent")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateAgentEditSession(w http.ResponseWriter, r *http.Request) {
	ctx := store.WithUser(r.Context(), store.DefaultUser)
	sess, err := s.store.CreateAgentEditSession(ctx, store.DefaultUser, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		s.writeErr(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not create edit session")
		return
	}
	s.writeJSON(w, http.StatusCreated, toSessionDTO(sess, false))
}

func (s *Server) handleListAgentEditSessions(w http.ResponseWriter, r *http.Request) {
	ctx := store.WithUser(r.Context(), store.DefaultUser)
	sessions, err := s.store.ListAgentEditSessions(ctx, store.DefaultUser)
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not list edit sessions")
		return
	}
	out := make([]sessionDTO, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, toSessionDTO(sess, false))
	}
	s.writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListAgentVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := s.store.ListAgentVersions(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not list versions")
		return
	}
	out := make([]agentVersionDTO, 0, len(versions))
	for _, version := range versions {
		out = append(out, toAgentVersionDTO(version))
	}
	s.writeJSON(w, http.StatusOK, out)
}

// --- session handlers ---

type createSessionReq struct {
	AgentID string `json:"agent_id" validate:"required"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var body createSessionReq
	if !s.decode(w, r, &body) {
		return
	}
	ctx := store.WithUser(r.Context(), store.DefaultUser)
	sess, err := s.store.CreateSession(ctx, store.DefaultUser, body.AgentID)
	if errors.Is(err, store.ErrNotFound) {
		s.writeErr(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not create session")
		return
	}
	s.writeJSON(w, http.StatusCreated, map[string]string{"session_id": sess.ID, "agent_id": sess.AgentID})
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sess, err := s.store.GetSession(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		s.writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not load session")
		return
	}
	s.writeJSON(w, http.StatusOK, toSessionDTO(sess, true))
}

type updateSessionReq struct {
	Title string `json:"title" validate:"required,max=80"`
}

func (s *Server) handleUpdateSession(w http.ResponseWriter, r *http.Request) {
	var body updateSessionReq
	if !s.decode(w, r, &body) {
		return
	}
	sess, err := s.store.UpdateSessionTitle(r.Context(), r.PathValue("id"), body.Title)
	if errors.Is(err, store.ErrNotFound) {
		s.writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not update session")
		return
	}
	s.writeJSON(w, http.StatusOK, toSessionDTO(sess, false))
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if !s.inflight.acquire(sessionID) {
		s.writeErr(w, http.StatusConflict, "a run is already in flight for this session")
		return
	}
	defer s.inflight.release(sessionID)

	if err := s.store.DeleteSession(r.Context(), sessionID); errors.Is(err, store.ErrNotFound) {
		s.writeErr(w, http.StatusNotFound, "session not found")
		return
	} else if err != nil {
		s.writeErr(w, http.StatusInternalServerError, "could not delete session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- shared helpers ---

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Error("write json", "err", err)
	}
}

func (s *Server) writeErr(w http.ResponseWriter, status int, msg string) {
	s.writeJSON(w, status, map[string]string{"error": msg})
}

// maxBodyBytes caps request bodies so a giant payload can't OOM the process (the
// validator's max runs only after a full decode, so it can't protect memory).
const maxBodyBytes = 1 << 20 // 1 MiB

// decode reads and validates a JSON body, writing a 400 and returning false on failure.
func (s *Server) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		s.writeErr(w, http.StatusBadRequest, "invalid or too-large JSON body")
		return false
	}
	if err := s.validate.Struct(dst); err != nil {
		s.writeErr(w, http.StatusBadRequest, "validation failed: "+err.Error())
		return false
	}
	return true
}

func genID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("runtime: crypto/rand failed: " + err.Error())
	}
	return prefix + "-" + hex.EncodeToString(b)
}
