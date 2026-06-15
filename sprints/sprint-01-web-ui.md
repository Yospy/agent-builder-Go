# Sprint 01 — Agent Builder Web UI (v1)

## Scope

Build the Next.js frontend in `web/` against the locked API contract in
`docs/00-CONTEXT.md` (§7 SSE protocol, §12 endpoints). UI only — the Go backend
(:8080) is being built in parallel by someone else. No mocks, no auth.

**Locked decisions (from user):**
- Monorepo as plain folders: `web/` (Next.js) beside the future Go backend.
- Next.js App Router + TypeScript + Tailwind + shadcn/ui.
- Dev wiring: Next rewrites proxy `/api/*` → `http://localhost:8080/api/*`.
- Recent-chats sidebar backed by localStorage (no list-sessions endpoint exists).

## Pages (v1)

| Route | What |
|---|---|
| `/` | Agents list (`GET /api/agents`). Click agent → mint session (`POST /api/sessions`) → chat. "+ New Agent" → same, with `agent_id: "builder"`. |
| `/chat/[sessionId]` | THE chat component (identical for builder + agents). Header from `GET /api/agents/:id`, history from `GET /api/sessions/:id`, streaming via `POST /api/sessions/:id/run`. |

## Architecture

```
web/
├── app/
│   ├── page.tsx                 agents list
│   └── chat/[sessionId]/page.tsx chat view
├── components/chat/             message-list, message, tool-call-row,
│                                confirm-card, agent-preview-card,
│                                success-card, composer, chat-header,
│                                chat-empty-state
├── components/agents/           agent-card, new-agent-button
├── components/sidebar/          recent-chats (localStorage)
├── hooks/
│   └── use-run.ts               run state machine (stream/confirm/abort)
├── lib/
│   ├── types.ts                 Agent, Session, SSE Event union (§7)
│   ├── api.ts                   typed client for the 6 endpoints
│   ├── sse.ts                   SSE frame parser over fetch ReadableStream
│   └── recent-chats.ts          localStorage store
└── next.config.ts               rewrites → :8080
```

## SSE handling (the hard part)

- `POST …/run` with `fetch(…, { signal })`; parse `data: <json>\n\n` frames
  from the ReadableStream.
- Event union per §7: `llm_text` (append delta) · `tool_use` (🔧 chip) ·
  `tool_result` (✓/✗ on chip) · `confirm` (Approve/Cancel card →
  `POST /approve`, then KEEP READING the original stream) · `done` (finalize) ·
  `error` (show, close) · `aborted` (mark canceled, close).
