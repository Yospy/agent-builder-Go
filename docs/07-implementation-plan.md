# Implementation Plan — Build Playbook

> **Purpose:** the hands-on build sequence I (the agent) refer to while implementing. The
> *what/why* of the system is in [00-CONTEXT.md](00-CONTEXT.md); this is the *how/when*.
> Update the checkboxes as phases complete.
>
> **Legend:** `[ ]` todo · `[~]` in progress · `[x]` done · 🔒 hard verify gate (can't pass without it).

---

## Principles (don't violate these while building)

- **Loop-first.** Build the riskiest thing — our own Go agent loop over OpenAI `tool_calls` —
  first, naked, proven by tests. Wrap layers around it after.
- **Gate-verified.** A phase is NOT done until its 🔒 verify step passes. No drifting forward on faith.
- **Contract-frozen.** The API + SSE shapes ([00-CONTEXT §12/§7](00-CONTEXT.md)) are locked, so the
  UI track builds in parallel.
- **No CLI.** The product is the HTTP service + UI. Phases 1–2 are libraries proven by `go test`
  (not a CLI). First runnable binary = the HTTP server in Phase 3.
- **Brain behind an interface.** `Brain` is an interface → `OpenAIBrain` (real) + `FakeBrain`
  (scripted, for deterministic tests). This both enables CLI-free testing and proves "the brain
  is a swappable dialect."
- **Simplicity first.** Minimal deps, minimal surface, touch only what's needed (per repo rules).

## Parallel tracks

```
SERVICE (me):   P0 ─► P1 ─► P2 ─► P3 ───────────────┐
                                                     ├─► P4 (UI served) ─► P5
UI (you):       build against 00-CONTEXT §12/§7 ─────┘
```

## Directory target (end state)

```
agent-builder/
├── brain/         Brain interface · OpenAIBrain · FakeBrain
├── engine/        engine.Run(ctx, …) + the loop          (+ engine_test.go)
├── tools/         registry + tool impls + safehttp        (+ tools_test.go)
├── store/         SQLite: agent_specs, sessions           (+ store_test.go)
├── runtime/       HTTP server, SSE, confirm/approve, abort, logs/ writer
├── web/           single chat UI component (served by the binary)
├── logs/          append-only JSONL audit (created at runtime)
├── docs/
├── .env           OPENAI_API_KEY, OPENAI_MODEL
└── main.go        boots the server (Phase 3) — no CLI subcommands
```

---

## Phase 0 — Project setup  `[x]`

- [ ] `go mod init agent-builder`
- [ ] add deps: OpenAI Go SDK, `modernc.org/sqlite`, `go-playground/validator/v10`
- [ ] `.env` with `OPENAI_API_KEY`, `OPENAI_MODEL=gpt-5.1`; load via a tiny env helper
- [ ] `.gitignore`: `.env`, `*.db`, `logs/`
- [ ] package skeleton dirs (empty `brain/ engine/ tools/ store/ runtime/ web/`)

🔒 **Verify:** `go build ./...` succeeds on the empty skeleton.

---

## Phase 1 — Engine + Brain + Registry (the loop, naked)  `[x]` ✅ verified (17 tests + live gpt-5.1 smoke)

**Build**
- [ ] `brain/brain.go` — `Brain` interface; `Step` type (text | tool_calls | done)
- [ ] `brain/openai.go` — `OpenAIBrain`: one streaming call; **strict function-calling**;
      normalize deltas/`tool_calls` → our `Step`/events; narrow `tool_calls` with
      `"function" in toolCall`
- [ ] `brain/fake.go` — `FakeBrain`: scripted steps for deterministic tests
- [ ] `tools/registry.go` — `name → {Definition (→LLM), Execute (→harness), Consequential}`
- [ ] `tools/calculator.go` — pure arithmetic
- [ ] `tools/fetch_url.go` — GET + text, **through `safehttp`**
- [ ] `tools/safehttp.go` — `net.Dialer.Control` blocks private/link-local/loopback on every hop
- [ ] `engine/engine.go` — `engine.Run(ctx, spec, registry, history, message)`:
      ① compose layered prompt ② resolve tool names → A/B ③ loop, emit events

**Edge cases wired here** (from [00-CONTEXT §11](00-CONTEXT.md))
- [ ] runaway loop → hard cap 10 iterations → `error`, stop
- [ ] tool exec error → returned as `tool_result {ok:false}` (loop never crashes)
- [ ] malformed tool args → unmarshal guard → error `tool_result`
- [ ] `ctx` checked between brain calls / tool execs (cancellation seam, even pre-HTTP)

🔒 **Verify (`go test ./engine/...`)**
- [ ] `FakeBrain` scripts a `fetch_url` call → engine runs it → feeds result back → finishes
- [ ] loop-cap, tool-error, malformed-args tests pass
- [ ] `safehttp` rejects `169.254.169.254` / `127.0.0.1` / `10.x` (table test)
- [ ] (optional, `LIVE_OPENAI=1`) one real `gpt-5.1` smoke run completes

---

## Phase 2 — SQLite store (agents become rows)  `[x]` ✅ verified (13 store/platform tests, race-clean)

**Build**
- [ ] `store/schema.sql` — `agent_specs`, `sessions` (per [00-CONTEXT §4a](00-CONTEXT.md), incl. `user_id` default `'local'`)
- [ ] `store/store.go` — open/migrate; `GetAgent`, `ListAgents`, `InsertAgent`, `UpdateAgent`,
      `CreateSession`, `GetSession`, `AppendMessages`
- [ ] seed the `builder` row on first boot (idempotent)
- [ ] platform tools `tools/agents.go` — `list_tools`, `list_agents`, `create_agent` (validates
      tool names exist in registry), `update_agent`

🔒 **Verify (`go test ./store/...` + an engine integration test)**
- [ ] create 2 agent **rows** with different `tools_json`
- [ ] run each through the engine — behavior differs **by row only** (same engine code)
- [ ] `create_agent` rejects an unknown tool name

---

## Phase 3 — HTTP runtime + SSE (first runnable binary)  `[x]` ✅ verified (42 tests race-clean + live OpenAI E2E incl. confirm/approve)

**Build** — endpoints per [00-CONTEXT §12](00-CONTEXT.md)
- [ ] `runtime/server.go` — router + static UI mount (`GET /`)
- [ ] `GET /api/agents`, `GET /api/agents/:id`
- [ ] `POST /api/sessions`, `GET /api/sessions/:id`
- [ ] `POST /api/sessions/:id/run` — SSE; mints `invocation_id`; one-in-flight lock (`409`)
- [ ] `POST /api/sessions/:id/approve` — unblocks the paused tool (channel by `call_id`)
- [ ] SSE event encoder for the §7 shapes (`llm_text`/`tool_use`/`tool_result`/`confirm`/`done`/`error`/`aborted`)
- [ ] confirmation gate: consequential tool → emit `confirm`, block on channel, resume/deny
- [ ] cancellation: thread `r.Context()` → `engine.Run` → OpenAI call; abort → drop state, log `aborted`
- [ ] `runtime/logs.go` — append-only JSONL writer to `logs/builder|agents/<id>/<session>/<invocation>.jsonl`
- [ ] request validation: typed structs + `validator/v10` → `400`
- [ ] remaining tools: `web_search`, `read_file`, `write_file` (confined to a working dir)
- [ ] `main.go` — boot server on `:8080`

🔒 **Verify (run the binary + `curl`)**
- [ ] full turn over HTTP streams events and ends in `done`
- [ ] consequential tool emits `confirm`; `/approve` resumes; result streams on the original connection
- [ ] cancel mid-run → `aborted`, session not committed, `logs/` has the `aborted` line, lock released
- [ ] double-submit on one session → `409`
- [ ] `logs/` tree is written with correct nesting

---

## Phase 4 — Web UI  `[x]` ✅ built (Next.js in web/) — build+lint green, live E2E through the :3000 proxy

> **Deployment reality (deviation from the original "binary serves static web/"):** the UI is a
> Next.js 16 app in `web/` run as its own dev server on `:3000`, proxying `/api/*` → the Go
> service on `:8080` (`web/next.config.ts` rewrites). Same-origin to the browser, no CORS. The
> Go binary's static `GET /` handler is unused in this setup (kept as a harmless fallback).
> Dev run: terminal 1 `go run .` (server/, :8080) · terminal 2 `npm run dev` (web/, :3000).


**Build** (served from `web/` by the binary)
- [ ] single **chat component** (reused for Builder and every agent; differs only by `agent_id`)
- [ ] agents list (`GET /api/agents`) with "+ New Agent" → opens a Builder chat
- [ ] SSE consumption: render `llm_text` deltas, tool activity, `confirm` → **[Approve]/[Cancel]**
- [ ] **Stop** button → `AbortController.abort()`
- [ ] reopen a chat → `GET /api/sessions/:id` history

🔒 **Verify (browser):** talk to the Builder → it calls `create_agent` → new agent appears in
the list → open it → chat with it.

---

## Phase 5 — End-to-end  `[x]` ✅ verified through the :3000 proxy: Builder created "Mathy", Mathy used (calculator→437), history persisted, audit logs isolated (builder/ vs agents/<id>/)

🔒 **Verify:** talk the Builder into creating a *real* agent, then use that agent — entirely in
the UI, streaming, with the confirm gate and a clean `logs/` trail.

---

## Quick reference

```
build:   go build ./...
test:    go test ./...
live:    LIVE_OPENAI=1 go test ./brain/...   # hits real gpt-5.1
run:     go run .            # Phase 3+   → http://localhost:8080
```

**Invariants to re-check at every phase** ([00-CONTEXT §15](00-CONTEXT.md)): engine never touches
HTTP/DB/files/secrets · orchestrator gathers & persists (never composes/loops) · rows hold names
not code · id lifecycle · every outbound fetch through `safehttp`.

**Definition of done (per phase):** code + tests written → 🔒 verify gate passes → diff reviewed
(minimal change? no architectural drift? staff-engineer approve?) → checkboxes updated here.
