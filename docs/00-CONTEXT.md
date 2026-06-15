# Agent Builder — CONTEXT & API LOCK (source of truth)

> **This file is authoritative.** Where any other doc disagrees, this wins. It is written to
> be self-contained: a UI developer or an agent can build against it without reading the rest.
> Docs `01`–`06` are background/rationale; this is the locked contract.
>
> **Status:** design finalized for **V1**. Safe to build the UI and the service in parallel.

---

## 1. What we're building (one paragraph)

A platform where **every agent is a database row** and **one shared engine** runs all of them.
A single Go binary serves a ChatGPT-style chat UI. You talk to a seeded **Agent Builder**; it
interviews you, picks tools, and writes a new agent row. You then chat with that agent through
the **same** endpoints. The brain is the **OpenAI API**; the loop (harness) is our own Go code.
State lives in **SQLite**; an **append-only `logs/` tree** is the audit trail.

---

## 2. Tech stack (locked)

| Thing | Choice |
|---|---|
| Language | Go (one binary, `go build`) |
| Module path | `agent-builder` (imports like `agent-builder/engine`) |
| Brain | OpenAI API |
| Model | `gpt-5.1`, from `.env` `OPENAI_MODEL`; stored per-row in `model` |
| API key | `.env` `OPENAI_API_KEY` |
| Loop | our own Go loop (NOT the SDK tool-runner) |
| Store (state) | SQLite, one `.db` file |
| Store (audit) | `logs/` directory, append-only JSONL |
| HTTP | stdlib `net/http` + SSE |
| Request validation | `go-playground/validator/v10` (struct tags) |
| SQLite driver | `modernc.org/sqlite` (pure Go, no cgo) |

Dependencies: OpenAI Go SDK, `modernc.org/sqlite`, `go-playground/validator/v10`. Everything
else is stdlib (`net/http`, `encoding/json`, `context`).

---

## 3. Architecture (one service, two roles)

One binary. Only the orchestrator has an inbound port; engine/tools are libraries, store is files.

```
 browser (ChatGPT-style UI)
        │ HTTP + SSE
        ▼
 ┌───────────────────────────────────────────────────────────┐
 │ ONE Go service :8080                                        │
 │  runtime/  ORCHESTRATOR  (service · stateful · world)       │
 │     gather row+history → build deps → engine.Run() →        │
 │     stream events to UI → persist (sessions + logs/)        │
 │  engine/   ENGINE  (library · stateless · pure)             │
 │     ① compose prompt ② resolve tools ③ run loop → emit      │
 │  tools/    REGISTRY  name → {definition→LLM, execute→harness}│
 │  store/    SQLite (state) + logs/ (audit)                   │
 └───────────────────────────────────────────────────────────┘
        │ outbound
        ▼  OpenAI API (gpt-5.1)
```

**The boundary rule:** touches the world (user/network/DB/files/secrets) → **orchestrator**;
pure compute on what it was handed → **engine**. The orchestrator **gathers & persists**; the
engine **arranges & loops**. They meet at one call: `engine.Run(ctx, spec, registry, history, message)`.

---

## 4. Data model

### 4a. SQLite — live app state

