package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"agent-builder/engine"
)

// auditLog is the append-only JSONL writer for one rollout (00-CONTEXT §4b). One file
// per invocation, nested under builder/ or agents/<id>/. Immutable: we only ever append.
type auditLog struct {
	f   *os.File
	enc *json.Encoder
}

// logLine is one audit record: the normalized event plus a timestamp. The embedded
// Event's fields are inlined into the same JSON object.
type logLine struct {
	TS int64 `json:"ts"`
	engine.Event
}

// openAuditLog creates logs/<builder|agents/<agentID>>/<sessionID>/<invocationID>.jsonl.
// The Builder's own rollouts go under builder/; every other agent under agents/<id>/.
func openAuditLog(logsDir, agentID, sessionID, invocationID string) (*auditLog, error) {
	var dir string
	if agentID == builderAgentID {
		dir = filepath.Join(logsDir, "builder", sessionID)
	} else {
		dir = filepath.Join(logsDir, "agents", agentID, sessionID)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("audit: mkdir: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, invocationID+".jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("audit: open: %w", err)
	}
	return &auditLog{f: f, enc: json.NewEncoder(f)}, nil
}

// write appends one event line. Errors are returned to the caller, which logs but does
// not abort the turn over an audit hiccup.
func (a *auditLog) write(ts int64, e engine.Event) error {
	return a.enc.Encode(logLine{TS: ts, Event: e})
}

func (a *auditLog) close() error { return a.f.Close() }
