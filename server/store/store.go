// Package store is the persistence layer for live state: agent_specs (the agents) and
// sessions (conversations + resume history). The append-only audit trail is NOT here —
// it lives in the logs/ tree (00-CONTEXT.md §4b). The store touches the world (disk),
// so it sits on the orchestrator side of the boundary; the engine never imports it.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-builder/brain"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no cgo)
)

//go:embed schema.sql
var schemaSQL string

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("store: not found")

// DefaultUser is the single owner in v1 (no auth yet).
const DefaultUser = "local"

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// Agent is one agent_specs row in Go form.
type Agent struct {
	ID           string
	UserID       string
	Name         string
	Persona      string
	Instructions string
	Model        string
	Tools        []string
	Sources      []string
	Skills       []string
	CreatedAt    int64
	UpdatedAt    int64
}

const (
	SessionKindNormal    = "normal"
	SessionKindAgentEdit = "agent_edit"
)

// Session is one sessions row in Go form.
type Session struct {
	ID               string
	UserID           string
	AgentID          string
	Kind             string
	Title            string
	Messages         []brain.Message
	BuilderStateJSON string
	CreatedAt        int64
	UpdatedAt        int64
}

// AgentVersion is one applied agent-spec snapshot.
type AgentVersion struct {
	ID            string
	AgentID       string
	SessionID     string
	VersionNumber int
	SnapshotJSON  string
	ChangeSummary string
	CreatedAt     int64
}

// Open opens (creating if needed) the SQLite database at path and applies the schema.
// MaxOpenConns is pinned to 1: it serializes all access, which removes "database is
// locked" entirely — correct and simple for a single-node v1.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(context.Background(), "PRAGMA foreign_keys=ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: pragma: %w", err)
	}
	if _, err := db.ExecContext(context.Background(), schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate: %w", err)
	}
	if err := ensureColumn(context.Background(), db, "sessions", "builder_state_json", `TEXT NOT NULL DEFAULT '{}'`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate builder state: %w", err)
	}
	if err := ensureColumn(context.Background(), db, "agent_specs", "sources_json", `TEXT NOT NULL DEFAULT '[]'`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate sources: %w", err)
	}
	if err := ensureColumn(context.Background(), db, "sessions", "kind", `TEXT NOT NULL DEFAULT 'normal'`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate session kind: %w", err)
	}
	if err := ensureColumn(context.Background(), db, "sessions", "title", `TEXT NOT NULL DEFAULT 'New chat'`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate session title: %w", err)
	}
	if _, err := db.ExecContext(context.Background(), `CREATE INDEX IF NOT EXISTS idx_sessions_kind_agent ON sessions(kind, agent_id, updated_at)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate session kind index: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// --- agents ---

// InsertAgent inserts a new agent. ID, timestamps, and defaults are filled if unset.
// The completed Agent (with generated ID) is returned.
func (s *Store) InsertAgent(ctx context.Context, a Agent) (Agent, error) {
	if a.ID == "" {
		a.ID = newID("agent")
	}
	if a.UserID == "" {
		a.UserID = DefaultUser
	}
	if a.Model == "" {
		a.Model = "gpt-5.1"
	}
	now := time.Now().Unix()
	if a.CreatedAt == 0 {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	tools, sources, skills := marshalList(a.Tools), marshalList(a.Sources), marshalList(a.Skills)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_specs (id, user_id, name, persona, instructions, model, tools_json, sources_json, skills_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.UserID, a.Name, a.Persona, a.Instructions, a.Model, tools, sources, skills, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return Agent{}, fmt.Errorf("store: insert agent: %w", err)
	}
	return a, nil
}

// GetAgent returns the agent by id, or ErrNotFound.
func (s *Store) GetAgent(ctx context.Context, id string) (Agent, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, persona, instructions, model, tools_json, sources_json, skills_json, created_at, updated_at
		 FROM agent_specs WHERE id = ?`, id)
	return scanAgent(row)
}

