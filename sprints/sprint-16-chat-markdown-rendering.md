# Sprint 16 — Chat Markdown Rendering

## Scope

- Render assistant text as Markdown in chat messages.
- Keep user messages and tool JSON/pre blocks unchanged.
- Support the Markdown subset currently emitted by Builder and agents.

## Assumptions

- Agent output is trusted only as text; rendering must not use raw HTML injection.
- Adding a dependency is unnecessary for the current subset and would add network/setup risk.
- The main visible issue is assistant `text` items showing raw `**bold**`, bullets, and inline code.

## Architectural Decisions

- Add a local React Markdown subset renderer with escaped text by default.
- Render links as normal anchors with safe `target`/`rel`.
- Preserve streaming cursor behavior outside the Markdown renderer.

## Tasks

1. Add sprint record and task checklist.
2. Add a safe Markdown subset renderer for assistant messages.
3. Wire assistant text items to use the renderer.
4. Verify lint, TypeScript, build, and UI rendering.

## Risks

- A small renderer will not cover every Markdown edge case.
- Streaming partial Markdown may temporarily render as plain text until closing syntax arrives.

## Verification Strategy

- Run `npm run lint`, `npx tsc --noEmit`, and `npm run build`.
- Open the current Builder chat and verify bold/list/code Markdown renders instead of raw syntax.
