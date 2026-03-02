# Arda Web Gateway — Project Plan (Iteration 4)

Date: 2026-03-02
Planning style: iterative, high-detail, capability-based milestones

## 1) Confirmed Direction (from discussion)

1. Milestone 1 scope: **Terminal + Suggestions** (no autopilot yet).
2. Test strategy (early): **Balanced**.
   - Unit tests for parser and command queue/rate limiter.
   - Integration tests for WebSocket broadcast/connect/send/receive flow.
3. Milestone structure: **Capability milestones** (each one should be shippable).
4. Autopilot timing: after parser/state pipeline is stable and safety guards are proven.

## Iteration 2 decisions
5. User architecture now: **Single-user first**, with extension points for future multi-user sessions.
6. Initial command queue defaults: **Strict profile**.
   - Send delay: **500ms**
   - Max queue size: **20**
   - Overflow behavior: **reject new command when full**
7. Prompt budget policy: **Wide budget** with deterministic truncation when needed.
8. SQLite strategy: **Minimal core schema** first (rooms, exits, character_stats, inventory, events).
9. Observability baseline: **structured logs + metrics endpoint**.
10. Delivery flow: **Hybrid** (local development with maintained Docker Compose parity).

## Iteration 3 decisions
11. OpenAPI v0 is finalized for MVP HTTP surface (health, session, queue, suggestions, state).
12. Error model is standardized across endpoints with stable `error.code` values.
13. Orval generation layout and regeneration command are fixed for repeatable frontend sync.
14. Queue-full UX is explicit (frontend inline status + toast, no silent drops).
15. Parser golden fixture baseline is defined for CP1251/ANSI and extraction edge cases.
16. Milestone 1 is sliced into ordered implementation issues.

## Iteration 4 decisions
17. Telnet handling is explicit: support IAC negotiation filtering and record negotiation traces in fixtures.
18. Prompt detection must handle non-newline-terminated prompts for responsive suggestions and UI updates.
19. SQLite runtime defaults to WAL mode with controlled write serialization to avoid lock contention.
20. Suggestion pipeline applies debounce + stale-response discard to avoid UI flicker and outdated actions.
21. Queue reconnect policy is explicit for MVP: preserve in-memory queue only while connected; on disconnect, fail pending sends and require manual re-enqueue after reconnect.
22. Early spike is mandatory before full bridge hardening: capture real upstream transcript and build a Telnet simulator harness.
23. Arda domain baseline is now explicit from `docs/arda.md` (prompt pattern, aura/status tags, equipment vocabulary, combat condition phrases).

## Iteration 5 decisions
24. Phase 0 bootstrap is mandatory before Milestone 1 feature coding.
25. Toolchain baseline is pinned for onboarding consistency: Go 1.22.x, Node 22.x, Docker Compose v2.
26. Repository scaffolding is standardized: `backend/`, `frontend/`, `api/openapi/`, `plan/`, `infra/docker/`, `scripts/`.
27. Golden fixture file tree is created up front under `backend/testdata/parser/` and expanded continuously as transcripts are captured.
28. Developer workflow is normalized around `Makefile` task entrypoints for run/test/generate/compose.
29. WebSocket runtime contract now includes explicit `session.status` bootstrap event for frontend hydration.
30. Metrics baseline now includes queue send latency histogram and dropped-unsent per-session counter labels.

---

## 2) Product Goals (precise)

### Primary Goal
Deliver a stable web gateway that feels like a classic MUD client while safely adding AI-assisted action suggestions.

### Success Criteria (MVP)
- User can connect to `86.110.194.3:7000` from browser terminal via backend proxy.
- Input/output loop is responsive and robust under rapid user input.
- CP1251 decoding works, including SMAUG lowercase artifact correction (`Я` -> `я` in expected contexts).
- Suggestion buttons appear from strict JSON LLM output and send commands back to MUD.
- No autopilot shipped in MVP.

### Non-Goals (for MVP)
- Full autonomous gameplay loop.
- Advanced map visualization polish.
- Complex analytics dashboards.

---

## 3) Technical Scope Breakdown

## A. Transport + Session Core
- Go backend maintains Telnet TCP session(s).
- Frontend connects through WebSocket.
- Backend fan-out of MUD output to relevant frontend session.
- Session lifecycle: connect, reconnect, close, and cleanup.
- Telnet negotiation handling is explicit (`IAC`, `WILL/WONT`, `DO/DONT`, subnegotiation) with deterministic filtering.
- Prompt/event detection must work for lines that do not terminate with `\n` (classic MUD prompt style).

