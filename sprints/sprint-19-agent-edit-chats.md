# Sprint 19 — Agent Edit Chats

## Scope

- Add an `Edit Agent` flow where each click creates a new edit chat for that agent.
- Keep edit chats under `Agents > Agent name` in the sidebar.
- Keep normal user-agent chats under the global `Chats` section.
- Show current agent config, sources, tools, prompt preview, and versions in an edit-chat context panel.

## Assumptions

- Edit chats are persisted as `sessions.kind = "agent_edit"`.
- The session `agent_id` remains the target agent being edited.
- Runtime injects a synthetic editor spec for edit sessions, rather than granting `update_agent` to normal agents.
- Sources are simple string references in v1.
- Versions are visible in the edit context panel but rollback is out of scope.

## Architectural Decisions

- Extend `agent_specs` with `sources_json`.
- Extend `sessions` with `kind` and `title`.
- Store applied snapshots in `agent_spec_versions`.
- Use existing `/api/sessions/:id/run`, `/approve`, and `/answer` endpoints for edit chats.
- Restrict `update_agent` in edit sessions to the current session's target agent.

## Tasks

1. [x] Add sprint record and task checklist.
2. [x] Extend store schema, migrations, structs, and versioned update behavior.
3. [x] Extend platform `update_agent` with sources, edit-session scoping, and version writes.
4. [x] Add edit-session and version HTTP APIs.
5. [x] Run edit sessions through a synthetic Agent Editor spec.
6. [x] Add backend tests for sources, edit sessions, version writes, deny behavior, and delete cleanup.
7. [x] Add frontend types, API methods, and `useStartAgentEdit`.
8. [x] Add `Edit Agent` card action, agent-scoped sidebar edit chats, edit route, context panel, and update confirmation copy.
9. [x] Verify backend, frontend, browser behavior, and subagent review.
10. [x] Add confirmed delete affordance for agent edit chats in the sidebar.

## Completion Notes — 2026-06-14

- Added agent edit chats as `sessions.kind = "agent_edit"`.
- Added agent sources and version snapshots.
- Scoped edit-session `update_agent` calls to the session's target agent.
- Added `Edit Agent` card action, agent-scoped sidebar edit chats, edit route, context panel, and update confirmation copy.
- Subagent review found no critical issues; two should-fixes were applied.
- Added edit-chat deletion from the agent sidebar using the existing session delete API; applied agent config and version audit remain intact.

## Risks

- Edit sessions could accidentally expose `update_agent` outside the target agent.
- Versions could drift from applied changes if the update and version insert are not transactional.
- Sidebar could mix edit chats with normal chats if localStorage recent chats is reused.
- Context panel could show stale config after an applied update.

## Verification Strategy

- Run `go test ./...` in `server/`.
- Run `npm run lint`, `npx tsc --noEmit`, and `npm run build` in `web/`.
- Browser smoke the edit path with a live backend.
- Spawn a fresh subagent to review the diff and verify high-risk paths.
