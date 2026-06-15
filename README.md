# Agent Builder Go

Agent Builder is a local full-stack app for creating, editing, and running custom AI agents. Each agent is stored as a SQLite row, and all agents run through one shared Go engine. The frontend is a ChatGPT-style Next.js UI; the backend is a single Go service with SSE streaming, tool execution, confirmation gates, and persistent chat history.

## What It Does

- Chat with a seeded **Agent Builder** to create new agents.
- Store every agent in `agent_specs` with name, persona, instructions, model, tools, and sources.
- Start normal chats with any created agent.
- Open **Edit Agent** chats under each agent to change prompts, tools, sources, and model settings.
- Keep edit chats separate from normal chats in the sidebar.
- Stream model output and tool activity over SSE.
- Require approval before consequential tools such as `create_agent`, `update_agent`, and `write_file`.
- Persist chat history in SQLite and write append-only JSONL runtime logs locally.

## Repository Layout

```txt
.
├── server/      Go backend, engine, runtime, store, tools, tests
├── web/         Next.js frontend
├── docs/        Architecture, data model, lifecycle, UI, security notes
├── sprints/     Implementation sprint records
└── tasks/       Sprint task checklist and working notes
```

## Architecture

```txt
Browser UI (:3000)
  |
  | HTTP + SSE
  v
Go backend (:8080)
  runtime/  loads sessions, streams events, persists results
  engine/   shared agent loop, stateless and DB-free
  tools/    capability and platform tools
  store/    SQLite live state
  logs/     local JSONL audit trail
  |
  v
OpenAI API
```

Core invariant: the runtime touches HTTP, DB, files, network, and secrets; the engine only receives dependencies and runs the loop.

## Requirements

- Go 1.25+
- Node.js 20+
- npm
- OpenAI API key

The app was verified with:

- Go tests via `go test ./...`
- Next 16 frontend build with Node 20

## Environment Setup

Create your local environment file:

```bash
cp .env.example .env
```

Set at least:

```bash
OPENAI_API_KEY=sk-...
OPENAI_MODEL=gpt-5.1
```

Useful backend settings:

```bash
ADDR=:8080
DB_PATH=agent-builder.db
LOGS_DIR=logs
WORK_DIR=workspace
OPENAI_TIMEOUT_SECONDS=120
```

Optional tracing:

```bash
BRAINTRUST_API_KEY=...
BRAINTRUST_PROJECT=agent-builder
```

## Run Locally

Install frontend dependencies:

```bash
cd web
npm install
```

Start the backend:

```bash
cd ../server
go run .
```

In another terminal, start the frontend:

```bash
cd web
npm run dev
```

Open:

```txt
http://localhost:3000
```

The Next app proxies API calls to the Go backend at `http://localhost:8080`.

## How To Use The App

1. Click **New agent** in the sidebar.
2. Describe the agent you want.
3. Answer any guided setup questions.
4. Approve the `create_agent` confirmation.
5. The new agent appears in the agent registry.
6. Click **Chat** on an agent card to start a normal chat.
7. Click **Edit Agent** to start a new edit chat for that agent.
8. In an edit chat, ask for changes like:

```txt
Change the system prompt to be more concise.
Add https://example.com/docs as a source.
Give this agent calculator and fetch_url access.
```

Edit chats appear under their agent in the sidebar. Normal chats appear under global **Chats**.

## Data Model

SQLite stores live state:

- `agent_specs`: one row per agent.
- `sessions`: normal chats and edit chats. `kind` is `normal` or `agent_edit`.
- `agent_spec_versions`: snapshots written when an edit chat applies an agent update.

Local runtime artifacts are intentionally ignored by git:

- `.env`
- `server/agent-builder.db`
- `server/logs/`
- `server/workspace/`
- `web/node_modules/`
- `web/.next/`

## API Overview

Key endpoints:

```txt
GET    /api/agents
GET    /api/agents/:id
DELETE /api/agents/:id

POST   /api/sessions
GET    /api/sessions/:id
PATCH  /api/sessions/:id
DELETE /api/sessions/:id
POST   /api/sessions/:id/run
POST   /api/sessions/:id/approve
POST   /api/sessions/:id/answer

POST   /api/agents/:id/edit-sessions
GET    /api/agent-edit-sessions
GET    /api/agents/:id/versions
POST   /api/chat-title
```

`POST /api/sessions/:id/run` streams SSE events:

- `status`
- `llm_text`
- `tool_use`
- `tool_result`
- `confirm`
- `user_question`
- `done`
- `error`
- `aborted`

## Tools

Platform tools:

- `list_tools`
- `list_agents`
- `ask_user_question`
- `create_agent`
- `update_agent`

Capability tools:

- `calculator`
- `fetch_url`
- `read_file`
- `write_file`

Consequential tools require approval before they run.

## Verification

Backend:

```bash
cd server
go test ./...
```

Frontend:

```bash
cd web
npm run lint
npx tsc --noEmit
npm run build
```

If `npm run build` fails with a Node version error, use Node 20+.

## Development Notes

- Backend logs are local JSONL files under `server/logs/`.
- The local SQLite DB is created at `server/agent-builder.db` by default.
- The seeded Builder agent has id `builder`.
- Normal agents are invoked through sessions bound to their `agent_id`.
- Edit sessions use a synthetic Agent Editor spec and can only update their own target agent.
- `update_agent` writes both the live agent row and a version snapshot.

## Docs

Start here for deeper design context:

- `docs/00-CONTEXT.md`
- `docs/02-architecture.md`
- `docs/03-data-model.md`
- `docs/04-request-lifecycle.md`
- `docs/05-security.md`
- `docs/07-ui-plan.md`
