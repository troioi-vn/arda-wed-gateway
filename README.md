# Arda Web Gateway

A planning-first project to build a safe, observable web gateway for Arda MUD.

The gateway connects a modern browser terminal to the existing MUD Telnet server (`86.110.194.3:7000`) and layers in AI-assisted action suggestions with strict safety controls.

## Project Status
This repository is currently in an extended **planning-first phase**.

Implementation is intentionally delayed until:
- Scope is stable.
- API contracts are defined.
- Milestones and testing strategy are explicit.

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
- Upstream protocol: Telnet TCP to Arda MUD.
- State store: SQLite for parsed state and event persistence.
- AI provider: OpenRouter with strict JSON decision contracts.
- API contract: OpenAPI v0 as source of truth.
- Client/docs generation: Orval from OpenAPI.

## AI Modes (Planned)
1. Auto-Suggest Mode (MVP): context-aware action suggestions on new text updates.
2. Background LLM Mode: periodic state/map analysis without suggestion spam.
3. Autopilot Mode: goal-driven execution loop with explicit safety boundaries.
4. Macro System: replay successful command chains without extra LLM calls.

## Documentation Map
- Detailed iterative planning: `plan.md`
- High-level milestones and phase outcomes: `docs/roadmap.md`
- Agent conventions and coding behavior: `AGENTS.md`

Consistency rule:
- `plan.md` holds detailed operational defaults and task slicing.
- `docs/roadmap.md` stays high-level and phase-oriented.
- `README.md` remains onboarding-focused.
- When scope or milestone sequencing changes, update all three in the same pass.

## Testing Strategy (Current)
- Balanced approach:
  - Unit tests for decoder/parser/queue behavior.
  - Integration tests for WebSocket gateway flow.
  - Golden parser fixtures for CP1251/ANSI edge cases.

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
