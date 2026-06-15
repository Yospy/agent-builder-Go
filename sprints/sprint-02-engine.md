# Sprint 02 — Phase 1: Engine + Brain + Registry (Go backend)

## Scope
Build the agent loop, naked, under `server/` (module `agent-builder`). Prove it with `go test`
(no CLI, no DB, no HTTP). Per `docs/07-implementation-plan.md` Phase 1.

## Deliverables
- `brain/` — `Brain` interface, `Step`/`Message`/`ToolCall`/`ToolDef` types, `FakeBrain`, `OpenAIBrain`
- `tools/` — `Registry`, `Tool`, `calculator`, `fetch_url`, `safehttp`
- `engine/` — `engine.Run(ctx, Input) (Output, error)`: compose prompt → resolve tools → loop → emit events
- `logx/` — slog setup (level from `LOG_LEVEL`)
- tests: engine loop + edge cases, safehttp blocked-IP table, calculator

## Edge cases wired (00-CONTEXT §11)
- runaway loop → cap 10 → error, stop
- tool exec error → `tool_result{ok:false}`, loop continues (never crash)
- malformed tool args → tool returns error → `tool_result{ok:false}`
- ctx cancel checked between brain/tool steps → `aborted`

## Verify (🔒)
- `go build ./...` clean
- `go test ./...` green: FakeBrain drives tool call→result→finish; cap/tool-error/malformed tests; safehttp rejects 169.254/127/10.x; calculator correctness
- optional `LIVE_OPENAI=1 go test ./brain/...` real gpt-5.1 smoke

## Risks
- OpenAI Go SDK API surface — verify against docs before writing `openai.go` (don't guess).
- Phase-1 simplification: `Brain.Next` returns a complete Step (OpenAIBrain streams internally to
  avoid timeouts); token-level UI streaming is Phase 3. Engine still emits the event stream.

## Logging (high-signal only)
engine start/done (invocation_id, tool_count, iters, duration) · each tool exec (name, ok,
duration_ms) · warns on tool error / loop cap / cancel. No per-token noise.