```sql
-- THE AGENTS. Each row is one agent. New agent = INSERT. Edit = UPDATE.
CREATE TABLE agent_specs (
  id           TEXT PRIMARY KEY,
  user_id      TEXT NOT NULL DEFAULT 'local',   -- owner seam (single user in v1)
  name         TEXT NOT NULL,
  persona      TEXT,
  instructions TEXT,
  model        TEXT NOT NULL DEFAULT 'gpt-5.1',
  tools_json   TEXT NOT NULL DEFAULT '[]',       -- tool NAMES, e.g. ["web_search","fetch_url"]
  sources_json TEXT NOT NULL DEFAULT '[]',       -- URL/file/source references
  skills_json  TEXT NOT NULL DEFAULT '[]',
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);

-- CONVERSATIONS + the session→agent binding + resume history.
CREATE TABLE sessions (
  id            TEXT PRIMARY KEY,                 -- session_id, minted per conversation
  user_id       TEXT NOT NULL DEFAULT 'local',
  agent_id      TEXT NOT NULL,                    -- the binding
  kind          TEXT NOT NULL DEFAULT 'normal',   -- normal | agent_edit
  title         TEXT NOT NULL DEFAULT 'New chat',
  messages_json TEXT NOT NULL DEFAULT '[]',       -- full OpenAI message history (the resume)
  builder_state_json TEXT NOT NULL DEFAULT '{}',  -- Builder-only guided setup state
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);

-- APPLIED AGENT EDIT SNAPSHOTS.
CREATE TABLE agent_spec_versions (
  id             TEXT PRIMARY KEY,
  agent_id       TEXT NOT NULL,
  session_id     TEXT NOT NULL,
  version_number INTEGER NOT NULL,
  snapshot_json  TEXT NOT NULL,
  change_summary TEXT NOT NULL DEFAULT '',
  created_at     INTEGER NOT NULL
);
```

`sessions.messages_json` is the **only** thing needed to resume/recover. Logs are NOT needed
for resume. (`run_events`/`agent_audit` tables from earlier drafts are **dropped** in v1 — the
audit trail is the `logs/` tree below.)

### 4b. `logs/` — append-only audit trail (files)

```
logs/
├── builder/                          ← Agent Builder activity (the BUILD log)
│   └── <session_id>/
│       └── <invocation_id>.jsonl     ← one rollout (turn) = one file
└── agents/                           ← each built agent's RUN logs
    └── <agent_id>/                   ← isolated per agent
        └── <session_id>/
            └── <invocation_id>.jsonl
```

Each `.jsonl` line is one event (same shape as the SSE protocol, §7):

```jsonl
{"ts":1749600000,"seq":0,"type":"llm_text","text":"Let me search…"}
{"ts":1749600001,"seq":1,"type":"tool_use","name":"web_search","args":{"q":"…"}}
{"ts":1749600002,"seq":2,"type":"tool_result","name":"web_search","ok":true,"data":"…"}
{"ts":1749600003,"seq":3,"type":"done","text":"Here's what I found…"}
```

Audit slices: `builder/**` = all builder activity · `agents/<id>/**` = one agent's rollouts ·
`…/<session>/` = one conversation · `…/<invocation>.jsonl` = one rollout.

**Seam:** multi-user later just prefixes a top level → `logs/<user_id>/builder/…`.

---

## 5. ID lifecycle

```
agent_id        ← born WITH the agent row (permanent)        e.g. "agent-7", "builder"
  └ session_id      ← MINTED per conversation (POST /api/sessions)
      └ invocation_id   ← MINTED per turn (POST /api/sessions/:id/run)
          └ seq             ← event index within the turn (0,1,2,…)
```

Two are minted (`session_id`, `invocation_id`); one is assigned at creation (`agent_id`).

---

## 6. Tool catalog (V1)

Two families. A tool is `consequential` if it changes the world (create/update/write) — those
**pause the loop for confirmation** (§8).

### Platform tools (build/manage agents — normally only on the `builder` row)

| Name | Does | Consequential |
|---|---|---|
| `list_tools` | return the capability-tool catalog | no |
| `list_agents` | list existing agent rows | no |
| `ask_user_question` | ask one structured Builder setup question | no |
| `create_agent` | INSERT a new agent row | **yes** |
| `update_agent` | UPDATE an existing agent row | **yes** |

### Capability tools (what built agents can do)

| Name | Does | Consequential | Notes |
|---|---|---|---|
| `calculator` | evaluate arithmetic | no | pure, no I/O |
| `fetch_url` | GET a URL, return text | no | routes through `safehttp` (SSRF wall) |
| `web_search` | search the web | no | |
| `read_file` | read a local file | no | confined to a fixed working dir |
| `write_file` | write a local file | **yes** | confined to a fixed working dir |

`create_agent` may only grant tool names that exist in the registry (validated at call time).

---

## 7. SSE event protocol (the wire format the UI renders)

