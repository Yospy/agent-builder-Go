# Sprint 15 — Builder SSE Question Rendering

## Scope

- Route Builder `/run` SSE traffic through a dedicated Next route handler.
- Preserve the existing browser-facing `/api/sessions/:id/run` API.
- Ensure backend `status` and `user_question` SSE frames reach the existing UI state machine live.

## Assumptions

- Backend SSE emission is correct; logs show flushed `status` and `user_question` events.
- The current failure is in the Next rewrite/proxy path for long-lived paused SSE streams.
- Direct browser calls to `localhost:8080` are out of scope because the Go backend has no CORS layer.

## Architectural Decisions

- Add a filesystem App Router route for `POST /api/sessions/[id]/run`.
- Move the generic `/api/:path*` backend rewrite to `fallback` so filesystem routes win first.
- Stream `upstream.body` unchanged; do not parse or transform SSE in the route handler.
- Keep `api.run()` unchanged so the frontend contract remains stable.

## Tasks

1. Add sprint record and task checklist.
2. Add the dedicated Next route handler for run SSE streaming.
3. Move the generic API rewrite to fallback precedence.
4. Add dev-only malformed SSE frame diagnostics.
5. Verify static checks, route precedence, live Builder question rendering, and side effects.

## Risks

- Route precedence could still hit the generic rewrite if fallback is not configured correctly.
- Route handler typing may differ under Next 16 unless `params` is handled as a promise.
- Streaming verification needs an active backend and a prompt that reliably triggers `ask_user_question`.

## Verification Strategy

- Run `npm run lint`, `npx tsc --noEmit`, and `npm run build` in `web/`.
- Smoke `POST /api/sessions/:id/run` through port 3000 and verify `X-Agent-Builder-Route: next-run-stream`.
- Confirm UI status advances beyond the local optimistic status, renders the question card, and puts the composer in `awaiting_answer`.
- Confirm existing JSON APIs still work through the fallback rewrite.
