package store

import "context"

// userKey carries the current owner through a request. In v1 it is always DefaultUser;
// the seam exists so Phase-3 auth can set the real principal without changing call sites.
type userKey struct{}
type editSessionKey struct{}

// EditSession identifies an agent-edit chat carried on a request context.
// Platform tools use this to constrain update_agent to the edited agent and to
// attach version snapshots to the edit chat that caused them.
type EditSession struct {
	SessionID string
	AgentID   string
}

// WithUser returns a context scoped to user id.
func WithUser(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userKey{}, id)
}

// UserFrom returns the user on the context, or DefaultUser if none is set.
func UserFrom(ctx context.Context) string {
	if id, ok := ctx.Value(userKey{}).(string); ok && id != "" {
		return id
	}
	return DefaultUser
}

// WithEditSession scopes a request to one agent-edit chat.
func WithEditSession(ctx context.Context, sessionID, agentID string) context.Context {
	return context.WithValue(ctx, editSessionKey{}, EditSession{SessionID: sessionID, AgentID: agentID})
}

// EditSessionFrom returns the edit-session scope on the context, if any.
func EditSessionFrom(ctx context.Context) (EditSession, bool) {
	es, ok := ctx.Value(editSessionKey{}).(EditSession)
	return es, ok && es.SessionID != "" && es.AgentID != ""
}
