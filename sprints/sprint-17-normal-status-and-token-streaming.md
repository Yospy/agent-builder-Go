# Sprint 17 — Normal Status + Token Streaming

## Scope

- Emit live operational statuses for normal agent runs, not only Builder runs.
- Stream OpenAI assistant text through existing `llm_text` SSE events.
- Keep the existing `/run` SSE endpoint and event wire shapes unchanged.

## Assumptions

- Status text is operational activity, not private chain-of-thought.
- Token streaming applies to assistant text deltas only.
- Partial tool-call JSON is accumulated server-side until the tool call is complete.
- `done.text` remains the authoritative final assistant answer.

## Architectural Decisions

- Keep Chat Completions and use the installed OpenAI Go SDK streaming API.
- Extend the internal `brain.Brain` boundary with a text-delta callback rather
  than moving provider concerns into the engine.
- Add `Step.TextStreamed` so the engine can avoid duplicate pre-tool text.
- Keep status events transient in the UI and remove them on `done`.

## Tasks

1. [x] Add sprint record and task checklist.
2. [x] Add generic normal-agent status emissions.
3. [x] Stream OpenAI text deltas through `llm_text`.
4. [x] Seed normal-agent UI runs with an initial activity status.
5. [x] Update tests for status order, streamed text, and duplicate prevention.
6. [x] Verify backend, frontend, and side effects.

## Completion Notes — 2026-06-14

- Normal agent runs now emit transient activity statuses.
- OpenAI chat completions now stream text deltas through `llm_text` and still
  return authoritative final content in `done.text`.
- Tool-call deltas are accumulated in the brain and emitted as `tool_use` only
  once complete.
- Verification passed: `go test ./...`, `npm run lint`, `npx tsc --noEmit`,
  and production build with bundled Node >=20.9.0.

## Risks

- Streaming tool-call deltas may arrive over multiple chunks and must not be
  exposed to tools until complete.
- Existing fake-brain tests should remain deterministic while the real brain
  becomes streaming.
- `llm_text` plus `done.text` must not double-render final answers.

## Verification Strategy

- `go test ./...` in `server/`.
- `npm run lint`, `npx tsc --noEmit`, and `npm run build` in `web/`.
- Review changed files for protocol compatibility and persistence invariants.
