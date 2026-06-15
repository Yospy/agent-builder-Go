# Agent Builder — Documentation

A platform for building and running AI agents, where **every agent is a database row**
and **one shared engine** runs all of them. Learned from studying a production
multi-tenant agent platform (codename "Wajo"), generalized and scoped for our build.

## The one-sentence model

> A request reaches the **orchestrator**, which gathers the agent's row + history + tools
> and hands them to a stateless **engine** (the loop/harness); the engine composes a
> layered prompt, calls the **brain** (OpenAI, remote), runs tools, and streams events
> back through the orchestrator to the UI — while all memory lives in the **store**.

## Read in this order

0. **[00-CONTEXT.md](00-CONTEXT.md) — the locked source of truth (APIs, schema, edge cases). Where any doc disagrees, this wins.** Build against this.
1. [01-concepts.md](01-concepts.md) — the core mental models (read first; everything hangs off these)
2. [02-architecture.md](02-architecture.md) — the components, the boundary, the diagrams
3. [03-data-model.md](03-data-model.md) — agents-as-data: the rows and schema
4. [04-request-lifecycle.md](04-request-lifecycle.md) — gather → execute → stream → persist, and crash recovery
5. [05-security.md](05-security.md) — SSRF/sandbox, last-hop credentials, prompt injection
6. [06-build-plan.md](06-build-plan.md) — the scoped simple build (one Go service + SQLite + UI)
7. **[07-implementation-plan.md](07-implementation-plan.md) — the hands-on build playbook: phases, checklists, verify gates (no CLI; test-driven; `Brain` interface)**
8. **[08-status.md](08-status.md) — what's actually shipped today (v1 capabilities, what's deferred, how to run). Start here for "is X done?"**

## The non-negotiable invariants

1. The **engine** never touches HTTP/DB/auth — everything is injected.
2. The **orchestrator** never composes prompts or runs loops — it gathers and persists.
3. **Specs hold names, not code; tools never know about agents** — they meet only at resolution.
4. **Session id minted once per conversation; invocation id fresh per run.**
5. **Every LLM-influenced outbound request** goes through one hardened chokepoint.

## Current decisions

- **Language:** Go (one binary, no SDK tool-runner — our own loop)
- **Brain:** OpenAI API; key from `.env` (`OPENAI_API_KEY`)
- **Model:** `gpt-5.1`, set via `.env` (`OPENAI_MODEL`) and stored per-row in `model`
- **Store:** SQLite for state (`agent_specs`, `sessions`) + an append-only `logs/` tree for audit
- **Services:** one Go binary (the orchestrator); engine + tools + store are packages inside it
- **UI:** single ChatGPT-style chat (Next.js on `:3000`, proxying `/api/*` → `:8080`); the Agent Builder is just an agent you talk to
- **Tools shipped:** calculator, fetch_url, read/write files, + platform tools (create_agent, etc.). `web_search` is **deferred** (not wired in v1).
- **Tracing:** optional Braintrust (OpenTelemetry); off unless `BRAINTRUST_API_KEY` is set
- **Out of scope for v1:** auth, multi-tenancy, Redis, Postgres, separate sandbox/worker services
  (seams kept — `user_id` column exists, tools route through one `safehttp` chokepoint)

> **Build status:** all phases complete and verified — see **[08-status.md](08-status.md)** for the
> exact shipped capability list and what's deliberately deferred.

> Full locked contract (every endpoint, the SSE event shapes, the edge-case ledger) lives in
> **[00-CONTEXT.md](00-CONTEXT.md)**.
