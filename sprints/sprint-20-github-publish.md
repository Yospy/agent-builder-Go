# Sprint 20 - GitHub Publish Prep

## Scope

- Prepare this directory as its own Git repository, separate from the parent Desktop repo.
- Add root repository documentation for running, developing, testing, and understanding the app.
- Add ignore rules so local secrets, databases, logs, builds, and dependencies are not pushed.
- Commit and push to `https://github.com/Yospy/agent-builder-Go.git` on `main`.

## Assumptions

- The target GitHub repository is owned by the authenticated `gh` account `Yospy`.
- This directory should be the repo root; the parent Desktop git repository must not be used.
- Runtime state such as `.env`, SQLite DBs, logs, `node_modules`, and `.next` are local-only.

## Architectural Decisions

- Keep backend and frontend as separate subprojects under one repository:
  - `server/` Go backend, runtime, store, tools, tests.
  - `web/` Next.js frontend.
- Use root README as the operator entrypoint and keep existing detailed docs under `docs/`.
- Use the existing local dev topology: backend `:8080`, web `:3000`, Next rewrites API traffic to backend.

## Tasks

1. [x] Inspect current git state, remote, and local artifacts.
2. [ ] Add root `.gitignore`, `.env.example`, and detailed `README.md`.
3. [ ] Initialize this directory as its own git repo and review tracked files.
4. [ ] Run backend and frontend verification.
5. [ ] Commit on `main` and push to GitHub.

## Risks

- Accidentally pushing the parent Desktop repo or unrelated files.
- Accidentally committing `.env`, local SQLite DBs, or runtime logs.
- Target GitHub repo may already contain unrelated history.
- Build can fail under old Node versions; Node 20+ is required for Next 16.

## Verification Strategy

- `go test ./...` from `server/`.
- `npm run lint`, `npx tsc --noEmit`, and Node 20 `npm run build` from `web/`.
- `git status --short` from the nested repo before staging.
- `git ls-files` checks to ensure no secrets, DBs, logs, dependencies, or build outputs are tracked.
