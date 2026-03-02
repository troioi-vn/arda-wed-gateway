# Arda Web Gateway Roadmap

High-level roadmap for delivering a safe, immersive, and observable MUD web gateway with AI-assisted gameplay support.

## Vision
Create a terminal-first browser experience for Arda MUD that preserves immersion while adding reliable AI assistance through explicit safety and control boundaries.

## Product Principles
- Immersion first: terminal is primary, supporting panels are secondary.
- Safety first: command flood prevention, strict parsing contracts, explicit kill switches.
- Contract first: OpenAPI as source of truth, Orval-generated client and docs.
- Learning first: Go implementations should remain understandable and educational.

## Roadmap Phases

## Phase 0 — Planning and Contract Baseline
Outcome:
- Scope, milestones, and test strategy are explicit.
- OpenAPI v0 endpoint surface and error model are defined.
- Orval generation workflow is documented.
- MVP operational defaults are explicit (queue policy, reconnect policy, suggestion freshness policy).
- Telnet protocol handling constraints are explicit (negotiation filtering + prompt-without-newline behavior).

Exit criteria:
- `plan.md` approved for MVP start.
- `README.md` and this roadmap reflect current decisions.
- Early spike is complete: short real upstream transcript captured and simulator fixture path defined.

## Phase 1 — Playable Gateway + Suggestions (MVP)
Outcome:
- Browser terminal connected to MUD through Go gateway.
- Queue/rate limiting enforced (strict defaults).
- CP1251 decoding and SMAUG artifact correction working.
- Telnet negotiation filtering and prompt-without-newline handling are stable.
- Auto-Suggest actions available from strict JSON model output.
- Suggestion freshness controls prevent stale/flickering actions (debounce + stale-response discard).
- SQLite runtime defaults support concurrent read/write usage (WAL + controlled writes).

Exit criteria:
- Stable connect/send/receive loop in browser.
- Queue flood protection verified.
- Suggestion buttons render and execute commands.
- Reconnect behavior is explicit and predictable (no hidden command replay in MVP).

## Phase 2 — Reliable State + Background LLM
Outcome:
- Parser-to-SQLite state pipeline stabilized.
- In-memory context generation (`character.md`, `map.md` strings) in use.
- Background LLM Mode updates analysis without UI spam.

Exit criteria:
- State snapshot APIs are reliable.
- Background processing does not degrade terminal responsiveness.

## Phase 3 — Autopilot (Safety-First)
Outcome:
- Goal-driven execution loop is available.
- Loop detection, low-HP safety policy, and immediate pause/kill control implemented.

Exit criteria:
- Autopilot performs bounded tasks and halts safely on risk conditions.

## Phase 4 — Macro System
Outcome:
- Reusable command chains can be saved and replayed from UI.
- Reduces redundant LLM calls for repeated play patterns.

Exit criteria:
- Macro creation and replay are predictable and transparent to user.

## Cross-Cutting Streams
- Observability: structured logs + core metrics from early stages.
- Testing: balanced unit + integration strategy with parser golden fixtures, Telnet negotiation traces, and simulator-backed flow tests.
- Developer Experience: OpenAPI + Orval workflow, reproducible local + Docker parity.

## Out of Scope for MVP
- Full multi-user production architecture.
- Heavy UI polish beyond immersion-focused terminal workflow.
- Advanced analytics platform.

## How to use this roadmap
- Keep this file strategic and high-level.
- Put detailed decisions and defaults in `plan.md`.
- Keep milestone names aligned with `plan.md` and `README.md`.
- Update all three docs in the same pass when scope or sequencing changes.
