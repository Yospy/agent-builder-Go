# Request Lifecycle

The full spine: **gather → execute → stream → persist** — and how it recovers from a crash.

## A single chat turn, end to end

```
UI ── POST /api/sessions/:id/run  {"message": "move my 3pm to 4pm"}
        │
 ╔══════▼═══════════ ORCHESTRATOR (gather + drive) ═══════════════╗
 ║ A. resolve session :id → agent_id   (read the binding)          ║
 ║ B. load agent row from store        → spec                      ║
 ║ C. load session history             → messages                  ║
 ║ D. build deps: tool registry, model client (OpenAI key)         ║
 ║ E. engine.Run(spec, registry, history, message) ───────┐        ║
 ╚════════════════════════════════════════════════════════│════════╝
                                                           │ hand off
 ╔═════════════════ ENGINE (arrange + loop) ═══════════════▼════════╗
 ║ F. compose prompt: persona + instructions + skills + tool defs   ║
 ║ G. resolve tools: names → definitions (→LLM) + executors (→here) ║
 ║ H. loop:                                                         ║
 ║      brain (OpenAI) → tool_calls: fetch_url {…}                  ║
 ║      run executor → result → append → call brain again          ║
 ║      brain → finish_reason stop: "Done — moved to 4pm."          ║
 ║    (normalized into OUR event STREAM as it goes)                 ║
 ╚════════════════════════════════════════════════════════│════════╝
                                       events stream up    │
 ╔═════════════════ ORCHESTRATOR (serve + persist) ════════▼════════╗
 ║ I. relay each event to the UI as it arrives (SSE)                ║
 ║ J. on done: save updated history to sessions (SQLite),          ║
 ║    append events to logs/…jsonl (audit)                          ║
 ╚══════════════════════════════════════════════════════════════════╝
        │
   UI renders the streamed answer
```

## Streaming is *during*, not *after*

The engine does not run to completion and then return one finished blob. It **emits
events as they happen**, and the orchestrator relays each one live:

```
ENGINE emits:              ORCHESTRATOR:        UI sees:
  text token "Let me…"  ──► relay ───────────►  appears immediately
  tool_use: fetch_url   ──► relay ───────────►  "🔧 fetching…"
  tool_result           ──► relay ───────────►  "✓ got it"
  text token "Done…"    ──► relay ───────────►  appears immediately
  done                  ──► save to store ────►  stream closes
```

That live feel — token-by-token text, tool calls shown as they fire — is why we stream
instead of letting the page freeze until the turn finishes. The orchestrator also **taps**
that same stream to persist the conversation and audit log at the end.

## Who does what (the boundary, applied)

| Step | Orchestrator | Engine |
|---|:---:|:---:|
| Resolve session→agent binding | ✅ | |
| Load agent row from store | ✅ | |
| Load session history | ✅ | |
| Build tool registry + model client | ✅ (builds) | ⬅ uses |
| Compose the layered prompt | | ✅ |
| Resolve tool names → defs + executors | | ✅ |
| Run the reason→act loop | | ✅ |
| Relay events to UI | ✅ | (emits) |
| Persist history + audit | ✅ | (emits) |

Everything the orchestrator does **touches the world** (DB, network, UI). Everything the
engine does is **pure compute on what it was handed**.

## Crash recovery

The engine is stateless and runs inside the orchestrator process. If the process crashes:

```
crash mid-turn → in-flight turn is lost → ❌ one error to the user
        │
   state still safe in the store ✅  (history saved per completed turn;
        │                            events appended as they happened)
new request → fresh process reads history back → builds a NEW engine → continues ✅
```

- The crashed turn is **not** resumed at the exact tool call it died on. It fails; the
  user re-issues; the **next** turn sees everything that was saved.
- Recovery is possible **because** the engine holds nothing — there is nothing in it to
  lose. You just rebuild it and feed it the saved history.
- In v1 the "store" is the **SQLite file on disk**: binary crashes → restart → the `.db`
  file is intact → history is there → resume. Same principle, one node.

> Statelessness turns "the server crashed" from a disaster into a hiccup. The boundary
> (world-touching vs. pure-compute) is what lets the engine stay empty in the first place.