## B. Text Processing + Parsing
- ANSI stream handling.
- CP1251 -> UTF-8 conversion.
- SMAUG artifact correction pass.
- Basic structured extraction (location, hp/mana/move, inventory snippets) to support LLM context.
- Prompt parsing baseline: support `HP/MA/MV/EXP` style numeric prompt tuples (example family: `(hp/hp ma/ma mv/mv exp |)`).
- Status token parsing baseline: recognize common Russian state prefixes like `(Белая Аура)`, `(СераЯ Аура)`, `(Красная Аура)`, `(В полете)`, `(Плавает)`, `(Светится)`, `(Волшебное)` without corrupting entity names.
- Inventory/equipment parsing baseline: map known slot vocabulary (head/neck/body/fingers/arms/shoulders/legs/wrist/shield/wield/held) into normalized state fields.

## C. Command Queue + Safety
- Buffer rapid inputs.
- Apply send interval/rate limit to avoid server flood.
- Preserve order guarantees.
- Provide queue visibility in logs/metrics.
- Initial defaults (Iteration 2): 500ms interval, queue max 20, reject-new on overflow.
- Reconnect semantics (Iteration 4): no automatic replay across reconnect boundaries in MVP.

## D. Suggestion Engine (Auto-Suggest only in MVP)
- Trigger on new text updates.
- Apply short debounce window before dispatching LLM calls to reduce churn during high-frequency output.
- Build prompt from recent lines + parsed state + generated character/map context strings.
- Prompt policy: wide context budget with deterministic truncation order when cap is reached.
- If a newer prompt job supersedes an older one, discard stale response on arrival.
- Add lightweight play-etiquette guardrails to prompt policy (avoid recommending obvious KS/loot-steal/spam behaviors unless user explicitly requests).
- Require strict JSON contract:
  - `commands[]`
  - `reason`
  - `expected_outcome`
- Render suggestions as clickable UI actions.

## E. State Store
- SQLite-backed state updates from parser events.
- Enable SQLite WAL mode and bounded busy-timeout defaults for concurrent read/write resilience.
- Use a controlled write path (single writer worker or transactional queue) to avoid lock contention.
- Generate `character.md` and `map.md` as in-memory strings for LLM calls.
- Initial schema (Iteration 2): `rooms`, `exits`, `character_stats`, `inventory`, `events`.

## F. API Contract + Generated Client/Docs
- Maintain backend HTTP/WS-adjacent API contract in OpenAPI specification.
- Use Orval to generate frontend API client from OpenAPI.
- Generate API documentation artifacts from the same OpenAPI source.
- Keep OpenAPI as the single source of truth to prevent backend/frontend drift.

## G. OpenAPI v0 Scope (Iteration 3)

OpenAPI file path target:
- `api/openapi/gateway.v0.yaml`

HTTP endpoints in v0:
1. `GET /api/v0/health`
   - Returns service status and build metadata.
2. `POST /api/v0/session/connect`
   - Starts gateway-to-MUD session.
   - Body includes host, port, optional credential alias (not raw password in logs).
3. `POST /api/v0/session/disconnect`
   - Graceful session shutdown and cleanup.
4. `GET /api/v0/session/status`
   - Returns connected/disconnected, queue depth, last activity timestamp.
5. `GET /api/v0/state/snapshot`
   - Returns parser-derived state projection used by UI and suggestion prompt builder.
6. `GET /api/v0/suggestions/latest`
   - Returns latest suggestion set (`commands`, `reason`, `expected_outcome`) and timestamp.
7. `POST /api/v0/commands/enqueue`
   - Enqueues user command; returns accepted/rejected + queue metadata.

Standardized response envelopes:
- Success:
  - `{ "data": <payload>, "meta": { "request_id": "...", "timestamp": "..." } }`
- Error:
  - `{ "error": { "code": "QUEUE_FULL", "message": "...", "details": {} }, "meta": { "request_id": "...", "timestamp": "..." } }`

Stable error codes in v0:
- `INVALID_REQUEST`
- `SESSION_NOT_CONNECTED`
- `SESSION_ALREADY_CONNECTED`
- `QUEUE_FULL`
- `UPSTREAM_UNAVAILABLE`
- `INTERNAL_ERROR`

WebSocket note:
- WebSocket runtime messaging remains primary for terminal stream and real-time events.
- OpenAPI v0 documents HTTP endpoints and shared schemas used by both HTTP and WS message payload definitions.

---

## 4) Testing Strategy (Balanced)

## Unit Tests (Go)
1. Decoder tests:
   - CP1251 conversion cases (including mixed ANSI/control bytes).
   - SMAUG artifact correction cases and non-regression samples.
   - Telnet negotiation/control-sequence interleaving samples.
