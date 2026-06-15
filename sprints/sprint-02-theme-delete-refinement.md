# Sprint 02 — Theme toggle, delete, refinement pass

> Refs: `docs/08-delete-api-contract.md` (delete), `docs/07-ui-plan.md`
> (vocabulary + motion spec carry over). Monochrome-only rule still absolute.

## Scope

1. **Light/dark mode** — `next-themes`, class strategy. Dark tokens already
   exist in globals.css (zero-chroma, verified Phase 2). Toggle = icon button
   in the sidebar footer, Sun⇄Moon, `aria-label`, no flash (suppressHydrationWarning).
2. **Delete chats + built agents** — per the delete contract:
   - `api.deleteAgent(id)` / `api.deleteSession(id)` (204 = no body → new
     no-content request path in api.ts).
   - Agent card (home grid): hover/focus-revealed trash icon button →
     **inline confirmation** strip on the card ("…and all its chats") →
     DELETE. Never rendered for `id === "builder"`.
   - Sidebar chat row: hover/focus-revealed trash → row swaps to a compact
     inline confirm → DELETE. If it was the active chat, navigate to `/`.
   - Status handling: 204/404 → remove locally (+ recent-chats cleanup,
     AGENTS_CHANGED); 409 → toast "run in flight, retry after"; 400/500 → toast.
   - recent-chats.ts: `removeRecentChat(sessionId)`,
     `removeRecentChatsForAgent(agentId)`.
3. **Refinement pass** (structural precision, not decoration):
   - Shared `h-14` header rhythm: sidebar gets a brand header (h-14,
     border-b) and the agents page gets a matching page header — the
     border-b line runs continuously across sidebar + content.
   - Sidebar restructure: brand top / CTA + nav middle / utility footer
     (theme toggle) with border-t.
   - Composer: char counter behind progressive disclosure (hidden until 90%
     of limit), tightened spacing, gutters aligned with message column.
   - Consistent gutters (px-4 headers, px-3 sidebar body), boundary audit.

## Assumptions

- Backend delete endpoints are live on :8080 (verify with curl before UI work).
- No modal anywhere (locked decision) — confirmations stay inline.
- "Disable delete while running" is best-effort per contract; cross-component
  run state isn't tracked globally, so 409 toast is the safety net.

## Steps

1. [x] Verify backend DELETE endpoints exist and are covered by server tests.
2. [x] `next-themes` provider + toggle + suppressHydrationWarning; sonner theme.
3. [x] api.ts no-content requests + recent-chats removals.
4. [x] Agent card delete (inline confirm, stopPropagation vs. card click).
5. [x] Sidebar chat delete (inline confirm, active-chat navigation).
6. [x] Refinement: sidebar restructure, page headers, composer, boundary audit.
7. [x] Verify: tsc, eslint, next build; browser smoke for shell + theme toggle.
8. [ ] E2E vs live backend — delete test agent
   Testy2 (agent-71f7ad9402281a8f) through the real UI flow, delete a chat,
   409 path if reproducible, theme toggle both modes.

## Risks

- Hydration mismatch from theme class → suppressHydrationWarning + CSS-only
  icon swap (no mounted-state flicker).
- Card delete button inside a `role="button"` card → keyboard/click event
  leaks (stopPropagation + preventDefault on Enter/Space).
- Deleting active chat mid-run returns 409 (server lock) — surface toast,
  keep row.

## Verification

- tsc / eslint / next build clean.
- E2E against live backend: agent delete (204), chat delete (204), 404
  tolerated, builder shows no delete affordance, theme persists across reload.

## Completion Notes — 2026-06-12

- Implemented light/dark mode with semantic tokens, `next-themes`, sidebar
  footer toggle, and `suppressHydrationWarning`.
- Implemented no-body delete API calls, recent-chat removals, inline chat
  delete confirmation, and built-agent card delete confirmation. Builder
  remains protected in UI (`id === "builder"`) and backend.
- Refined structure: continuous `h-14` header rhythm, brand/CTA/nav/footer
  sidebar, page header, composer footer, aligned gutters, progressive character
  counter at 90%.
- Verification clean: `npm run lint`, `npx tsc --noEmit`, `npm run build`
  with Node 24 runtime, `go test ./...`.
- Browser smoke: existing dev server on `localhost:3000`; no console errors;
  theme toggle verified light and dark classes. Live delete E2E not run because
  the available dev session had no reachable backend data.

## Fix Notes — 2026-06-12

- Root cause for "delete not working": the process on `:8080` was a stale
  `go run` binary from before DELETE routes existed. Restarted it from current
  `server/` source; safe probes now return expected `400`/`404`, and a
  temporary chat deleted through `localhost:3000` with `204`.
- Replaced inline confirmations with anchored popovers. Vocabulary decision:
  interactive confirmation belongs in a **popover**, not a tooltip or badge.
- Agent cards are no longer clickable containers with nested destructive
  controls. They expose explicit `Chat` and `Delete` actions.
- Fixed persistent sidebar disabled state after starting a chat by resetting
  `startingAgentId` on successful navigation.