`POST /api/sessions/:id/run` responds with `Content-Type: text/event-stream`. Each event is a
single SSE frame: `data: <json>\n\n`. Event JSON shapes:

| `type` | Shape | UI renders |
|---|---|---|
| `llm_text` | `{"type":"llm_text","seq":N,"text":"…"}` | append text delta |
| `status` | `{"type":"status","seq":N,"message":"…"}` | show transient activity |
| `tool_use` | `{"type":"tool_use","seq":N,"call_id":"c1","name":"web_search","args":{…}}` | "🔧 web_search…" |
| `tool_result` | `{"type":"tool_result","seq":N,"call_id":"c1","name":"web_search","ok":true,"data":"…"}` | "✓ done" / "✗ err" |
| `confirm` | `{"type":"confirm","seq":N,"call_id":"c1","name":"create_agent","args":{…}}` | show **[Approve] [Cancel]** |
| `user_question` | `{"type":"user_question","seq":N,"call_id":"q1","field":"fetch_mode","question":"…","options":[…],"allow_custom":true}` | show guided question card |
| `done` | `{"type":"done","seq":N,"text":"<full final text>"}` | **authoritative** final text; finalize, close stream |
| `error` | `{"type":"error","seq":N,"message":"…"}` | show error, close stream |
| `aborted`| `{"type":"aborted","seq":N}` | mark turn canceled, close stream |

These same shapes are what land in `logs/…jsonl` (plus a `ts`). The UI never sees OpenAI's raw
format — the engine normalizes it.

> **`llm_text` vs `done`:** `llm_text` events are incremental text deltas.
> `done.text` is the **authoritative** full final text — render the final
> assistant message from it, don't also append the final `llm_text`, or you'll
> double-render.
>
> **`status`:** operational activity, not private chain-of-thought. Normal runs
> may emit `Preparing response`, `Checking available tools`, `Thinking`,
> `Calling <tool>`, `Reading tool result`, and `Drafting final response`.
> Builder runs keep Builder-specific wording such as `Preparing agent brief`,
> `Drafting agent spec`, `Waiting for approval`, `Creating agent`, and
> `Saving agent`.

---

## 8. Confirmation gate (consequential tools, over SSE)

```
loop hits a consequential tool_use
     │
emit  {"type":"confirm","call_id":"c1","name":"create_agent","args":{…}}  ──► UI shows [Approve][Cancel]
     │ (run goroutine BLOCKS on a channel keyed by call_id; SSE stays open)
     ▼
UI ── POST /api/sessions/:id/approve {"call_id":"c1","decision":"approve"|"deny"}  → 202 ack
     │
approve → executor runs → tool_result streams on the ORIGINAL run connection → loop continues
deny    → "denied" fed back as tool_result → loop continues without it
```

- The continuation flows on the **original `run` SSE connection**; `/approve` is a separate
  request that just unblocks it and returns `202`.
- If the SSE connection drops while paused → treat as **abort** (§9).

---

## 9. Cancellation (AbortController → ctx)

```
UI: fetch(run, {signal})  ── user clicks Stop → controller.abort()
        │
Go: r.Context() canceled → same ctx threaded into engine.Run() and the OpenAI call
        │
loop checks ctx between brain calls / tool execs → stops, emits {"type":"aborted"}
```

On abort (and on plain disconnect — same path):
- `sessions` (resume state): **not committed** — partial turn dropped, like a crash. History stays clean.
- `logs/…jsonl` (audit): **appended**, ending with an `aborted` event. Truthfully recorded.
- The per-session in-flight lock is released so the user can retry.

---

## 10. Validation

| Boundary | How |
|---|---|
| ① client → orchestrator (HTTP bodies) | typed structs + `validator/v10` tags (e.g. `validate:"required,max=10000"`) → `400` on fail |
| ② model → harness (tool args) | OpenAI **strict function-calling** + defensive `json.Unmarshal` into the tool's typed args struct; failure → error `tool_result` (no crash). Narrow `tool_calls` with `"function" in toolCall` first |
| ③ orchestrator → client (SSE) | our own typed `Event` structs → `json.Marshal`; we own the shape |