2. Parser tests:
   - Location/status/inventory extraction from representative log lines.
   - Prompt detection for non-newline-terminated prompt endings.
   - HP/MA/MV/EXP prompt tuple variants.
   - Aura/status token extraction and normalization for Russian tag prefixes.
   - Equipment slot extraction variants (multi-line and reordered outputs).
3. Queue tests:
   - Burst input (10 rapid commands), order preservation, and interval enforcement.
   - Disconnect/reconnect semantics (no hidden replay in MVP).
4. Suggestion contract tests:
   - Malformed/partial JSON output is rejected safely.
   - Stale response suppression when newer suggestion cycle exists.

## Integration Tests
1. WebSocket channel behavior:
   - Client connect/disconnect.
   - Broadcast from Telnet reader to WS client.
   - Input from WS client to Telnet writer path.
2. Gateway flow smoke:
   - Simulated Telnet upstream + real WS client in test harness.
   - Include negotiation, fragmented ANSI, and prompt-without-newline fixtures.

## Exit Criteria per Milestone
- No failing tests in changed component area.
- At least one end-to-end happy-path smoke test for the new capability.

---

## 5) Capability Milestones

## Milestone 1 — Playable Gateway + Suggestions (MVP)
Deliverables:
- Terminal-first React UI (center terminal, right map panel placeholder, bottom suggestions bar).
- Go proxy (TCP Telnet <-> WS) with command queue/rate limit.
- CP1251 + artifact correction.
- Auto-suggest mode with strict JSON parsing and action buttons.
- OpenAPI baseline for backend endpoints + Orval-generated frontend client/documentation.

Acceptance:
- User can play through browser with stable I/O and no flood behavior.
- Suggestions appear and are executable via click.

## Milestone 2 — Reliable State + Background LLM
Deliverables:
- Parser-to-SQLite pipeline hardened.
- In-memory character/map context generation.
- Background LLM timer mode (non-spammy UI updates).

Acceptance:
- State is persisted and recovered between sessions.
- Background analysis updates state/context without cluttering main UX.

## Milestone 3 — Autopilot (Safety-First)
Deliverables:
- Goal-driven execution loop.
- Loop detection + low-HP retreat/pause safeguards.
- Immediate frontend kill switch.

Acceptance:
- Autopilot can execute bounded tasks and stop safely on risk/loop triggers.

## Milestone 4 — Macro System
Deliverables:
- Save/replay successful command chains from suggestions/autopilot output.

Acceptance:
- User can execute stored macro sequences without new LLM call.

---

## 6) Risks and Controls

1. **Encoding edge cases**
   - Control: golden sample fixtures + focused decoder tests.
2. **Flooding/anti-spam server behavior**
   - Control: queue rate limit + burst tests + telemetry.
3. **LLM malformed output**
   - Control: strict schema validation and safe fallback (drop invalid suggestions).
4. **Autopilot runaway loops**
   - Control: bounded retries, state checks, explicit kill switch.
5. **Telnet protocol quirks**
   - Control: explicit IAC handling + recorded upstream traces + simulator-backed integration tests.
6. **SQLite lock contention**
   - Control: WAL mode + single writer path + timeout telemetry.
7. **Domain parser drift (Arda text variants)**
   - Control: extend golden fixtures with Arda-specific prompt, aura, combat, and equipment transcripts.
8. **Unsafe social suggestions**
   - Control: prompt-level etiquette constraints + command allow/deny validation before enqueue.

---

## 7) Implementation Defaults (Iterations 2-4)

1. **User/session model**
   - Build for single-player runtime first.
   - Introduce clear session abstraction boundaries to support future multi-user expansion.
2. **Queue behavior**
   - Enforce fixed 500ms send interval.
   - Stop accepting new commands when queue length is 20 (surface explicit queue-full feedback).
3. **Prompt assembly/truncation order**
   - Prioritize newest server lines, then critical state snippets, then extended map/history blocks.
   - Apply deterministic truncation so behavior is repeatable and debuggable.
4. **Observability baseline**
   - Structured logs with session identifiers.
   - Basic metrics endpoint for queue length, send latency histogram, parse failures, and LLM call timing.
   - Track dropped-unsent commands with per-session labels for disconnect diagnosis.
5. **Delivery workflow**
   - Run backend/frontend locally for fast iteration.
   - Keep Docker Compose configuration continuously in sync (no long-lived drift).

6. **Orval generation layout (Iteration 3)**
   - OpenAPI input: `api/openapi/gateway.v0.yaml`
   - Orval config: `frontend/orval.config.ts`
   - Generated client output: `frontend/src/lib/api/generated/gateway.ts`
   - Generated schemas output: `frontend/src/lib/api/generated/model/`
   - Regeneration command: `npm run api:generate` (frontend package script wrapping Orval)
   - Rule: do not hand-edit generated files; edit OpenAPI spec and re-generate.

