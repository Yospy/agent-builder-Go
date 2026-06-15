# todo — Sprint 01: Web UI (ref: docs/07-ui-plan.md)

## Sprint 20 — GitHub Publish Prep

- [x] 20.1 Inspect current git state, remote, and local artifacts
- [x] 20.2 Add root `.gitignore`, `.env.example`, and detailed `README.md`
- [x] 20.3 Initialize this directory as its own git repo and review tracked files
- [x] 20.4 Run backend and frontend verification
- [x] 20.5 Commit on `main` and push to GitHub

## Sprint 19 — Agent Edit Chats

- [x] 19.1 Add sprint record and task checklist
- [x] 19.2 Extend backend schema/store for sources, edit sessions, and versions
- [x] 19.3 Add scoped versioned `update_agent` behavior
- [x] 19.4 Add edit-session/version API endpoints and runtime edit mode
- [x] 19.5 Add frontend edit action, sidebar grouping, edit route, context panel, and confirmation copy
- [x] 19.6 Verify backend/frontend, browser smoke, docs, and subagent review
- [x] 19.7 Add delete affordance for agent edit chats

## Sprint 18 — Builder Question Keyboard Navigation

- [x] 18.1 Add sprint record and task checklist
- [x] 18.2 Investigate current keyboard behavior and identify root cause
- [x] 18.3 Implement roving keyboard navigation and selection
- [x] 18.4 Verify lint, TypeScript, build, and side effects

## Sprint 17 — Normal Status + Token Streaming

- [x] 17.1 Add sprint record and task checklist
- [x] 17.2 Add generic normal-agent status emissions
- [x] 17.3 Stream OpenAI text deltas through `llm_text`
- [x] 17.4 Seed normal-agent UI runs with an initial activity status
- [x] 17.5 Update tests for status order, streamed text, and duplicate prevention
- [x] 17.6 Verify backend, frontend, and side effects

## Sprint 16 — Chat Markdown Rendering

- [x] 16.1 Add sprint record and task checklist
- [x] 16.2 Add safe Markdown subset renderer for assistant messages
- [x] 16.3 Wire assistant text items to use the renderer
- [x] 16.4 Verify lint, TypeScript, build, and UI rendering

## Sprint 15 — Builder SSE Question Rendering

- [x] 15.1 Add sprint record and task checklist
- [x] 15.2 Add dedicated Next route handler for `/run` SSE streaming
- [x] 15.3 Move generic API rewrite to fallback precedence
- [x] 15.4 Add dev-only malformed SSE frame diagnostics
- [x] 15.5 Verify static checks, route precedence, live Builder question rendering, and side effects

## Sprint 14 — ChatGPT-Style Composer

- [x] 14.1 Add sprint record and task checklist
- [x] 14.2 Implement clamped auto-grow with hidden scrollbar chrome
- [x] 14.3 Refine composer spacing, focus, and button alignment
- [x] 14.4 Verify lint, TypeScript, build, and side effects

## Sprint 13 — New Agent Sidebar Behavior

- [x] 13.1 Add sprint record and task checklist
- [x] 13.2 Add skip-recent-chat option to session start hook
- [x] 13.3 Use the option from sidebar `New agent` only
- [x] 13.4 Verify frontend lint/build and side effects

## Sprint 12 — Runtime Diagnostics

- [x] 12.1 Add sprint record and task checklist
- [x] 12.2 Add runtime logs for run intake, SSE delivery, waiters, and persistence
- [x] 12.3 Add engine logs around status, brain calls, and tool/user-input flow
- [x] 12.4 Add OpenAI request timing/error logs and configurable timeout
- [x] 12.5 Verify backend tests and diff side effects

## Sprint 11 — Compact Chat Titles

- [x] 11.1 Add sprint record and task checklist
- [x] 11.2 Add backend title summarizer endpoint and tests
- [x] 11.3 Wire frontend title generation with fallback
- [x] 11.4 Refine sidebar chat row delete affordance
- [x] 11.5 Verify tests, lint/build, and side effects

## Sprint 10 — Guided Builder Questions

- [x] 10.1 Add sprint record and task checklist
- [x] 10.2 Add `builder_state_json` storage and commit path
- [x] 10.3 Add `ask_user_question`, `user_question`, `/answer`, and backend tests
- [x] 10.4 Add frontend types, run-state handling, answer API, and `BuilderQuestionCard`
- [x] 10.5 Update docs/contracts and verify backend/frontend

