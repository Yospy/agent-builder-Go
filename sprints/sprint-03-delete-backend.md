# Sprint 03 — Delete Backend Contract

## Scope
Add backend support for deleting user-created agents and chats. The seeded Agent Builder (`id=builder`) is protected and must never be deleted through the API.

## Assumptions
- A chat is a `sessions` row.
- Deleting an agent also deletes its sessions so the database does not keep orphaned chats.
- Audit JSONL files are append-only historical records and are not removed by these endpoints.
- In-flight runs are protected by the existing session lock; deleting an in-flight chat should return `409`.

## Architectural Decisions
- Use REST resource deletes:
  - `DELETE /api/agents/{id}`
  - `DELETE /api/sessions/{id}`
- Return `204 No Content` on successful delete.
- Return existing error envelope `{ "error": "..." }` for failures.
- Store deletion remains in `store`; HTTP policy checks remain in `runtime`.

## Tasks
- Add store methods for deleting sessions and agents.
- Protect `builder` from agent deletion.
- Delete an agent's sessions in the same transaction as the agent.
- Add DELETE routes and handlers.
- Add store and runtime tests for success, 404, 400/409, and builder protection.

## Risks
- Foreign key constraints require deleting sessions before deleting an agent.
- Deleting a chat while `/run` is active could race with persistence; use the inflight guard.

## Verification Strategy
- Run targeted store/runtime tests.
- Run `go test ./...`.
- Review diff for minimal API surface and side effects.
