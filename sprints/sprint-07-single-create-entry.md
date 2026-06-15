# Sprint 07 — Single Create Entry

## Scope

- Keep exactly one create-agent entry point in the visible desktop UI: sidebar
  `New agent`.
- Stop rendering the Builder as a normal agent row or registry card.
- Remove empty-state create buttons so the UI does not duplicate the same job.

## Assumptions

- Builder is a system command, not a user-created agent.
- Created agents should still appear in the sidebar and registry.
- Chat buttons on user-created agent cards are not create actions.

## Tasks

1. [x] Add sprint note.
2. [x] Filter Builder out of sidebar agent list.
3. [x] Filter Builder out of registry and empty state actions.
4. [x] Verify lint, TypeScript, build, and browser smoke.

## Verification

- `npm run lint`
- `npx tsc --noEmit`
- Production build with Node >=20.9.0
- Browser smoke confirming no Builder card/row and no duplicate create CTA.

## Completion Notes — 2026-06-13

- Kept the sidebar `New agent` button as the only create-agent control.
- Filtered the system Builder out of the sidebar agent list and main registry.
- Removed the empty-state create button and duplicate Builder wording.
- Verification clean: lint, TypeScript, production build, and browser smoke
  confirmed one `New agent` label, zero Builder cards, no console errors.