- Stop button → `AbortController.abort()`. On abort, drop the partial turn
  locally too (server doesn't commit it — keeps UI and DB history consistent).
- `409` on run → toast "a run is already in flight"; `400`/`404` → inline error.

## Implementation slices (each independently verifiable)

**Slice 0 — Foundation.** Scaffold `web/` (create-next-app TS + Tailwind, App
Router), shadcn init + components (button, card, textarea, scroll-area, badge,
sonner, skeleton, avatar, separator). Semantic tokens: tinted-neutral surface
palette, strictly monochrome (no accent colors). Rewrites → :8080.

**Slice 1 — Contract layer (no UI).** `lib/types.ts` (Agent, Session, SSE
Event discriminated union per §7) · `lib/api.ts` (6 endpoints, typed errors
incl. 409) · `lib/sse.ts` (frame parser over fetch ReadableStream) ·
`lib/recent-chats.ts` (localStorage). Verify with tsc alone.

**Slice 2 — App shell + agents list.** Sidebar ("+ New Agent" CTA, AGENTS
section, CHATS section) · agent cards with avatar (glyph/initials fallback) +
tool-name badges · skeleton loaders for the list (not spinners) · empty state
with a first action · staggered entrance (~40ms, ease-out, honors reduced
motion).

**Slice 3 — Chat surface, static.** Chat header (avatar + name + tool badges)
· message list in a max-width column (~65ch line length) · composer (textarea
with full state set + char counter in tabular nums, 10000 cap mirroring the
validator) · per-agent empty state (persona as welcome + capability list,
consequential tools flagged "asks first") · history load behind a skeleton.

**Slice 4 — Live run.** `use-run` state machine
(idle→streaming→paused→terminal) · llm_text token streaming · tool-call rows
(progressive disclosure: collapsed name + status, expandable args/result;
spinner while in flight; ✓/✗ always paired with text, never color-only) ·
done/error/aborted · optimistic update of the user message with rollback on
abort/error (mirrors §9: server drops uncommitted turns) · Stop button
(aria-label "Stop generating", AbortController) · 409 → toast.

**Slice 5 — Confirmation gate.** Inline confirmation dialog (deliberately NOT
a modal — no overlay/focus trap; the surrounding chat is the context) that
describes what will happen, safe/consequential actions visually far apart ·
`create_agent` variant rendered as the agent preview card (same visual as list
cards) · success card with "Start chatting →" CTA minting the new session ·
sidebar AGENTS refresh on create.

**Slice 6 — Accessibility + polish + verify.** Custom focus states (never
removed) · semantic HTML (real buttons) · tab order = DOM order · WCAG AA
contrast incl. disabled states via muted tokens (not opacity) · hover/active
states (~150ms, transition opacity/transform only) · `tsc --noEmit`,
`next build`, lint, contract walkthrough §7/§12.

## UI vocabulary spec (terms locked via /vocabulary)

| Element | Correct term | Implementation consequence |
|---|---|---|
| Tool names on agent cards/header | **Badge** (attached, informational) | shadcn `badge`, non-interactive, no remove affordance |
| Tool activity in a message | tool-call row with **progressive disclosure** ("chip" is not a term) | collapsed: name+status; click to expand args/result |
| In-flight tool status | **Spinner** (short action-level wait) | spinner only here; never for page/list loads |
| List/history loading | **Skeleton** | holds content shape, no layout shift; shimmer honors reduced motion |
| Approve/cancel prompt | **Confirmation dialog**, inline (not Modal) | describes what will happen; safe vs consequential actions far apart |
| 409 double-submit | **Toast** | auto-dismisses, duration scaled to text length |
| Agent glyph | **Avatar** | initials fallback required |
| User msg before server ack | **Optimistic update** | rollback on abort/error = §9 consistency |
| Persona welcome screen | **Empty state** | explains what agent does + offers first action |
| Approve button copy | **CTA**, front-loaded, sentence case | "Create agent", not "Approve"/"Submit" |
| Char counter digits | **Tabular nums** | digits don't jiggle as count changes |
| Tool ✓/✗ | never **color-only state** | icon + text always accompany color |

## Motion spec

Library: `motion` (Framer Motion successor) for enter/exit + layout animation;
CSS transitions for hover/active. Animate opacity/transform only (GPU-safe);
everything behind `prefers-reduced-motion`.

| Moment | Animation | Why it earns its place |
|---|---|---|
| Agents list entrance | stagger ~40ms, fade+rise, ease-out | "sense of arrival" on the home view |
| Token streaming + caret | native stream + blinking caret | the hero motion — real progress, not decoration |
| Tool-call row | spinner → ✓/✗ crossfade; expand/collapse via Collapsible + height spring | status change is feedback |
| Confirm card entrance | scale 0.96→1 + fade, ease-out, ~250ms | must pull attention — the loop is paused on the user |
| Success card ("agent is live") | spring entrance (slight overshoot) | the one celebration moment in the product |
| New agent in sidebar | fade+slide into AGENTS list | makes the INSERT visible — the product's payoff |
| Send ⇄ Stop button | icon crossfade ~150ms | state machine made visible |
| Chat autoscroll | pinned-to-bottom, smooth; unpin on user scroll-up | streaming must not fight reading |
| Aborted/partial-turn rollback | fade-out removal (exit faster than enter) | quiet exit — don't celebrate a cancel |

## shadcn component map

button · card · badge · avatar · textarea · scroll-area · skeleton · sonner
(409 toast) · separator · tooltip (tool badge → tool description on hover) ·
collapsible (tool-call row disclosure). No modal/dialog component — the
confirmation is inline by design.

## Risks / assumptions

- **SSE through Next dev proxy buffering** — Next rewrites stream fine in
  recent versions; if dev proxy buffers, fallback is calling :8080 directly
  with CORS in dev. Flag to backend if needed.
- **Backend not running** — final E2E verification deferred until the Go
  service lands; this sprint verifies by build + contract review.
- Assumes contract is frozen; any drift in 00-CONTEXT.md re-opens this sprint.

## Verification checklist

- [ ] `next build` passes clean
- [ ] every §7 event type has a renderer; every §12 status code has a handler
- [ ] abort drops local partial turn (matches §9 server behavior)
- [ ] approve flow posts and resumes reading the original stream (§8)
- [ ] builder chat and agent chat are literally the same component
