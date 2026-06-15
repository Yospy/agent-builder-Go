# Sprint 11 — Compact Chat Titles

## Scope

- Replace prompt-length sidebar chat titles with compact 3-4 word labels.
- Generate labels with a small OpenAI nano model on the backend.
- Keep delete controls visible and reachable in the sidebar row.

## Assumptions

- Recent chats remain browser-local state; only the displayed/stored title changes.
- The Go service owns OpenAI credentials, so title generation belongs behind `/api/*`.
- `gpt-5.4-nano` is the current documented small nano model; expose an env override.
- If the model call fails, the UI should still store a short deterministic fallback.

## Architectural Decisions

- Add an injected `TitleSummarizer` dependency to `runtime.Server` instead of exposing OpenAI keys to the web app.
- Use the OpenAI Responses API for the title model, with low verbosity and a small output cap.
- Keep title sanitization on both backend and frontend: model output is advisory, UI layout is still protected.

## Tasks

- [x] Add sprint record and task checklist.
- [x] Add backend title summarizer endpoint and tests.
- [x] Wire frontend title generation with fallback.
- [x] Refine sidebar chat row delete affordance.
- [x] Verify tests, lint/build, and side effects.

## Risks

- Model call latency could delay first title update; mitigate by committing the turn first and updating title asynchronously.
- Model output could include punctuation or extra words; mitigate with normalization and word cap.
- Backend without the new endpoint should not break chat; frontend falls back locally.

## Verification Strategy

- Run `go test ./...` in `server/`.
- Run `npm run lint` and `npm run build` in `web/`.
- Review the diff for minimality, API boundaries, and fallback behavior.
