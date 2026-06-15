# Web UI — Plan & Spec (v1)

> Implementation reference for the Next.js frontend in `web/`. The API/SSE
> contract it builds against is `00-CONTEXT.md` (§7 events, §12 endpoints) —
> that file wins on any conflict. Sprint tracking: `sprints/sprint-01-web-ui.md`,
> `tasks/todo.md`.

## Locked decisions

- `web/` (Next.js App Router + TS + Tailwind + shadcn/ui) beside the Go
  backend as plain monorepo folders. No auth. No mocks — backend (:8080) is
  built in parallel.
- Dev wiring: Next rewrites proxy `/api/*` → `http://localhost:8080/api/*`.
- Recent normal chats: localStorage. Agent edit chats: server-backed
  `sessions.kind = "agent_edit"` and listed under each agent.
- **Monochrome only** — pure black/white/neutral greys; no accent colors
  anywhere. Build mode is distinguished by an inverted (black-on-white ⇄
  white-on-black) header treatment + a "designing a new agent" label, never
  by hue. Status (✓/✗) carried by icon + text, which monochrome forces anyway.
- **One chat message stack** for Builder, agents, and agent-edit chats. Normal
  chat mode is still derived from the session's agent; edit mode is derived
  from `sessions.kind === "agent_edit"`.
- No agent-creation endpoint exists: "+ New Agent" = open a Builder session.

## Routes

| Route | What |
|---|---|
| `/` | Agents list. `Chat` → `POST /api/sessions` → `/chat/:sessionId`; `Edit Agent` → `POST /api/agents/:id/edit-sessions` |
| `/chat/[sessionId]` | THE chat. Header `GET /api/agents/:id`, history `GET /api/sessions/:id`, turn `POST .../run` (SSE) |
| `/agents/[agentId]/edit/[sessionId]` | Agent edit chat. Same message stack + right context panel with config and versions |

## File map

```
web/
├── app/
│   ├── layout.tsx                shell: sidebar + content
│   ├── page.tsx                  agents list
│   ├── chat/[sessionId]/page.tsx chat view
│   └── agents/[agentId]/edit/[sessionId]/page.tsx edit chat view
├── components/
│   ├── sidebar/                  new-agent CTA, agents nav, recent chats
│   ├── agents/                   agent-card, edit chat view, context panel
│   └── chat/                     chat-header, message-list, message,
│                                 tool-call-row, confirm-card,
│                                 agent-preview-card, success-card,
│                                 composer, chat-empty-state
├── hooks/use-run.ts              run state machine
└── lib/
    ├── types.ts                  Agent, Session, SSE Event union (§7)
    ├── api.ts                    typed client, 6 endpoints, typed errors
    ├── sse.ts                    frame parser over fetch ReadableStream
    ├── recent-chats.ts           localStorage store for normal chats
    └── events.ts                 sidebar refresh events
```

## The run state machine (`use-run`)

```
idle ──send──► streaming ──confirm──► awaiting_approval ──approve/deny──► streaming
  │               │
  │               └──user_question──► awaiting_answer ──answer──► streaming
  ▲                                                                    │
  └── done / error / aborted ◄─────────────────────────────────────────┘
Stop (AbortController) from streaming|awaiting_* → aborted
```

- User message rendered as an **optimistic update**; on `aborted`/`error` the
  partial turn **rolls back** (mirrors §9 — server never committed it).
- `confirm` → POST `/approve`, `user_question` → POST `/answer`; both keep reading the original stream.
- Double-send blocked locally; server 409 → toast.
- Client-side 10,000-char cap mirrors the server validator.

## Event → renderer map (§7)

