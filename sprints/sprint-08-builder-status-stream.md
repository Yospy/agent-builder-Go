# Sprint 08 — Builder Status Stream

## Scope

- Add a normalized `status` SSE event for Builder lifecycle activity.
- Emit deterministic statuses while building agents.
- Render statuses in chat so confirmation does not look like a silent loop.

## Assumptions

- Status text is operational activity, not private chain-of-thought.
- Statuses are transient UI state and are not persisted into chat history.
- Builder-only statuses avoid misleading normal agents with agent-build language.

## Tasks

1. [x] Add sprint note.
2. [x] Add backend `status` event and Builder lifecycle emissions.
3. [x] Add frontend union handling and activity rendering.
4. [x] Update protocol docs and tests.
5. [x] Verify backend, frontend, build, and browser smoke.

## Verification

- `go test ./...`
- `npm run lint`
- `npx tsc --noEmit`
- Production build with Node >=20.9.0
- Browser smoke confirms status rows and confirm preview appear.

## Completion Notes — 2026-06-13

- Added backend `status` SSE events scoped to Builder runs.
- Builder now emits: `Preparing agent brief`, `Checking available tools`,
  `Drafting agent spec`, `Waiting for approval`, `Creating agent`, and
  `Saving agent`.
- Frontend parses `status`, renders it as a transient activity row, and removes
  status rows when `done` finalizes so chat history stays clean.
- Added engine coverage for ordered Builder statuses and fixed a runtime test
  race where persistence was asserted immediately after the flushed `done`
  frame.
- Verification clean: `go test ./...`, `npm run lint`, `npx tsc --noEmit`,
  production build, and local browser load smoke. Live Builder run was not
  exercised because it would call the external OpenAI API.
