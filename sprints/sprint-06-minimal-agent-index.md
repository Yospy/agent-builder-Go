# Sprint 06 — Minimal Agent Index

## Scope

- Remove duplicate Builder actions from the page header and oversized creation
  rail.
- Keep the index focused on the registry and agent cards.
- Simplify card metadata so users can scan name, model, persona, tools, and
  Chat.

## Assumptions

- The sidebar `New agent` action is enough for the normal create path.
- Empty state should still offer a create action because there is no other
  content yet.
- Minimal means less hierarchy and fewer panels, not removing core controls.

## Tasks

1. [x] Add sprint note.
2. [x] Simplify index layout and remove top-right Builder CTA.
3. [x] Simplify agent card chrome.
4. [x] Verify lint/build and side effects.

## Verification

- Run `npm run lint`, `npx tsc --noEmit`, and production build.
- Browser smoke for no overflow and no console errors.

## Completion Notes — 2026-06-13

- Removed the page header Builder CTA and deleted the oversized creation rail,
  blueprint presets, and capability map.
- Reduced the registry to a compact header plus card grid.
- Simplified cards back to name, model, persona, tool tags, Chat, and delete.
- Verification clean: lint, TypeScript, production build, and browser smoke at
  1280x720 and 390x844 with no horizontal overflow or console errors.
