# Build Plan

## Scope of v1

A simple agent builder: **one Go service + local SQLite + a simple UI**, no auth, no
multi-tenancy, no Redis/Postgres, no separate sandbox/worker. The architecture's seams
stay intact (engine is still a stateless library, tools still resolve from a registry,
agents are still rows) — only the operational hardening is deferred.

```
agent-builder/   (single `go build`)
├── engine/        the brain-harness: compose prompt · resolve tools · run loop  [stateless]
├── tools/         registry: calculator, fetch_url(+safehttp), read/write, web_search,
│                            create_agent, update_agent, list_agents, list_tools
├── store/         SQLite: agent_specs, sessions (one .db) + logs/ (append-only audit)
├── runtime/       HTTP server: gather → engine.Run → stream/persist
├── web/           simple UI, served by the same binary
├── docs/          this documentation
└── main.go        seeds the "Agent Builder" row on first boot
```

## Phases

### Phase 1 — Engine + registry + Brain interface
- `engine.Run(ctx, spec, tools, history, message)` — compose prompt, resolve tools, run loop.
- Manual loop (not the SDK tool runner) so we control it and can add the confirmation gate.
- `Brain` interface → `OpenAIBrain` (real) + `FakeBrain` (scripted, for tests). No CLI.
- 2 tools to start: `calculator` (trivial) + `fetch_url` (via `safehttp`).
- **Goal:** prove the loop end-to-end with `go test`, no DB/server.
- **Verify:** `FakeBrain` drives a tool call → result → finish; edge-case + `safehttp` tests pass.

### Phase 2 — SQLite store
- `agent_specs` + `sessions` tables (state); specs and sessions become rows. Audit → `logs/`.
- Seed the `builder` row on first boot.
- `create_agent` tool writes rows. **Goal:** "one engine, N rows" is real.
- **Verify:** create two different agents as rows; run each; behavior differs by row only.

### Phase 3 — HTTP runtime + SSE
- The 5 endpoints (`/api/agents`, `/api/sessions`, `/api/sessions/:id/run`).
- Mint-then-run session contract; SSE streaming of events.
- Add the rest of the tools: read/write files, web_search, and the platform tools
  (`create_agent`, `update_agent`, `list_agents`, `list_tools`).
- **Verify:** drive a full turn over HTTP, watch events stream.

### Phase 4 — Web UI
- Static files served by the binary: **agents list**, **chat view**, **builder chat**.
- The builder chat and the agent chat are the **same component** — the Builder is just
  another agent.
- **Verify:** open the browser, talk to the Builder, watch it `create_agent`, see the new
  agent appear in the list, then chat with it.

### Phase 5 — Prove it end to end
- Talk the Builder into creating your first real agent, then use that agent — all in the UI.

## Decisions locked

- **Language:** Go — one `go build` → single binary with SQLite inside.
- **Module path:** `agent-builder` (imports like `agent-builder/engine`).
- **Brain:** OpenAI API; `OPENAI_API_KEY` from `.env`.
- **Model:** `gpt-5.1`, set via `.env` (`OPENAI_MODEL`) and stored per-row; reasoning
  effort, streaming on, automatic prefix caching.
- **Loop:** our own Go loop (not the SDK tool-runner) — normalizes OpenAI `tool_calls`/
  deltas into our event types (`llm_text`, `tool_use`, `tool_result`, `done`).
- **Tool-call decoding:** narrow each `tool_calls` entry (`"function" in toolCall`) before
  reading name/arguments.
- **Services:** one (the orchestrator). Engine/tools/store are packages/files inside it.
- **First tools:** calculator, fetch_url, read/write files, web_search + platform tools.

## What we deliberately defer (and why it's safe)

| Deferred | Why safe to defer | When to add |
|---|---|---|
| Auth / multi-tenancy | single local user; no `owner_id` needed yet | when others use it → API keys → principal |
| Redis (hot state) | single node, low traffic → a `sessions` table is fine | when scaling across nodes |
| Postgres | rows are rows; SQLite has the same schema | when concurrency/scale demands it |
| Separate `sandbox` | in-process `safehttp` closes the actual SSRF hole | untrusted multi-tenant traffic or code-exec tools |
| `worker`/queue | synchronous-only v1 | cron / webhooks / background runs |

Each deferral is a **package seam that becomes a service seam** later, with minimal surgery.
Monolith-first with clean seams is the proven path; microservices are grown into, not started with.