---

## 11. Edge-case ledger (V1 defaults — all locked)

```
runaway tool loop     → hard cap 10 iterations → emit error, stop
tool exec error       → return as tool_result (ok:false); loop never crashes; model recovers
malformed tool_calls  → narrow + unmarshal guard → error tool_result
OpenAI error/timeout  → emit error event, persist, history stays clean
confirm gate          → pause + confirm event → POST /approve → resume/deny (§8)
user cancels tool     → feed "denied" back as tool_result → continue
cancel in-flight      → AbortController → ctx cancel → abort; drop state, log aborted (§9)
double-submit         → one in-flight run per session → reject 2nd with 409
path traversal        → read_file/write_file confined to fixed working dir; reject ../ + absolute
empty/oversized msg   → validator rejects → 400
client disconnect     → same as cancel (ctx) → abort path
```

---

## 12. API contract (build the UI against this)

Base: `http://localhost:8080`. All bodies JSON. Errors: `{"error":"message"}` with the status.

### `GET /`
Serves the UI (static files).

### `GET /api/agents`
List all agent rows.
```jsonc
// 200
[ { "id":"builder","name":"Agent Builder","persona":"…","model":"gpt-5.1",
    "tools":["list_tools","create_agent","update_agent","list_agents"], "sources":[],
    "created_at":1749600000,"updated_at":1749600000 },
  { "id":"agent-7","name":"Research Agent","persona":"…","model":"gpt-5.1",
    "tools":["web_search","fetch_url","write_file"], "created_at":…, "updated_at":… } ]
```

### `GET /api/agents/:id`
One agent (to render a chat header).
```jsonc
// 200
{ "id":"agent-7","name":"Research Agent","persona":"…","instructions":"…",
  "model":"gpt-5.1","tools":["web_search","fetch_url","write_file"], "sources":[],
  "created_at":…, "updated_at":… }
// 404  { "error":"agent not found" }
```

### `POST /api/agents/:id/edit-sessions`
Create a new edit chat for an agent.
```jsonc
// 201
{ "id":"sess-edit","agent_id":"agent-7","kind":"agent_edit","title":"New chat",
  "messages":[], "created_at":…, "updated_at":… }
```

### `GET /api/agent-edit-sessions`
List all edit chats for the sidebar's `Agents > Agent name` groups.

### `GET /api/agents/:id/versions`
List applied agent snapshots, newest first.

### `POST /api/sessions`
Start a conversation with an agent (mint `session_id`, freeze the binding).
```jsonc
// req
{ "agent_id":"agent-7" }
// 201
{ "session_id":"sess-abc","agent_id":"agent-7" }
// 404  { "error":"agent not found" }   // unknown agent_id
```

### `GET /api/sessions/:id`
Load a conversation's history (to reopen a chat).
```jsonc
// 200
{ "id":"sess-abc","agent_id":"agent-7","kind":"normal","title":"New chat",
  "messages":[ {"role":"user","content":"…"}, {"role":"assistant","content":"…"} ] }
// 404  { "error":"session not found" }
```

### `PATCH /api/sessions/:id`
Update session metadata used by navigation.
```jsonc
// req
{ "title":"Added source URL" }
```

### `POST /api/sessions/:id/run`  → **SSE stream**
Send a message; stream the turn. Mints an `invocation_id`.
```jsonc
// req
{ "message":"build a research agent that saves notes" }
// 200  Content-Type: text/event-stream  → frames per §7, ending in done|error|aborted
// 400  { "error":"message is required" }      // validation
// 404  { "error":"session not found" }
// 409  { "error":"a run is already in flight for this session" }   // double-submit
```
Client must send `fetch(..., { signal })` so Stop can abort (§9).

### `POST /api/sessions/:id/approve`
Approve/deny a paused consequential tool (§8).
```jsonc
// req
{ "call_id":"c1", "decision":"approve" }    // or "deny"
// 202  { "ok":true }     // continuation streams on the original /run connection
// 404  { "error":"no pending confirmation for this call_id" }
```

