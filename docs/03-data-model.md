# Data Model — Agents as Rows

The whole platform rests on: **an agent is a row, the engine is the only code.** The row
is a *manifest of references + identity content* — it names what an agent is and what it
may use, but holds no code.

## Tables (SQLite, v1)

```sql
-- THE AGENTS. Each row is one agent. New agent = INSERT. Edit = UPDATE.
CREATE TABLE agent_specs (
  id           TEXT PRIMARY KEY,
  user_id      TEXT NOT NULL DEFAULT 'local',  -- owner seam (single user in v1)
  name         TEXT NOT NULL,
  persona      TEXT,            -- who the agent is (rendered into the prompt)
  instructions TEXT,            -- the system-prompt instructions
  model        TEXT NOT NULL DEFAULT 'gpt-5.1',
  tools_json   TEXT NOT NULL DEFAULT '[]',   -- ["calculator","fetch_url"] — IDs, not code
  sources_json TEXT NOT NULL DEFAULT '[]',   -- URLs/files/source references
  skills_json  TEXT NOT NULL DEFAULT '[]',   -- prompt-fragment ids or inline texts
  created_at   INTEGER,
  updated_at   INTEGER
);

-- CONVERSATIONS + edit chats + the session→agent BINDING + resume history.
-- (In prod this splits into Redis hot-state + a conversations table; one node merges them.)
CREATE TABLE sessions (
  id            TEXT PRIMARY KEY,   -- the session_id (minted once per conversation)
  user_id       TEXT NOT NULL DEFAULT 'local',
  agent_id      TEXT NOT NULL,      -- the BINDING: which agent this thread is with
  kind          TEXT NOT NULL DEFAULT 'normal', -- normal | agent_edit
  title         TEXT NOT NULL DEFAULT 'New chat',
  messages_json TEXT NOT NULL DEFAULT '[]',  -- full OpenAI message history (the "resume")
  builder_state_json TEXT NOT NULL DEFAULT '{}', -- Builder-only guided setup state
  created_at    INTEGER,
  updated_at    INTEGER
);

-- APPLIED AGENT EDIT SNAPSHOTS.
CREATE TABLE agent_spec_versions (
  id             TEXT PRIMARY KEY,
  agent_id       TEXT NOT NULL,
  session_id     TEXT NOT NULL,
  version_number INTEGER NOT NULL,
  snapshot_json  TEXT NOT NULL,
  change_summary TEXT NOT NULL DEFAULT '',
  created_at     INTEGER
);
```

> **Audit is NOT a SQLite table in v1.** Earlier drafts had a `run_events` (and `agent_audit`)
> table; those are **dropped**. The append-only audit trail is the **`logs/` tree** instead —
> see [00-CONTEXT.md §4b](00-CONTEXT.md). SQLite holds only the live state above (and
> `sessions.messages_json` is the only thing needed to resume/recover). Agent edit versions
> are product state, not the run audit trail.

## What a row looks like — concrete example

```json
// agent_specs row
{
  "id": "agent-cal-1",
  "name": "Calendar Agent",
  "persona": "Terse, efficient assistant.",
  "instructions": "You manage the user's calendar. Confirm before deleting events.",
  "model": "gpt-5.1",
  "tools_json": ["fetch_url", "web_search"],   // ← NAMES only. No code, no schemas.
  "sources_json": ["https://example.com"]
}
```

The string `"fetch_url"` is just a reference. The **registry** (in code) holds what
`fetch_url` *is* (its schema) and *does* (its function). The engine's resolve step is the
join: name → `{definition, execute}`.

## The Agent Builder is a seeded row

On first boot the binary inserts this row. It is not special code — its only "power" is
that its `tools_json` names tools whose executors write rows.

```json
{
  "id": "builder",
  "name": "Agent Builder",
  "persona": "A friendly assistant that helps design new agents.",
  "instructions": "Interview the user about purpose, persona, and needed tools (use list_tools to see what's available), then call create_agent with the finished spec.",
  "model": "gpt-5.1",
  "tools_json": ["list_tools", "list_agents", "ask_user_question", "create_agent", "update_agent"]
}
```

So the platform **builds agents using its own engine** — the Builder runs through the
exact same orchestrator → engine → loop path as every agent it creates. The act of making
an agent is just an agent with a tool that writes a row.

## The id lifecycle

```
agent_id      ← which agent (chosen by the UI: a click, a route, or hardcoded)
   └─ session_id        ← ONE conversation/edit chat. Minted once. Reused.
        ├─ invocation_id (turn 1)   ← one per run. Audit/trace key.
        ├─ invocation_id (turn 2)
        └─ invocation_id (turn 3)
```

- **agent_id** is decided by the UI before the session exists.
- **session_id** is minted once and freezes the binding `session → agent`.
- **sessions.kind** decides whether it is a normal chat or an agent-edit chat.
- **invocation_id** is fresh on every `run`, keying the append-only audit log.