7. **Queue-full UX policy (Iteration 3)**
   - Backend returns `QUEUE_FULL` with current queue depth in error details.
   - Frontend shows inline queue status near input + toast notification.
   - Command is not auto-retried by default (user/manual retry only in MVP).

8. **Parser fixtures baseline (Iteration 3)**
   - Maintain golden fixtures under `backend/testdata/parser/`.
   - Minimum fixture set:
     - cp1251_plain_room.txt
     - cp1251_ansi_room.txt
     - smaug_ya_artifact_cases.txt
     - status_line_variants.txt
     - inventory_block_variants.txt
     - telnet_iac_negotiation_trace.txt
     - prompt_no_newline_variants.txt
     - prompt_hp_ma_mv_exp_variants.txt
     - aura_state_prefix_variants_ru.txt
     - equipment_slots_variants_ru.txt
     - combat_condition_phrases_ru.txt
   - Every parser/decoder change must pass fixture regression before merge.

9. **Suggestion freshness policy (Iteration 4)**
   - Debounce suggestion trigger by default (target 500ms-1000ms quiet window).
   - Track monotonic suggestion job IDs; ignore responses that are not from latest active job.
   - On provider timeout/error, keep previous valid suggestions visible and log failure reason.

10. **SQLite concurrency policy (Iteration 4)**
   - Enable WAL and set explicit busy timeout at DB init.
   - Serialize writes through one controlled worker path.
   - Reads may run concurrently, but must tolerate stale-by-one-event snapshot semantics.

11. **MVP reconnect/queue policy (Iteration 4)**
   - On upstream disconnect, immediately stop sender loop and mark queued commands unsent.
   - Do not auto-replay unsent commands after reconnect in MVP.
   - Surface reconnection + dropped-unsent count in status and logs.
   - Emit `session.status` on WS subscribe so frontend can hydrate current queue/session state without waiting for a transition event.

---

## 8) Milestone 1 Task Slicing (Iteration 4)

Ordered issue-level implementation plan:
1. **M1-01: Backend skeleton + health endpoint**
   - Boot Go service, config loading, structured logger, `GET /api/v0/health`.
2. **M1-02: WebSocket gateway baseline**
   - WS connect/disconnect, session ID propagation, terminal output event channel.
3. **M1-03: Command queue core**
   - Buffered queue, 500ms sender loop, max size 20, reject-new behavior.
4. **M1-04: Telnet bridge integration**
   - TCP connect/read/write loop, IAC negotiation handling, prompt-without-newline detection, session lifecycle wiring, reconnect-safe cleanup.
5. **M1-05: Decoder + artifact correction**
   - CP1251/ANSI conversion and SMAUG correction pass with golden fixtures.
6. **M1-06: Parser state extraction + snapshot API**
   - Parse key state fragments (including HP/MA/MV/EXP prompt tuple + aura/state tags + equipment slots), persist minimal schema, expose `GET /state/snapshot`.
7. **M1-07: Suggestion pipeline MVP**
   - Text update trigger with debounce, prompt assembly, strict JSON parse, stale-response discard, latest suggestion endpoint.
8. **M1-08: Frontend terminal + queue UX + suggestions**
   - Terminal center panel, input history, queue-full UX, clickable suggestion actions.
9. **M1-09: OpenAPI + Orval integration**
   - Finalize `gateway.v0.yaml`, wire Orval config/script, replace manual fetch calls.
10. **M1-10: Integration test pass + MVP hardening**
    - WS flow smoke, queue burst test, fixture regression, release checklist.

---

## 9) Iteration 5 Backlog (next planning round)

Questions to settle in Iteration 5:
1. Authentication/session ownership model for future multi-user mode.
2. Background LLM scheduler policy beyond MVP autosuggest (shared budget across timers + events).
3. Metrics schema and dashboard shortlist for live ops visibility.
4. OpenAPI versioning and deprecation policy (`v0` -> `v1` transition).
5. Release strategy (feature flags for suggestions/autopilot progression).
6. Convert `docs/arda.md` into versioned parser dictionaries (prompt regex set, aura lexicon, equipment lexicon, combat phrase map) with explicit update workflow.

---

## 10) Immediate Next Actions

1. Run dependency install locally (`cd frontend && npm install`) and generate API client (`npm run api:generate`).
2. Run an early spike: capture a short real upstream Telnet transcript and replace placeholder fixture seeds with real samples.
3. Implement M1-01 through M1-04 first (backend skeleton through Telnet bridge baseline), then continue in task order.
4. Keep docs synchronized each phase completion (`plan.md`, `docs/roadmap.md`, `README.md`).