// ListAgents returns all agents for a user (newest first).
func (s *Store) ListAgents(ctx context.Context, userID string) ([]Agent, error) {
	if userID == "" {
		userID = DefaultUser
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, persona, instructions, model, tools_json, sources_json, skills_json, created_at, updated_at
		 FROM agent_specs WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("store: list agents: %w", err)
	}
	defer rows.Close()
	out := []Agent{} // non-nil so the HTTP layer marshals [] not null
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAgent overwrites the mutable fields of an existing agent. ErrNotFound if absent.
func (s *Store) UpdateAgent(ctx context.Context, a Agent) error {
	a.UpdatedAt = time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		`UPDATE agent_specs SET name=?, persona=?, instructions=?, model=?, tools_json=?, sources_json=?, skills_json=?, updated_at=?
		 WHERE id=?`,
		a.Name, a.Persona, a.Instructions, a.Model, marshalList(a.Tools), marshalList(a.Sources), marshalList(a.Skills), a.UpdatedAt, a.ID)
	if err != nil {
		return fmt.Errorf("store: update agent: %w", err)
	}
	return mustAffectOne(res)
}

// UpdateAgentWithVersion atomically applies an agent update and records the resulting
// spec as a version caused by an agent-edit session.
func (s *Store) UpdateAgentWithVersion(ctx context.Context, a Agent, sessionID, summary string) error {
	if sessionID == "" {
		return fmt.Errorf("store: version session id is required")
	}
	a.UpdatedAt = time.Now().Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin versioned update: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE agent_specs SET name=?, persona=?, instructions=?, model=?, tools_json=?, sources_json=?, skills_json=?, updated_at=?
		 WHERE id=?`,
		a.Name, a.Persona, a.Instructions, a.Model, marshalList(a.Tools), marshalList(a.Sources), marshalList(a.Skills), a.UpdatedAt, a.ID)
	if err != nil {
		return fmt.Errorf("store: update agent: %w", err)
	}
	if err := mustAffectOne(res); err != nil {
		return err
	}

	var version int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version_number), 0) + 1 FROM agent_spec_versions WHERE agent_id = ?`, a.ID).Scan(&version); err != nil {
		return fmt.Errorf("store: next version: %w", err)
	}
	snapshot, err := json.Marshal(agentSnapshotFrom(a))
	if err != nil {
		return fmt.Errorf("store: encode version snapshot: %w", err)
	}
	if summary == "" {
		summary = "Updated agent"
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO agent_spec_versions (id, agent_id, session_id, version_number, snapshot_json, change_summary, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		newID("ver"), a.ID, sessionID, version, string(snapshot), summary, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: insert version: %w", err)
	}
	return tx.Commit()
}

// --- sessions ---

// CreateSession mints a session bound to an agent and returns it. It verifies the
// agent exists so a session can never point at a missing agent.
func (s *Store) CreateSession(ctx context.Context, userID, agentID string) (Session, error) {
	if userID == "" {
		userID = DefaultUser
	}
	if _, err := s.GetAgent(ctx, agentID); err != nil {
		return Session{}, err // ErrNotFound bubbles up
	}
	now := time.Now().Unix()
	sess := Session{ID: newID("sess"), UserID: userID, AgentID: agentID, Kind: SessionKindNormal, Title: "New chat", CreatedAt: now, UpdatedAt: now}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, agent_id, kind, title, messages_json, builder_state_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, '[]', '{}', ?, ?)`,
		sess.ID, sess.UserID, sess.AgentID, sess.Kind, sess.Title, sess.CreatedAt, sess.UpdatedAt)
	if err != nil {
		return Session{}, fmt.Errorf("store: create session: %w", err)
	}
	sess.BuilderStateJSON = "{}"
	return sess, nil
}

// CreateAgentEditSession mints a chat dedicated to editing one agent.
func (s *Store) CreateAgentEditSession(ctx context.Context, userID, agentID string) (Session, error) {
	if userID == "" {
		userID = DefaultUser
	}
	if _, err := s.GetAgent(ctx, agentID); err != nil {
		return Session{}, err
	}
	now := time.Now().Unix()
	sess := Session{ID: newID("sess"), UserID: userID, AgentID: agentID, Kind: SessionKindAgentEdit, Title: "New chat", CreatedAt: now, UpdatedAt: now, BuilderStateJSON: "{}"}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, agent_id, kind, title, messages_json, builder_state_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, '[]', '{}', ?, ?)`,
		sess.ID, sess.UserID, sess.AgentID, sess.Kind, sess.Title, sess.CreatedAt, sess.UpdatedAt)
	if err != nil {
		return Session{}, fmt.Errorf("store: create edit session: %w", err)
	}
	return sess, nil
}

