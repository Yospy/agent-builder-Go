# Sprint 12 — Runtime Diagnostics

## Scope

Diagnose Builder runs that appear stuck after submission by adding high-signal backend logging around request intake, SSE delivery, engine loop progress, and OpenAI calls.

## Assumptions

- The current hang is likely inside the non-streaming OpenAI call or around SSE delivery.
- Logs should be detailed enough for local debugging without changing the user-facing protocol.
- A bounded provider call is acceptable for local diagnosis so failures become visible.

## Architectural Decisions

- Keep diagnostics at package boundaries: runtime, engine, and brain.
- Do not persist partial turns on timeout/error; preserve the existing clean-history invariant.
- Make the OpenAI timeout configurable with `OPENAI_TIMEOUT_SECONDS`.

## Tasks

1. Add sprint record and todo checklist.
2. Add runtime logs for `/run`, SSE sends, answer/approval waits, and persistence.
3. Add engine logs around tool resolution, status emission, brain calls, and tool/user-input flow.
4. Add OpenAI request timing/error logs and configurable timeout.
5. Run backend tests and review the diff.

## Risks

- Over-logging could make local logs noisy. Keep fields structured and focused.
- Timeout must not convert a successful long run into a persisted partial turn.

## Verification Strategy

- Run `go test ./...` in `server/`.
- Review diff for protocol compatibility and persistence invariants.
- Re-run the stuck Builder flow and inspect logs for the last completed boundary.
