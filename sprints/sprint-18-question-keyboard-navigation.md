# Sprint 18 — Builder Question Keyboard Navigation

## Scope

- Fix keyboard navigation and selection for active Builder question cards.
- Keep mouse behavior and answer submission contract unchanged.
- Preserve custom-answer behavior.

## Assumptions

- Builder questions are single-select choice sets.
- Arrow keys should move through choices and select the focused choice.
- Space or Enter on a choice should select it; the Continue button remains the submission action.

## Architectural Decisions

- Keep the behavior local to `BuilderQuestionCard`.
- Use an explicit roving focus model on the visible option rows instead of relying on clipped native radio inputs.
- Keep answer payloads unchanged: option answers use `optionId`, custom answers use `customText`.

## Tasks

1. [x] Add sprint record and task checklist.
2. [x] Investigate current keyboard behavior and identify root cause.
3. [x] Implement roving keyboard navigation and selection.
4. [x] Verify lint, TypeScript, build, and side effects.

## Completion Notes — 2026-06-14

- Root cause: visible option rows relied on clipped radio inputs for keyboard behavior, leaving the visual selection surface without reliable roving navigation.
- Replaced option rows with visible `role="radio"` buttons inside a `radiogroup`.
- Added ArrowUp/ArrowDown/ArrowLeft/ArrowRight/Home/End navigation and Space/Enter selection.
- Kept answer payloads, Continue submission, disabled state, and custom-answer textarea behavior unchanged.
- Verification clean: `npm run lint`, `npx tsc --noEmit`, and `PATH="$HOME/.nvm/versions/node/v20.19.6/bin:$PATH" npm run build`.

## Self-Review

- Minimal correct change: limited to the Builder question card plus sprint records.
- Architectural drift: none; behavior remains local to `BuilderQuestionCard`.
- Boundaries/invariants: answer API shape and run-state flow are unchanged.
- Staff-engineer check: explicit keyboard model is preferable to relying on hidden native controls for a custom card UI.

## Risks

- Custom answer focus could jump unexpectedly when selected by keyboard.
- Replacing native radio inputs could regress accessibility if ARIA state is incomplete.

## Verification Strategy

- Run `npm run lint` in `web/`.
- Run `npx tsc --noEmit` in `web/`.
- Run `npm run build` in `web/`.
- Review diff for answer payload, disabled state, custom answer, and focus side effects.