// GetSession returns the session by id, or ErrNotFound.
func (s *Store) GetSession(ctx context.Context, id string) (Session, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, agent_id, kind, title, messages_json, builder_state_json, created_at, updated_at FROM sessions WHERE id = ?`, id)
	var sess Session
	var msgs string
	err := row.Scan(&sess.ID, &sess.UserID, &sess.AgentID, &sess.Kind, &sess.Title, &msgs, &sess.BuilderStateJSON, &sess.CreatedAt, &sess.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("store: get session: %w", err)
	}
	if err := json.Unmarshal([]byte(msgs), &sess.Messages); err != nil {
		return Session{}, fmt.Errorf("store: decode messages: %w", err)
	}
	if sess.BuilderStateJSON == "" {
		sess.BuilderStateJSON = "{}"
	}
	if sess.Kind == "" {
		sess.Kind = SessionKindNormal
	}
	if sess.Title == "" {
		sess.Title = "New chat"
	}
	return sess, nil
}

// AppendMessages atomically appends msgs to a session's history. Done in a transaction
// so a concurrent append can't clobber the read-modify-write.
func (s *Store) AppendMessages(ctx context.Context, sessionID string, msgs []brain.Message) error {
	return s.AppendMessagesAndBuilderState(ctx, sessionID, msgs, "")
}

// AppendMessagesAndBuilderState atomically appends msgs and optionally replaces the
// session's Builder state. Empty builderStateJSON means leave the state unchanged.
func (s *Store) AppendMessagesAndBuilderState(ctx context.Context, sessionID string, msgs []brain.Message, builderStateJSON string) error {
	if len(msgs) == 0 {
		if builderStateJSON == "" {
			return nil
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer tx.Rollback()

	var raw string
	err = tx.QueryRowContext(ctx, `SELECT messages_json FROM sessions WHERE id = ?`, sessionID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("store: read messages: %w", err)
	}
	var existing []brain.Message
	if err := json.Unmarshal([]byte(raw), &existing); err != nil {
		return fmt.Errorf("store: decode messages: %w", err)
	}
	existing = append(existing, msgs...)
	encoded, err := json.Marshal(existing)
	if err != nil {
		return fmt.Errorf("store: encode messages: %w", err)
	}
	now := time.Now().Unix()
	if builderStateJSON != "" {
		if !json.Valid([]byte(builderStateJSON)) {
			return fmt.Errorf("store: invalid builder state json")
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE sessions SET messages_json = ?, builder_state_json = ?, updated_at = ? WHERE id = ?`,
			string(encoded), builderStateJSON, now, sessionID); err != nil {
			return fmt.Errorf("store: write messages/state: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx,
			`UPDATE sessions SET messages_json = ?, updated_at = ? WHERE id = ?`,
			string(encoded), now, sessionID); err != nil {
			return fmt.Errorf("store: write messages: %w", err)
		}
	}
	return tx.Commit()
}

// ListSessionIDsForAgent returns the live chat ids currently bound to an agent.
func (s *Store) ListSessionIDsForAgent(ctx context.Context, agentID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM sessions WHERE agent_id = ? ORDER BY created_at DESC`, agentID)
	if err != nil {
		return nil, fmt.Errorf("store: list agent sessions: %w", err)
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: scan agent session: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListAgentEditSessions returns edit chats for all agents owned by userID.
func (s *Store) ListAgentEditSessions(ctx context.Context, userID string) ([]Session, error) {
	if userID == "" {
		userID = DefaultUser
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, agent_id, kind, title, messages_json, builder_state_json, created_at, updated_at
		 FROM sessions WHERE user_id = ? AND kind = ? ORDER BY updated_at DESC`,
		userID, SessionKindAgentEdit)
	if err != nil {
		return nil, fmt.Errorf("store: list edit sessions: %w", err)
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// UpdateSessionTitle updates the title shown for a chat in navigation.
func (s *Store) UpdateSessionTitle(ctx context.Context, id, title string) (Session, error) {
	if title == "" {
		title = "New chat"
	}
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `UPDATE sessions SET title = ?, updated_at = ? WHERE id = ?`, title, now, id)
	if err != nil {
		return Session{}, fmt.Errorf("store: update session title: %w", err)
	}
	if err := mustAffectOne(res); err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, id)
}

