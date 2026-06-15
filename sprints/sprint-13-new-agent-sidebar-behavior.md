# Sprint 13 — New Agent Sidebar Behavior

## Scope

- Stop the `New agent` CTA from creating a placeholder `New chat` row.
- Preserve normal chat rows when starting conversations with already-created agents.

## Assumptions

- The Builder session is an agent-creation workflow, not a user-facing chat row by default.
- Created agents continue to appear through the existing agents list refresh path.
- Existing stale `New chat` rows can still be removed manually with the visible delete control.

## Architectural Decisions

- Keep `useStartChat` as the shared session creation hook.
- Add an option to skip recent-chat insertion for Builder-only entrypoints.
- Leave agent conversation starts unchanged.

## Tasks

- [x] Add sprint record and task checklist.
- [x] Add skip-recent-chat option to session start hook.
- [x] Use the option from the sidebar `New agent` CTA only.
- [x] Verify frontend lint/build and side effects.

## Risks

- Builder sessions will not appear in `CHATS` until explicitly inserted elsewhere.
- Existing stale localStorage rows are not auto-deleted to avoid surprising data removal.

## Verification Strategy

- Run frontend lint, TypeScript, and production build.
- Review call sites to confirm only the Builder CTA skips recent chat insertion.
