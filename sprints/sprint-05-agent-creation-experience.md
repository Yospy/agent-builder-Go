# Sprint 05 — Agent Creation Experience

## Scope

- Reframe the agents index from a generic card gallery into a professional
  agent assembly workspace.
- Make the Builder the primary creation surface with a distinct creation rail,
  suggested agent briefs, and workflow signals.
- Upgrade agent cards to show role, model, tool coverage, and action affordance
  with clearer hierarchy.
- Keep existing backend contracts, chat start behavior, delete behavior, and
  sidebar navigation unchanged.

## Assumptions

- Users create agents through the Builder chat; the index should accelerate that
  intent without introducing a new create API.
- The experience should feel memorable through layout, density, colour tokens,
  and interaction polish, not decorative excess.
- Vocabulary guidance applies directly: use hierarchy, progressive disclosure,
  tags for categorisation, visible affordance, stable skeletons, and accessible
  focus states.

## Architectural Decisions

- Keep the route as a client page because it uses effects, state, browser
  events, and chat-start actions.
- Add local presentational helpers inside `web/app/page.tsx` for page-specific
  composition rather than creating shared abstractions prematurely.
- Preserve `AgentCard` as the reusable agent summary component, but enrich its
  visual language and metadata treatment.

## Tasks

1. [x] Add sprint record and task checklist.
2. [x] Redesign index header, creation rail, templates, and state surfaces.
3. [x] Rebuild agent cards with stronger hierarchy, tags, and action rail.
4. [x] Review responsive behavior, accessibility, and side effects.
5. [x] Run lint, TypeScript, build, and local browser smoke.

## Risks

- A richer page could obscure the single real creation action. Keep Builder CTA
  and chat action visually dominant.
- More colour could drift from the restrained product tone. Use semantic tokens
  and muted accents, not a one-note palette.
- Page-only helper components can grow too large. Keep them tightly scoped.

## Verification Strategy

- Run `npm run lint`, `npx tsc --noEmit`, and `npm run build` in `web/`.
- Smoke the index route in browser at desktop and mobile widths.
- Review diff for backend contract preservation, delete behavior, and
  responsive text overflow.

## Completion Notes — 2026-06-12

- Rebuilt the index as an agent workshop with a Builder creation rail,
  blueprint presets, build-step signposts, and local capability telemetry.
- Reworked agent cards into spec tiles with role/model hierarchy, persona/tool
  readiness signals, tool tags, and a stronger Chat action.
- Updated the sidebar brand treatment and made the sidebar desktop-only so the
  workshop does not squeeze on mobile.
- Preserved API contracts and existing chat/delete behavior; all creation
  affordances still route through Builder chat.
- Verification clean: `npm run lint`, `npx tsc --noEmit`, and production build
  with Node >=20.9.0. Browser smoke passed at 1280x720 and 390x844 with no
  horizontal overflow and no console errors.
