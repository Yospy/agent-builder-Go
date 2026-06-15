# Sprint 09 — Builder Live Status

## Scope

- Replace the Builder's generic three-dot wait state with one animated live
  status indicator.
- Seed Builder runs with the first status immediately, before the first SSE
  frame arrives.
- Replace the visible status text as backend `status` events arrive; do not
  show a full checklist of possible states.
- Keep normal agent runs on the generic loading dots.

## Assumptions

- Backend `status` events remain the source of truth after the first local
  optimistic status.
- Status rows are transient and removed on `done`.
- The live status should be compact enough to sit in the chat stream.

## Tasks

1. [x] Add sprint note.
2. [x] Seed Builder runs with initial status.
3. [x] Render status events as animated live status.
4. [x] Verify frontend checks and build.
5. [x] Replace status history/checklist with latest-status-only UI.

## Verification

- `npm run lint` passed.
- `npx tsc --noEmit` passed.
- `PATH=/Users/yashwadgave/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npm run build` passed.
- `go test ./...` passed from `server/`.
- Browser smoke on `http://localhost:3000/` passed with no console errors.
- Live Builder run was not executed because it would call the external model API.
- Correction: status history/checklist was replaced with latest-status-only UI;
  `npm run lint`, `npx tsc --noEmit`, and production build passed again.