| Event | Renders as |
|---|---|
| `llm_text` | token append in assistant message, blinking caret |
| `status` | transient activity row; removed when `done` finalizes |
| `tool_use` | tool-call row, spinner state |
| `tool_result` | row crossfades to ✓ done / ✗ failed (icon + text, never color-only) |
| `confirm` | inline confirmation card (generic) / agent preview card (`create_agent`) / apply-changes card (`update_agent`) |
| `done` | finalize turn, composer → idle |
| `error` | turn rolled back + error toast (server doesn't commit errored turns — §11 "history stays clean"); draft restored to composer |
| `aborted` | partial turn fades out (rollback), composer → idle, draft restored |

## UI vocabulary (locked via /vocabulary skill)

| Element | Term | Consequence |
|---|---|---|
| Tool names on cards/header | **Badge** (attached, informational) | non-interactive, no remove affordance |
| Tool activity in message | tool-call row, **progressive disclosure** | collapsed name+status; expand for args/result (Collapsible) |
| In-flight tool | **Spinner** (action-level wait only) | never used for page/list loads |
| List/history loading | **Skeleton** | holds shape, no layout shift |
| Approve prompt | inline **confirmation dialog** (NOT modal) | describes what will happen; safe/consequential actions far apart |
| 409 | **Toast** | auto-dismiss |
| Agent glyph | **Avatar** | initials fallback |
| Pre-ack user msg | **Optimistic update** | rollback on abort/error |
| Fresh-chat welcome | **Empty state** | persona + capabilities + consequential tools flagged "asks first" |
| Approve copy | **CTA**, front-loaded, sentence case | "Create agent", not "Approve" |
| Char counter | **tabular nums** | digits don't jiggle |

## Motion spec

`motion` lib for enter/exit/layout; CSS for hover/active. Opacity/transform
only; all behind `prefers-reduced-motion`. Whisper level ~150ms everywhere
except the three product beats:

1. **Confirm/question card in** — scale 0.96→1 + fade, ~250ms ease-out (loop waiting on the user).
2. **Success card** — spring with slight overshoot (the celebration: agent is live).
3. **New agent slides into sidebar** — the INSERT made visible.

Also: agents-list stagger ~40ms · Send⇄Stop icon crossfade · pinned-to-bottom
autoscroll that unpins on user scroll-up · abort fade-out quicker than entrances.

## shadcn components

button · card · badge · avatar · textarea · scroll-area · skeleton · sonner ·
separator · tooltip · collapsible. **Deliberately no dialog/modal** — the
confirmation is inline by design.

---

## PHASE 1 — The working contract (function)

> Exit: the full build-an-agent → use-the-agent journey works end to end
> against the real backend, visually plain but contract-complete.

| Slice | Builds |
|---|---|
| 1.0 Foundation | scaffold, shadcn (neutral base), monochrome token check, rewrites |
| 1.1 Contract layer | types.ts · api.ts · sse.ts · recent-chats.ts (verify: tsc alone) |
| 1.2 Shell + list | sidebar (CTA, AGENTS, CHATS), agent cards w/ badges, skeletons, empty state |
| 1.3 Chat static | header, message column (~65ch), composer + counter, history load |
| 1.4 Live run | use-run machine, token streaming, tool-call rows, Stop, 409 toast, error/aborted |
| 1.5 Plain confirm gate | **generic** inline confirm row [Approve][Deny] → /approve → resume stream |

Phase-1 verification: `tsc` + `next build` + lint clean; every §7 event and
§12 status handled; abort rolls back; approve resumes original stream.

## PHASE 2 — The product identity (experience)

> Exit: build mode feels distinct, the three beats land, a11y passes.
> Requires Phase 1 done; ideally smoke-tested against the live backend first.

| Slice | Builds |
|---|---|
| 2.0 Build-mode identity | inverted monochrome header + filled avatar when `agent_id === "builder"`, "designing a new agent" label |
| 2.1 Agent preview card | `create_agent` confirm rendered as the agent card (reuses agents/agent-card) |
| 2.2 Success bridge | "agent is live" card + "Start chatting →" (mint session w/ new id) + sidebar AGENTS refresh |
| 2.3 Empty states | persona-as-welcome per agent, capability list, "asks first" flags |
| 2.4 Motion pass | stagger, springs, crossfades, autoscroll behavior, reduced-motion |
| 2.5 A11y + polish | focus states, semantic HTML, tab order, WCAG AA, muted disabled tokens, tooltips |

Phase-2 verification: design checklist (three beats present), reduced-motion
honored, contrast pass, E2E journey in the browser with the real backend.

## Risks — RESOLVED in integration (2026-06-12)

- ~~SSE through Next dev proxy~~ — streams fine, but the proxy's default 30s
  `proxyTimeout` killed idle paused-confirm streams. Fixed:
  `experimental.proxyTimeout = 600_000` in next.config.ts.
- ~~`create_agent` tool_result shape~~ — it's plain text containing the id
  (`created agent agent-<hex> ("Name") …`); success card extracts by regex
  with list+name fallback.
- Observed backend behaviors now encoded in the UI: tool_use AND confirm
  arrive for the same call_id (rows are upgraded, not duplicated); history is
  normalized OpenAI format incl. role:"tool" messages; no llm_text deltas —
  final text arrives only in `done`.