// ListAgentVersions returns applied spec snapshots for one agent, newest first.
func (s *Store) ListAgentVersions(ctx context.Context, agentID string) ([]AgentVersion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_id, session_id, version_number, snapshot_json, change_summary, created_at
		 FROM agent_spec_versions WHERE agent_id = ? ORDER BY version_number DESC`, agentID)
	if err != nil {
		return nil, fmt.Errorf("store: list versions: %w", err)
	}
	defer rows.Close()
	out := []AgentVersion{}
	for rows.Next() {
		var v AgentVersion
		if err := rows.Scan(&v.ID, &v.AgentID, &v.SessionID, &v.VersionNumber, &v.SnapshotJSON, &v.ChangeSummary, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scan version: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// DeleteSession removes one chat from live state. Audit logs, if any, are historical and
// intentionally left on disk.
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete session: %w", err)
	}
	return mustAffectOne(res)
}

// DeleteAgent removes an agent and its chats from live state in one transaction.
func (s *Store) DeleteAgent(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin delete agent: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM agent_spec_versions WHERE agent_id = ?`, id); err != nil {
		return fmt.Errorf("store: delete agent versions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE agent_id = ?`, id); err != nil {
		return fmt.Errorf("store: delete agent sessions: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM agent_specs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete agent: %w", err)
	}
	if err := mustAffectOne(res); err != nil {
		return err
	}
	return tx.Commit()
}

// --- helpers ---

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanAgent(sc scanner) (Agent, error) {
	var a Agent
	var toolsJSON, sourcesJSON, skillsJSON string
	err := sc.Scan(&a.ID, &a.UserID, &a.Name, &a.Persona, &a.Instructions, &a.Model, &toolsJSON, &sourcesJSON, &skillsJSON, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, fmt.Errorf("store: scan agent: %w", err)
	}
	if err := json.Unmarshal([]byte(toolsJSON), &a.Tools); err != nil {
		return Agent{}, fmt.Errorf("store: decode tools: %w", err)
	}
	if err := json.Unmarshal([]byte(sourcesJSON), &a.Sources); err != nil {
		return Agent{}, fmt.Errorf("store: decode sources: %w", err)
	}
	if err := json.Unmarshal([]byte(skillsJSON), &a.Skills); err != nil {
		return Agent{}, fmt.Errorf("store: decode skills: %w", err)
	}
	return a, nil
}

func scanSession(sc scanner) (Session, error) {
	var sess Session
	var msgs string
	err := sc.Scan(&sess.ID, &sess.UserID, &sess.AgentID, &sess.Kind, &sess.Title, &msgs, &sess.BuilderStateJSON, &sess.CreatedAt, &sess.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("store: scan session: %w", err)
	}
	if err := json.Unmarshal([]byte(msgs), &sess.Messages); err != nil {
		return Session{}, fmt.Errorf("store: decode messages: %w", err)
	}
	if sess.BuilderStateJSON == "" {
		sess.BuilderStateJSON = "{}"
	}
	if sess.Kind == "" {
		sess.Kind = SessionKindNormal
	}
	if sess.Title == "" {
		sess.Title = "New chat"
	}
	return sess, nil
}

type agentSnapshot struct {
	ID           string   `json:"id"`
	UserID       string   `json:"user_id"`
	Name         string   `json:"name"`
	Persona      string   `json:"persona"`
	Instructions string   `json:"instructions"`
	Model        string   `json:"model"`
	Tools        []string `json:"tools"`
	Sources      []string `json:"sources"`
	UpdatedAt    int64    `json:"updated_at"`
}

func agentSnapshotFrom(a Agent) agentSnapshot {
	tools := a.Tools
	if tools == nil {
		tools = []string{}
	}
	sources := a.Sources
	if sources == nil {
		sources = []string{}
	}
	return agentSnapshot{
		ID: a.ID, UserID: a.UserID, Name: a.Name, Persona: a.Persona,
		Instructions: a.Instructions, Model: a.Model, Tools: tools,
		Sources: sources, UpdatedAt: a.UpdatedAt,
	}
}

// marshalList encodes a string slice as a JSON array, never null (so tools_json is
// always a valid "[]" not "null").
func marshalList(xs []string) string {
	if xs == nil {
		xs = []string{}
	}
	b, _ := json.Marshal(xs)
	return string(b)
}

func mustAffectOne(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func newID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is catastrophic and must never silently mint a
		// zero/duplicate id; fail loudly instead.
		panic(fmt.Sprintf("store: crypto/rand failed: %v", err))
	}
	return prefix + "-" + hex.EncodeToString(b)
}

func ensureColumn(ctx context.Context, db *sql.DB, table, column, definition string) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition)
	return err
}
