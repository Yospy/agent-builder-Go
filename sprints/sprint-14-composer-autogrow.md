# Sprint 14 — ChatGPT-Style Composer

## Scope

- Refine the chat composer so it starts as a compact pill, grows with input, then caps height and scrolls internally.
- Hide the textarea scrollbar chrome at the capped height.
- Improve the input surface spacing, line-height, send-button alignment, and focus affordance.
- Preserve keyboard submit behavior and Stop/Send state transitions.

## Assumptions

- Textarea-internal scrolling is acceptable after the composer reaches its max height; visible scrollbar chrome is not.
- The composer should match ChatGPT's compact-to-expanded behaviour without copying unsupported controls.
- The composer should feel like a polished command bar, not a raw form field.
- Motion should stay limited to state transitions and focus feedback.

## Architectural Decisions

- Keep the existing `Composer` component and shadcn `Textarea`.
- Measure textarea content height and clamp it to a max height.
- Keep scrolling native so keyboard and trackpad navigation still work.

## Tasks

- [x] Add sprint record and task checklist.
- [x] Implement clamped auto-grow with hidden scrollbar chrome.
- [x] Refine composer spacing, focus, and button alignment.
- [x] Verify lint, TypeScript, build, and side effects.

## Risks

- Hidden scrollbars reduce discoverability; textarea cursor movement and wheel/trackpad scroll still work.
- Max height must be tall enough for drafting without swallowing the page.

## Verification Strategy

- Run frontend lint, TypeScript, and production build.
- Review the composer for compact/expanded states, hidden scrollbar chrome, retained scroll behaviour, line-height, focus, and keyboard submit behaviour.
