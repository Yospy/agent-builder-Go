# Lessons

- **shadcn CLI ≥3 changed `-b`**: it now means component library (`radix`|`base`),
  not base color. Non-interactive init needs `-p <preset>` (e.g. `-p nova`);
  base color comes from the preset. Check `--help` before scripted init.
- **Next 16 ships `react-hooks/set-state-in-effect` as an error.** Patterns that
  pass: retry-counter effects (`[attempt]` dep, setState only in async
  callbacks), `useSyncExternalStore` for browser-storage-backed lists, and the
  "adjust state during render" idiom for param-keyed resets. Don't call a
  loader that setStates synchronously from an effect body.
- **SSE-over-fetch state machines need an unmount abort.** If the server holds
  a per-session lock for in-flight runs, a component unmounting without
  `AbortController.abort()` strands the lock — deadlock if the stream was
  paused on a human-approval gate.
- **When a protocol has a terminal event carrying the full payload (`done.text`),
  treat it as authoritative**: replace trailing streamed text, append when the
  stream tail is a non-text item. "Only use it if nothing streamed" silently
  drops the final answer in tool-use turns.
