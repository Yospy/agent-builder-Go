# Sprint 10 — Guided Builder Questions

## Scope

- Add session-scoped Builder draft state.
- Add an `ask_user_question` platform tool that pauses Builder runs for a structured answer.
- Stream `user_question` events over SSE and resume through `/api/sessions/:id/answer`.
- Render Builder questions as inline decision cards in chat.
- Preserve existing create-agent confirmation, abort, audit, and clean-history behavior.

## Assumptions

- The guided question flow is Builder-only in v1.
- Builder state is session-scoped and persists only when a turn completes successfully.
- Question option values are strings in v1.
- Questions are first-class chat items, not generic tool-call rows.

## Architectural Decisions

- Reuse the existing engine pause/resume shape, but keep answers separate from consequential approvals.
- Store pending question waiters in runtime memory, keyed by `session_id:call_id`, matching confirmation waiters.
- Persist `messages_json` and `builder_state_json` together after `done`.
- Keep the engine stateless: runtime supplies the user-question hook and state commit callback.

## Tasks

1. [x] Add sprint record and task checklist.
2. [x] Add `builder_state_json` storage and commit path.
3. [x] Add `ask_user_question`, `user_question`, `/answer`, and backend tests.
4. [x] Add frontend types, run-state handling, answer API, and `BuilderQuestionCard`.
5. [x] Update docs/contracts and verify backend/frontend.

## Completion Notes — 2026-06-13

- Added session-scoped Builder state with successful-turn-only commit behavior.
- Added `ask_user_question`, `user_question`, and `/api/sessions/:id/answer`.
- Added inline `BuilderQuestionCard` with option tiles, custom answer disclosure, and answered summary.
- Verification clean: `go test ./...`, `npm run lint`, `npx tsc --noEmit`, and production build with Node >=20.9.0.

## Risks

- If state commits before `done`, aborted turns can leave stale builder drafts.
- If the question waiter is coupled to approval, UI copy and behavior will imply the wrong decision.
- Existing builder rows may not receive the new tool unless seeding updates stale rows.

## Verification Strategy

- Run `go test ./...` in `server/`.
- Run `npm run lint`, `npx tsc --noEmit`, and `npm run build` in `web/`.
- Smoke the Builder flow through one option answer, one custom answer, and final create-agent approval.
