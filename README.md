# Arda Web Gateway

A planning-first project to build a safe, observable web gateway for Arda MUD.

The gateway connects a modern browser terminal to the existing MUD Telnet server (`86.110.194.3:7000`) and layers in AI-assisted action suggestions with strict safety controls.

## Project Status
This repository is currently in an extended **planning-first phase**.

Implementation is intentionally delayed until:
- Scope is stable.
- API contracts are defined.
- Milestones and testing strategy are explicit.

Phase 0 bootstrap is now completed to unblock implementation:
- Backend service skeleton with `GET /api/v0/health`.
- Frontend React/Vite scaffold with terminal-first placeholder layout.
- OpenAPI v0 file and Orval configuration.
- Docker Compose local parity baseline.
- Parser fixture directory seeded for golden test corpus.

Milestone 1 progress in-flight:
- M1-02/M1-03 shipped: WS baseline + command queue/rate limiting with tests.
- M1-04 baseline shipped: real Telnet TCP bridge (upstream read/write loop) with disconnect drop semantics.
- M1-08 baseline shipped: frontend wired to `/api/v0/ws/terminal` with queue metadata and `QUEUE_FULL` inline + toast UX.
- M1-15 groundwork shipped: metrics include send-latency histogram and dropped-unsent counters per session.

## Goals
- Deliver an immersive terminal-first MUD web client.
- Keep transport reliable (WebSocket <-> Telnet) under rapid user input.
- Correct CP1251 text decoding and SMAUG `Я` artifact behavior.
- Add practical AI assistance (suggestions first, autonomy later with safety guards).
- Maintain observability and predictable operations from day one.

## Non-Goals (MVP)
- Full autonomous gameplay loop.
- Heavy visual UI overlays that reduce terminal immersion.
- Advanced analytics dashboards.

## Planned Architecture
- Frontend: React + Tailwind + shadcn/ui.
- Backend: Go service managing Telnet session, parser pipeline, and AI orchestration.
- Realtime transport: WebSocket between frontend and backend.
- Upstream protocol: Telnet TCP to Arda MUD with explicit negotiation handling and prompt-without-newline support.
- State store: SQLite for parsed state and event persistence.
- SQLite runtime policy: WAL mode + controlled writes for predictable concurrency.
- AI provider: OpenRouter with strict JSON decision contracts.
- API contract: OpenAPI v0 as source of truth.
- Client/docs generation: Orval from OpenAPI.

## AI Modes (Planned)
1. Auto-Suggest Mode (MVP): context-aware action suggestions on new text updates with debounce and stale-response suppression.
2. Background LLM Mode: periodic state/map analysis without suggestion spam.
3. Autopilot Mode: goal-driven execution loop with explicit safety boundaries.
4. Macro System: replay successful command chains without extra LLM calls.

## Documentation Map
- Detailed iterative planning: `plan.md`
- High-level milestones and phase outcomes: `docs/roadmap.md`
- Agent conventions and coding behavior: `AGENTS.md`
- Phase task files for AI execution: `plan/plan-*.md`

Consistency rule:
- `plan.md` holds detailed operational defaults and task slicing.
- `docs/roadmap.md` stays high-level and phase-oriented.
- `README.md` remains onboarding-focused.
- When scope or milestone sequencing changes, update all three in the same pass.

## Testing Strategy (Current)
- Balanced approach:
  - Unit tests for decoder/parser/queue behavior.
  - Integration tests for WebSocket gateway flow with simulated Telnet upstream.
  - Golden parser fixtures for CP1251/ANSI/Telnet negotiation edge cases.

## MVP Operational Defaults (Current)
- Queue: 500ms send interval, max depth 20, reject new when full.
- Reconnect policy: no automatic replay of unsent queued commands after reconnect.
- Suggestion freshness: short debounce window and stale-response discard.

## Milestone Outline
- M1: Playable Gateway + Suggestions.
- M2: Reliable State + Background LLM.
- M3: Autopilot (Safety-First).
- M4: Macro System.

See `docs/roadmap.md` for high-level outcomes and `plan.md` for concrete defaults, acceptance criteria, and task slicing.

## Developer Learning Context
This project is also a Go learning journey.

Agent work in this repo should:
- explain Go idioms and concurrency decisions,
- avoid black-box edits,
- highlight reusable “aha” patterns after coding tasks.

## Toolchain Baseline
- Go: `1.22.x`
- Node.js: `22.x`
- npm: `10.x` (bundled with Node 22)
- Docker Engine + Docker Compose v2

## Local Setup
1. Copy environment defaults (already committed for bootstrap): `.env`
2. Backend check:
   - `cd backend && go test ./...`
3. Frontend install:
   - `cd frontend && npm install`
4. Generate API client:
   - `cd frontend && npm run api:generate`
5. Run local stack:
   - Backend: `make run-backend`
   - Frontend: `make dev-frontend`
   - Docker parity: `make docker-up`

## Repository Layout (Bootstrap)
- `backend/`: Go service and testdata fixtures.
- `frontend/`: React/Vite UI shell + Orval generation config.
- `api/openapi/`: OpenAPI source of truth (`gateway.v0.yaml`).
- `plan/`: Phase-by-phase execution plans for AI agents.
- `infra/docker/`: Docker orchestration notes and future overrides.
- `scripts/`: bootstrap and automation helpers.
