-- Live app state (audit lives in logs/, not here). See docs/00-CONTEXT.md §4a.

CREATE TABLE IF NOT EXISTS agent_specs (
  id           TEXT PRIMARY KEY,
  user_id      TEXT    NOT NULL DEFAULT 'local',   -- owner seam (single user in v1)
  name         TEXT    NOT NULL,
  persona      TEXT    NOT NULL DEFAULT '',
  instructions TEXT    NOT NULL DEFAULT '',
  model        TEXT    NOT NULL DEFAULT 'gpt-5.1',
  tools_json   TEXT    NOT NULL DEFAULT '[]',        -- tool NAMES, not code
  sources_json TEXT    NOT NULL DEFAULT '[]',
  skills_json  TEXT    NOT NULL DEFAULT '[]',
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_specs_user ON agent_specs(user_id);

CREATE TABLE IF NOT EXISTS sessions (
  id            TEXT    PRIMARY KEY,                 -- session_id, minted per conversation
  user_id       TEXT    NOT NULL DEFAULT 'local',
  agent_id      TEXT    NOT NULL,                    -- the binding (frozen at mint)
  kind          TEXT    NOT NULL DEFAULT 'normal',    -- normal | agent_edit
  title         TEXT    NOT NULL DEFAULT 'New chat',
  messages_json TEXT    NOT NULL DEFAULT '[]',       -- full message history (the resume)
  builder_state_json TEXT NOT NULL DEFAULT '{}',      -- Builder-only guided setup state
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  FOREIGN KEY (agent_id) REFERENCES agent_specs(id)  -- a session can't point at a missing agent
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id, agent_id);

CREATE TABLE IF NOT EXISTS agent_spec_versions (
  id             TEXT    PRIMARY KEY,
  agent_id       TEXT    NOT NULL,
  session_id     TEXT    NOT NULL,
  version_number INTEGER NOT NULL,
  snapshot_json  TEXT    NOT NULL,
  change_summary TEXT    NOT NULL DEFAULT '',
  created_at     INTEGER NOT NULL,
  FOREIGN KEY (agent_id) REFERENCES agent_specs(id)
);
CREATE INDEX IF NOT EXISTS idx_agent_versions_agent ON agent_spec_versions(agent_id, version_number DESC);
