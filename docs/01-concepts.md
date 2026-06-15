# Core Concepts (the mental models)

These are the load-bearing ideas. Everything else is a consequence of them.

---

## 1. Agents are data, not code

An agent is **one row in a database**, not a program. The row holds the agent's
identity (persona, instructions), brain choice (model), and a **manifest of
capability ids** (which tools, which skills). There is **one shared engine** that
loads a row and "becomes" that agent for one run.

- New agent = `INSERT` a row. Edit an agent = `UPDATE` a row. No deploy, no code change.
- 100 agents = 100 rows, one engine. The runtime has no per-agent code path.
- **The Agent Builder is itself just a seeded row** whose tools happen to write rows.

> One engine, N rows. The agent *is* its spec; the runtime is universal.

---

## 2. Service vs. library — the two halves

- **Orchestrator = a service.** It has an inbound port, talks to users, the DB, the
  network. It does the worldly work: authenticate, gather, wire dependencies, stream,
  persist. (Generalized name for Wajo's "agentnet".)
- **Engine = a library.** No port, no state. Pure logic you *import* and call. It
  composes the prompt, resolves tools, and runs the loop. (Generalized name for Wajo's
  "/agent".)

**Why the engine is a library, not a service:**
A thing becomes a service only to (a) hold state others need, (b) be shared by many
callers, or (c) scale/deploy independently. The engine does **none** of these:
- it holds no state (all state is external),
- only the orchestrator calls it,
- it runs exactly **once per request, 1:1** with the orchestrator — no separate load curve.

**Outbound calls don't make you a service.** The engine makes an outbound HTTP call to
OpenAI, but it has no *inbound* port — nobody dials the engine. Direction matters:
inbound port = service; outbound client = just a library using a client.

---

## 3. The boundary: "touches the world" vs. "pure compute"

The dividing line between orchestrator and engine is one simple rule:

> If it touches the outside world (users, network, database) → **orchestrator**.
> If it's pure thinking with what it was handed → **engine**.

```
ORCHESTRATOR (talks to the world)        ENGINE (just computes)
- receive the user request               - compose the prompt
- fetch the agent row from DB            - resolve tool names → code
- load the session history               - run the loop (call brain, run tools)
- build the tools + model client         - return the answer/events
- SAVE the result back to DB

       ── doorway: engine.Run(spec, tools, history, msg) ──
```

**Decoupling lives in this boundary, not in a separate process.** Two packages with a
clean interface are fully decoupled even inside one binary. Splitting into separate
services is a *deployment* choice, a different thing from decoupling. We decouple in
code (clean packages + one interface) and co-locate in one process (nothing forces a split).

---

## 4. Brain vs. harness vs. tools (inside the engine)

Three players; two live in the engine, one is remote:

| Player | Where | Deterministic? | Job |
|---|---|---|---|
| **Brain** (LLM) | remote (OpenAI) | No | **Decides** — emits text or a `tool_calls` intent |
| **Harness / loop** | engine (local Go) | Yes | **Executes** the intent, feeds result back, repeats |
| **Tool executor** | engine (local code) | Yes | Does the actual work (fetch, calculate, …) |

The **harness and the loop are the same thing** — the deterministic conductor. The
thing that *decides* the intent is the remote brain. The loop asks the brain "what now?",
runs whatever the brain asked for, feeds the result back, and repeats until the brain
stops requesting tools (OpenAI `finish_reason: "stop"`). **Nothing in the engine stores anything.**

---

## 5. Capabilities are ids; resolution splits them A/B

The row stores tool **names**, not code. At runtime the engine resolves each name into
**two** things via a registry lookup:

- **(A) Definition** — name + description + JSON schema → goes **into the LLM's context**
  (so the brain knows the tool exists and how to call it).
- **(B) Executor** — the actual code (and any credentials) → **stays in the harness**,
  never shown to the LLM.

> The model knows *what* it can do (definitions). The *doing* — and especially the
> secrets — lives in deterministic code the model never sees.

This is why creating agent #101 is an INSERT: the tools already sit in the registry,
waiting to be named.

---

## 6. Statelessness = crash safety

Because the engine holds no state, "the server crashed" is a hiccup, not a disaster:

- A crash mid-turn loses only the **one in-flight turn** (the stream breaks → an error).
- All memory was in the **store**, untouched.
- Recovery: a new request → fresh process → **read history back from the store** →
  build a brand-new engine → continue from the last saved turn.

Resume happens at the **turn boundary**, not mid-rollout. The dead turn isn't resumed at
tool-call #3; it's re-asked, and now it works. Statelessness is exactly what makes the
engine a disposable library you rebuild every turn.

---

## 7. Two IDs, two jobs

- **session_id** — minted **once per conversation**, reused every turn. It is the
  routing key (which agent, via the binding) and the continuity key (the history). This
  is what makes turn 2 remember turn 1.
- **invocation_id** — fresh **per run/turn**. The append-only audit/tracing key.

Don't confuse them: session = the durable thread; invocation = one rollout within it.
