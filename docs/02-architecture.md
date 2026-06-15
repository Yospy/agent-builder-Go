# Architecture

## The components

For our build, the four conceptual components collapse into **one Go binary** (the
orchestrator) with the engine, tools, and store as packages/files inside it. The seams
stay in the same places, so this is the production design with the hard parts swapped for
simple ones — not a different design.

```
   browser (UI)
        │  HTTP / SSE
        ▼
 ┌─────────────────────────────────────────────────────┐
 │  ONE Go service  (:8080)  — the only thing with a port│
 │                                                       │
 │  runtime/   the ORCHESTRATOR (service role)           │
 │    • receive request                                  │
 │    • resolve session → agent binding                  │
 │    • load agent row from store                        │
 │    • build deps (tool registry, model client)         │
 │    • call engine.Run(...)                             │
 │    • stream events to UI, persist results             │
 │                                                       │
 │  engine/    the ENGINE (library, no port, stateless)  │
 │    • compose layered prompt from spec                 │
 │    • resolve tool ids → definitions + executors       │
 │    • run reason→act loop                              │
 │                                                       │
 │  tools/     the REGISTRY (library)                    │
 │    • one module per tool: {definition, execute}       │
 │    • platform tools (create_agent…) write rows        │
 │                                                       │
 │  store/     SQLite file + logs/ tree (no port)        │
 │    • SQLite: agent_specs, sessions  (live state)      │
 │    • logs/:  append-only JSONL audit trail            │
 └─────────────────────────────────────────────────────┘
        │ outbound
        ▼
   OpenAI API (the brain, remote)
```

## How many services? One.

Only the orchestrator has an **inbound port**, so it's the only service. The engine and
tools are libraries (imported, no port); the store is a file. The UI connects to the
orchestrator on `:8080` — for both the page (`GET /`) and the chat API.

It becomes more than one service only when a concrete pressure appears later:

| Future pressure | Service you'd split out |
|---|---|
| Cron / webhooks / background runs | a `worker` (queue consumer, same engine) |
| Untrusted URL fetching at scale | a `sandbox` (the SSRF wall) |
| Identity shared across many clients | an `auth` service |

None exist in v1, so neither do those services. Package seams make any future split mechanical.

## The engine internals

```
engine.Run(spec, tools, history, message):

  1. compose prompt   ── stack spec layers in fixed stability order
                         (persona → instructions → skills → tool guidance → output format)
                         static first (cacheable), per-turn content last

  2. resolve tools    ── for each tool name in spec:
                         registry.get(name) → { definition (→LLM), execute (→harness) }

  3. run the loop:
        call brain (OpenAI) with prompt + definitions + history + message
        while brain returns tool_calls:           (OpenAI dialect)
            run the executor for each call        (deterministic, local)
            append a tool result message
            call brain again
        on finish_reason "stop" → emit final answer

  → normalizes OpenAI's tool_calls/deltas into OUR event STREAM
    (llm_text, tool_use, tool_result, done) — the orchestrator never sees OpenAI's shape
```

The engine never authenticates, never touches the DB, never opens a port. Everything
it needs is passed in. It emits events; it does not persist them.

## API surface

Full request/response shapes + SSE event protocol are in [00-CONTEXT.md §12/§7](00-CONTEXT.md).

```
GET  /                          the UI (static files)
GET  /api/agents                list agent rows
GET  /api/agents/:id            one agent (render a chat header)
POST /api/sessions              {agent_id} → mint session id, create binding
GET  /api/sessions/:id          load history (reopen a chat)
POST /api/sessions/:id/run      {message} → SSE stream of events
POST /api/sessions/:id/approve  {call_id, decision} → resume a paused consequential tool
```

> **No agent-creation endpoint.** Agents are created by chatting with the Builder (its
> `create_agent` tool does the INSERT). "New Agent" in the UI just opens a Builder chat.

## The five invariants (keep these or the design drifts)

1. The **engine** never touches HTTP/DB/auth; everything is injected.
2. The **orchestrator** never composes prompts or runs loops; it gathers and persists.
3. **Specs hold names, not code; tools never know about agents** — they meet only at resolution.
4. **session_id minted once per conversation; invocation_id fresh per run.**
5. **Every LLM-influenced outbound request** goes through one hardened chokepoint
   (in-process `safehttp` now; a separate `sandbox` service if the threat model earns it).