> **No agent-creation endpoint.** Agents are created by **chatting with the Builder** (its
> `create_agent` tool does the INSERT). The UI's "New Agent" simply opens a Builder chat
> (`POST /api/sessions {agent_id:"builder"}`).

### `POST /api/sessions/:id/answer`
Answer a paused `ask_user_question` tool. Continuation streams on the original `/run` SSE connection.
```jsonc
// req
{ "call_id":"q1", "option_id":"both" }
// or
{ "call_id":"q1", "custom_text":"Use only official a16z essays" }
// 202  { "ok":true }
// 400  { "error":"invalid option_id" }
// 404  { "error":"no pending question for this call_id" }
```

---

## 13. The two user journeys (UI flows)

**Build an agent:**
```
"+ New Agent" → POST /api/sessions {agent_id:"builder"} → POST …/run {message}
  → stream: user_question → POST …/answer, tool_use list_tools, llm_text proposal,
            confirm create_agent → [Approve] → POST …/approve → tool_result → done
  → GET /api/agents (refresh list) → new agent appears
```

**Talk to an agent (same endpoints, different agent_id):**
```
click agent → POST /api/sessions {agent_id:"agent-7"} → POST …/run {message} → stream → done
reopen chat → GET /api/sessions/:id (history) → POST …/run to continue
```

**Edit an agent:**
```
Edit Agent → POST /api/agents/:id/edit-sessions → /agents/:id/edit/:session
  → POST /api/sessions/:id/run
  → confirm update_agent → [Apply changes] → version snapshot + agent row UPDATE
```

The message UI is shared. Edit chats add a right-side context panel and run through a
synthetic Agent Editor spec so normal agents never receive `update_agent`.

---

## 14. Security (two walls)

- **Wall A — SSRF:** every LLM-influenced outbound fetch goes through `safehttp`, which blocks
  internal IPs (`169.254/16`, `10/8`, `172.16/12`, `192.168/16`, loopback, IPv6 private) at
  **connect time, on the final IP, on every redirect hop** (Go `net.Dialer.Control`).
- **Wall B — prompt injection:** least privilege (agent only has its row's tools) + the
  `consequential` confirmation gate + credentials injected only at the last hop (never in the
  LLM's context or logs). See `05-security.md`.

---

## 15. Invariants (hold these or the design drifts)

1. The **engine** never touches HTTP/DB/files/secrets — everything is injected.
2. The **orchestrator** never composes prompts or runs loops — it **gathers & persists**.
   (orchestrator gathers · engine arranges)
3. Rows hold tool **names, not code**; tools never know agents — they meet only at resolution.
4. `agent_id` born with the row · `session_id` minted per conversation · `invocation_id` per run.
5. Every LLM-influenced outbound request goes through one hardened chokepoint (`safehttp`).

---

## 16. Out of scope for V1 (seams kept)

| Deferred | Seam that lights it up later |
|---|---|
| Auth / multi-tenancy | `user_id` column exists (`'local'`); auth → principal fills it |
| Multi-node scale (Redis/Postgres) | rows are rows; same schema migrates |
| Separate `sandbox` process | tools already route through one `safehttp` chokepoint |
| `worker`/queue (cron/webhooks) | engine is reusable; add a queue consumer |
| Agent deletion, log rotation, retries | additive, no redesign |

---

## 17. Build phases

1. **Engine + registry + Brain interface** — `engine.Run(ctx, …)`, manual loop, `OpenAIBrain`/`FakeBrain`, `calculator` + `fetch_url`. Prove the loop via `go test` (no CLI, no DB/server).
2. **SQLite store** — `agent_specs` + `sessions`; seed the `builder` row; `create_agent` tool. "One engine, N rows" — proven by tests.
3. **HTTP runtime + SSE** — the §12 endpoints, the confirm/approve + abort mechanics, `logs/` writing; add remaining tools (web_search, read/write_file, platform tools).
4. **Web UI** — single chat component; agents list; Builder chat == agent chat.
5. **End-to-end** — talk the Builder into creating a real agent, then use it, all in the UI.

Each phase ends with a verification step before it's considered done.