## Sprint 09 — Builder Live Status

- [x] 9.1 Add sprint note
- [x] 9.2 Seed Builder runs with initial status
- [x] 9.3 Render status events as animated live status
- [x] 9.4 Verify frontend checks and build
- [x] 9.5 Replace status history/checklist with latest-status-only UI

## Sprint 08 — Builder Status Stream

- [x] 8.1 Add sprint note
- [x] 8.2 Add backend `status` event and Builder lifecycle emissions
- [x] 8.3 Add frontend union handling and activity rendering
- [x] 8.4 Update protocol docs and tests
- [x] 8.5 Verify backend, frontend, build, and browser smoke

## Sprint 07 — Single Create Entry

- [x] 7.1 Add sprint note
- [x] 7.2 Filter Builder out of sidebar agent list
- [x] 7.3 Filter Builder out of registry and empty state actions
- [x] 7.4 Verify lint, TypeScript, build, and browser smoke

## Sprint 06 — Minimal Agent Index

- [x] 6.1 Add sprint note
- [x] 6.2 Simplify index layout and remove top-right Builder CTA
- [x] 6.3 Simplify agent card chrome
- [x] 6.4 Verify lint/build and side effects

## Sprint 05 — Agent Creation Experience

- [x] 5.1 Add sprint record and task checklist
- [x] 5.2 Redesign index header, creation rail, templates, and state surfaces
- [x] 5.3 Rebuild agent cards with stronger hierarchy, tags, and action rail
- [x] 5.4 Review responsive behavior, accessibility, and side effects
- [x] 5.5 Run lint, TypeScript, build, and local browser smoke

## Sprint 03 — Delete Backend Contract

- [x] 3.0 Add sprint plan for backend delete endpoints
- [x] 3.1 Add store delete methods and tests
- [x] 3.2 Add DELETE HTTP handlers and tests
- [x] 3.3 Run `go test ./...` and review side effects
- [x] 3.4 Document frontend integration contract

## Sprint 02 — Theme, Delete UI, Refinement

- [x] 2.6 Wire light/dark mode provider, root hydration guard, and sidebar
      theme toggle
- [x] 2.7 Add no-content delete API helpers and recent-chat cleanup helpers
- [x] 2.8 Add inline delete confirmation for built agent cards; exclude builder
- [x] 2.9 Add inline delete confirmation for sidebar chats; navigate home when
      deleting active chat
- [x] 2.10 Refine shell hierarchy: continuous headers, sidebar footer, composer
      footer, spacing, boundaries, progressive counter
- [x] 2.11 Verify lint, TypeScript, production build, server tests, and browser
      theme smoke
- [x] 2.12 Fix delete runtime + UX regression: restart stale backend, replace
      inline confirmations with popovers, verify temp chat delete via UI

## Sprint 04 — UI Uniformity Pass

- [x] 4.1 Tighten page shell and header metrics
- [x] 4.2 Rebuild agent cards with equal-height zones and aligned actions
- [x] 4.3 Normalize sidebar row heights, gutters, and footer spacing
- [x] 4.4 Refine delete popover hierarchy and action alignment
- [x] 4.5 Verify lint, TypeScript, build, and browser smoke
- [x] 4.6 Fix builder chat dark-mode surface blending and tool tag treatment

## Phase 1 — The working contract

- [x] 1.0 Foundation: scaffold web/, shadcn, semantic tokens, rewrites → :8080
- [x] 1.1 Contract layer: types.ts, api.ts, sse.ts, recent-chats.ts (tsc)
- [x] 1.2 Shell + agents list: sidebar, cards + badges, skeletons, empty state
- [x] 1.3 Chat static: header, message column, composer + counter, history
- [x] 1.4 Live run: use-run, streaming, tool-call rows, Stop, 409, error/abort
- [x] 1.5 Plain confirm gate: generic [Approve][Deny] → /approve → resume
- [x] Phase 1 verify: tsc/build/lint clean; smoke test both routes 200
- [x] Phase 1 subagent review + fixes (2 critical, 3 should-fix, 3 nits applied)

## Phase 2 — The product identity

