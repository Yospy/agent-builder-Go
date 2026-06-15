# Status — v1 Shipped Capabilities

> **Purpose:** the single source of truth for *what actually exists and works today*, as
> opposed to [07-implementation-plan.md](07-implementation-plan.md) (the build sequence) or
> [00-CONTEXT.md](00-CONTEXT.md) (the locked design contract). When this doc and the plan
> disagree about what's done, **this doc wins** — it reflects the running code.
>
> **As of:** 2026-06-12 · branch `codex/portfolio-trust-pass`
> **Health:** `go build ./...` ✓ · `go vet ./...` ✓ · `go test ./...` ✓ (42 tests, race-clean) · live E2E on real `gpt-5.1` ✓

---

## One-line state

All six phases (P0–P5) are complete and verified end-to-end: you can talk to the **Builder**
agent in the web UI, have it author a brand-new agent into the database, then open and use that
agent — streaming, with the confirm gate and an isolated audit-log trail — plus optional
Braintrust tracing of every model call.

---

## What works today

### Core loop (engine)
- Stateless `engine.Run(ctx, Input)` — one shared engine "becomes" any agent from its row.
- Brain-behind-interface: `OpenAIBrain` (real, `gpt-5.1`) and `FakeBrain` (scripted, offline tests).
- Multi-step tool-calling loop with a **hard cap of 10 iterations** (runaway → `error`, stop).
- A/B tool resolution: tool name → definition (to the model) + executor (to the harness); secrets never reach the model.
- Layered **prompt composed at runtime** from the row's `persona` + `instructions` + a tool-usage hint (`composePrompt`). No pre-baked `system_prompt` blob — steering lives as data.
- Panic-safe tool execution; malformed tool args and tool errors come back as `tool_result {ok:false}` — the loop never crashes.
- Cancellation honored between brain calls and tool execs (`ctx` threaded all the way to the OpenAI call).

### Tools (registered at boot)
| Tool | Consequential | Notes |
|---|---|---|
| `calculator` | no | recursive-descent float evaluator (real division: `10/4 → 2.5`) |
| `fetch_url` | no | GET + text, through the `safehttp` chokepoint |
| `read_file` | no | confined to `WORK_DIR` (lexical traversal blocked) |
| `write_file` | **yes** | confined to `WORK_DIR`; triggers the confirm gate |
| **platform tools** | — | `list_tools`, `list_agents`, `create_agent`, `update_agent` (Builder-only; hidden from normal agents) |

### Store (agents-as-data)
- SQLite (`modernc.org/sqlite`, pure-Go, no cgo), `MaxOpenConns(1)` serializes access.
- Tables: `agent_specs` (id, user_id, name, persona, instructions, model, tools_json, skills_json, timestamps) + `sessions` (FK → agent).
- `builder` row seeded idempotently on first boot.
- Create agent = `INSERT`, edit = `UPDATE`; `create_agent` validates every granted tool name exists and rejects platform tools.
- Per-turn history persisted transactionally (read-modify-write).

### HTTP runtime + SSE
- Go 1.22 `ServeMux` (method + wildcard). Routes:
  - `GET /api/agents` · `GET /api/agents/{id}`
  - `POST /api/sessions` · `GET /api/sessions/{id}`
  - `POST /api/sessions/{id}/run` (SSE) · `POST /api/sessions/{id}/approve`
  - `GET /` (static fallback, unused in the Next.js dev setup)
- SSE event protocol: `llm_text`, `status`, `tool_use`, `tool_result`, `confirm`, `done` (authoritative final text), `error`, `aborted`.
- **One run per session** lock → `409` on double-submit.
- **Confirm gate** for consequential tools: two-phase (register wait-channel *before* emitting `confirm`, so a fast `/approve` can't 404), resumes on the original `/run` stream.
- **§9 cancellation discipline:** abort / error / disconnect → server does **not** persist; the UI rolls the turn back to stay consistent with the DB.
- All checks (404/409/400) happen *before* SSE headers go out; persist uses `context.WithoutCancel` so a disconnect can't drop a completed turn.
- Request validation: typed structs + `validator/v10`; 1 MiB body cap; `ReadHeaderTimeout`/`IdleTimeout` (no `WriteTimeout` — SSE is long-lived).
- Append-only JSONL audit log per invocation, isolated: `logs/builder/...` vs `logs/agents/<id>/<session>/<inv>.jsonl`.
- Graceful shutdown on SIGINT/SIGTERM (drains in-flight turns and flushes tracing).

### Web UI (Next.js)
- Next.js 16 / React 19 / Tailwind v4 / shadcn, monochrome.
- Single reusable chat component (Builder and every agent differ only by `agent_id`).
- Agents list + "New Agent" → Builder chat; streams Builder activity `status`, `llm_text` deltas, tool activity, and `confirm` → Approve/Cancel.
- Stop button (`AbortController`); reopen a chat rehydrates history from `GET /api/sessions/{id}`.
- Error/abort/disconnect → rollback + toast, matching the server's no-persist rule.
- **Deployment:** runs as its own dev server on `:3000`, proxying `/api/*` → `:8080` (`next.config.ts` rewrites). Same-origin, no CORS.

### Observability — Braintrust tracing (optional)
- Off by default; enabled only when `BRAINTRUST_API_KEY` is set.
- OpenTelemetry → Braintrust via the contrib middleware on the OpenAI client.
- Each turn opens a **root span** (`agent.run`) so all LLM calls within a rollout nest into one trace.
- Lives entirely on the orchestrator side (`tracing/` package) — the engine stays pure.

---

## Not in v1 (deferred, seams preserved)

- **`web_search` tool** — referenced in early docs but **not wired** (needs a provider/key decision). Only the five tools above are registered.
- **Auth / multi-tenancy** — single `user_id = 'local'`; the column exists as a seam.
- **Postgres / Redis / separate sandbox or worker services** — single Go binary + SQLite.
- **Skills** — `skills_json` column exists but is unused.
- **Verbatim `system_prompt` column** — by design we store ingredients (`persona` + `instructions`) and compose at runtime; an opt-in verbatim column is a known future lever.

## Current streaming behavior

- `/api/sessions/:id/run` streams SSE through the Next route and Go runtime.
- OpenAI assistant text arrives as incremental `llm_text` frames, followed by
  authoritative `done.text`.
- Normal agents emit transient operational statuses such as `Preparing response`,
  `Thinking`, `Calling <tool>`, `Reading tool result`, and
  `Drafting final response`; Builder keeps Builder-specific status wording.

---

## How to run

```
# Terminal 1 — backend (server/)
go run .                      # :8080  (needs OPENAI_API_KEY in .env)

# Terminal 2 — UI (web/)
npm run dev                   # :3000  → open this in the browser
```

Optional: set `BRAINTRUST_API_KEY` (and `BRAINTRUST_PROJECT`, default `agent-builder`) in `.env`
to enable tracing. Env knobs: `OPENAI_MODEL` (`gpt-5.1`), `DB_PATH`, `LOGS_DIR`, `WORK_DIR`, `ADDR`.

```
build:  go build ./...
test:   go test ./...
live:   LIVE_OPENAI=1 go test ./brain/...   # hits real gpt-5.1
```
