# Sprint 04 — UI Uniformity Pass

## Scope

- Rebuild the agent index as a compact operational workspace, not a loose card
  gallery.
- Standardise spacing on an 8px rhythm: shared page gutters, row heights,
  card padding, action bars, and popover spacing.
- Make agent cards uniform height with explicit header/body/tools/actions zones.
- Align sidebar brand, CTA, navigation rows, chat rows, and footer.
- Keep destructive confirmation in popovers; do not misuse badges for actions.

## Assumptions

- The product is an agent operations surface: dense, calm, scannable.
- Monochrome tokens remain the project visual language.
- Builder remains non-deletable.

## Tasks

1. [x] Tighten page shell and header metrics.
2. [x] Rebuild agent card structure with equal heights and consistent action rail.
3. [x] Normalize sidebar row heights, gutters, and delete affordances.
4. [x] Refine delete popover size, copy hierarchy, and action alignment.
5. [x] Verify lint, TypeScript, build, and browser smoke.

## Risks

- Over-densifying the UI can bury the primary action. Keep Chat prominent.
- Popovers inside row/card action areas must not trigger navigation.

## Verification

- Visual smoke on `localhost:3000`.
- `npm run lint`, `npx tsc --noEmit`, `npm run build`.

## Completion Notes — 2026-06-12

- Reworked the agents page into a wider operational workspace (`max-w-5xl`)
  with a consistent 3-column desktop grid.
- Agent cards now have equal measured dimensions at 1280px viewport
  (`307x242`), explicit header/body/tools/action zones, and stable footer
  actions.
- Sidebar is now 288px wide with matching 56px top/footer bands and 32px nav
  rows.
- Delete popovers use a 304px rendered overlay with separated Cancel/Delete
  footer actions.
- Verification clean: `npm run lint`, `npx tsc --noEmit`, `npm run build`.

## Screenshot Fix — 2026-06-12

- Fixed builder chat header dark-mode layering. The white band came from using
  `foreground` as a background in dark mode; header now uses the app background
  surface so it blends with the chat body.
- Recast header tool labels as muted capability tags, not bright badges:
  vocabulary distinction is that tags categorise content, while badges are
  attached informational labels.
- Verified dark-mode metrics in browser: header dark surface, sidebar dark
  surface, tool tags muted with low-contrast border.