- [x] 2.0 Build-mode identity (inverted monochrome header + label + filled avatar)
- [x] 2.1 Agent preview card (create_agent confirm = agent card, "Create agent"/"Not yet")
- [x] 2.2 Success bridge: "agent is live" + Start chatting → + sidebar refresh
- [x] 2.3 Empty states: persona welcome + capabilities + "asks first" (lib/tool-catalog.ts)
- [x] 2.4 Motion pass: stagger 40ms, confirm scale-in, success spring, Send⇄Stop
      crossfade, sidebar slide-in, exit fades, MotionConfig reducedMotion="user"
- [x] 2.5 A11y + polish: badge tooltips (keyboard-focusable), composer aria,
      draft restore on rollback, dark-token chroma zeroed
- [x] Phase 2 subagent review + fixes (0 critical, 4 should-fix, nits applied)
- [x] E2E vs real backend (2026-06-12): full journey verified over the proxy —
      builder confirm → approve → create → chat with created agent → history
      reload; deny path verified (ok:false "denied by user")

## Integration findings + fixes (2026-06-12, real backend in server/)

1. Next dev proxy killed idle SSE after 30s (default proxyTimeout) → aborted
   any confirm gate awaiting a human. Fixed: experimental.proxyTimeout=600s.
   Proven: confirm held 46s, then approved successfully.
2. Backend emits BOTH tool_use and confirm for the same call_id → use-run now
   upgrades the existing row instead of duplicating (dup React keys avoided).
3. Session history is normalized OpenAI format (assistant tool_calls entries,
   role:"tool" results) — historyToDisplay now rebuilds turns with tool rows;
   ChatMessage type widened.
4. create_agent tool_result.data is PLAIN TEXT: `created agent agent-<hex>
   ("Name") with tools [...]` — success card now extracts the id by regex
   (JSON parse first, then regex, then list+name fallback).
5. Historical: no llm_text deltas from this backend at Phase 2. Sprint 17 added
   OpenAI token streaming while keeping done.text authoritative.

Note: integration testing created agent "Testy2" (agent-71f7ad9402281a8f) in
agent-builder.db — no delete endpoint in v1 to clean it up.

## Review — Phase 2 (2026-06-12)

All five slices implemented and reviewed by subagent: verdict "complete per
the plan, no Phase 1 regressions, no criticals." Fixes applied from review:
- AnimatePresence stays mounted when rollback empties the chat (exit fade now
  fires on the first-message abort path); empty state cross-fades back in
- Success card CTA disabled while the turn is still streaming (clicking would
  navigate away → §9 abort → builder turn dropped from history)
- Badge tooltips keyboard-focusable (tabIndex=0); +N badge tooltip lists
  hidden tools
- SuccessCard falls back to "New agent" name (matches preview card)
- Empty-state builder avatar uses filled (dark) treatment, not inverted
- Dead error-item code removed (error event rolls back + toasts per the
  intentional use-run change); 07-ui-plan event map amended to match
- Composer: aria-label + aria-invalid on over-limit

Verification: tsc, eslint, next build clean; both routes 200.

## Review — Phase 1 (2026-06-11)

Implemented in web/: contract layer (types/api/sse/recent-chats), app shell +
agents list, static chat surface, use-run state machine (all 7 §7 events),
plain inline confirm gate. Monochrome theme (destructive token flattened to
grey). tsc + eslint + next build clean; both routes smoke-tested (200).

Subagent review verdict after fixes: contract-complete vs 00-CONTEXT.md.
Fixes applied from review:
- CRITICAL: done.text now authoritative (replace trailing text / append after
  tool rows) — final answer was being dropped after tool activity
- CRITICAL: abort on unmount — navigating away mid-run/paused no longer
  strands the server's per-session 409 lock
- Stop button enabled while paused (only escape if /approve fails)
- Stream-drop without terminal event now rolls back turn (§9 parity) + toast
- ConfirmCard re-enables buttons if /approve POST fails
- History load: spinner → skeleton (vocabulary spec)
- Denied tool_result stays "denied", not "failed"; TextDecoder flush +
  reader.cancel in sse.ts; IME composition guard on Enter

Deferred (noted, not bugs): restore rolled-back message into composer;
confirm with backend whether `error` events commit the partial turn
server-side (UI currently keeps it visible; reload would drop it).

Open for backend team: shape of create_agent tool_result.data (Phase 2
success card needs the new agent id); SSE buffering through Next dev proxy
untested until backend lands.
