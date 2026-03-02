# Phase 1 Plan — Playable Gateway + Suggestions (MVP)

## Goal
Ship a stable terminal-first browser gateway that connects to Arda MUD, enforces safe command pacing, and provides clickable AI suggestions.

## Non-Goals
- Autopilot execution loops.
- Advanced map rendering polish.
- Multi-user production tenancy.

## Operational Defaults (must be implemented)
- Command send interval: `500ms`
- Max queue size: `20`
- Queue overflow behavior: reject new command with `QUEUE_FULL`
- Reconnect behavior: do not replay unsent commands automatically
- Suggestion freshness: debounce + stale-response discard

## Task List

### P1-T01: Bootstrap backend service skeleton
Instructions:
1. Create backend module layout for config, logging, HTTP server, and lifecycle management.
2. Add startup configuration for host/port and build metadata.
3. Add graceful shutdown handling.

Expected outcome:
- Service starts, logs structured startup fields, and shuts down cleanly on signal.

### P1-T02: Implement health endpoint
Instructions:
1. Add `GET /api/v0/health` handler.
2. Return standardized success envelope with status and build metadata.
3. Add error envelope helper shared by all handlers.

Expected outcome:
- Health endpoint responds with stable schema matching `plan.md` response envelope format.

### P1-T03: Define OpenAPI v0 baseline
Instructions:
1. Create `api/openapi/gateway.v0.yaml`.
2. Define endpoints listed in `plan.md` section “OpenAPI v0 Scope”.
3. Define stable error codes and shared success/error envelope schemas.

Expected outcome:
- OpenAPI spec is valid and fully describes MVP HTTP surface with standardized error model.

### P1-T04: Add Orval generation pipeline
Instructions:
1. Add `frontend/orval.config.ts`.
2. Add frontend script `npm run api:generate`.
3. Generate client output paths exactly as defined in `plan.md`.

Expected outcome:
- Frontend API client is generated from OpenAPI without manual edits.

### P1-T05: Build WebSocket session gateway
Instructions:
1. Add WS endpoint for terminal stream.
2. Implement connect/disconnect session lifecycle with single-user-first boundaries.
3. Emit session IDs in logs and runtime events.

Expected outcome:
- Browser client can connect/disconnect WS channel and receive backend events.

### P1-T06: Build command queue and sender loop
Instructions:
1. Implement bounded FIFO queue with max depth `20`.
2. Enforce fixed send interval `500ms` in sender worker.
3. Reject enqueue when full and return `QUEUE_FULL` with queue depth details.

Expected outcome:
- Burst input is rate-limited, preserves order, and exposes explicit overflow response.

### P1-T07: Implement Telnet bridge core
Instructions:
1. Add upstream TCP connect/read/write loops.
2. Filter Telnet negotiation bytes (`IAC`, `WILL/WONT`, `DO/DONT`, subnegotiation) from user-visible stream.
3. Wire disconnect handling to stop sender loop and mark pending commands unsent.

Expected outcome:
- Telnet traffic is bridged to WS cleanly with deterministic negotiation handling.

### P1-T08: Add decoder and SMAUG artifact correction
Instructions:
1. Implement CP1251 to UTF-8 decoding pipeline.
2. Preserve ANSI content where needed for parser compatibility.
3. Add correction pass for known SMAUG lowercase `Я` artifact contexts.

Expected outcome:
- Russian text displays correctly and artifact regression samples pass.

### P1-T09: Add prompt detection for non-newline endings
Instructions:
1. Implement prompt detector that does not require trailing `\n`.
2. Support HP/MA/MV/EXP tuple variants described in `plan.md`.
3. Trigger suggestion cycle and UI prompt update on prompt detection.

Expected outcome:
- Prompt-driven interactions remain responsive even for classic MUD non-newline prompts.

### P1-T10: Implement parser state extraction baseline
Instructions:
1. Extract location, status tokens, aura prefixes, and equipment slot states.
2. Persist minimal state to SQLite schema (`rooms`, `exits`, `character_stats`, `inventory`, `events`).
3. Expose `GET /api/v0/state/snapshot` projection endpoint.

Expected outcome:
- Snapshot endpoint returns parser-derived state used by frontend and prompt builder.

### P1-T11: Implement suggestion pipeline MVP
Instructions:
1. Trigger on new text updates with debounce window (`500-1000ms` target).
2. Build prompt context from newest lines + critical state + extended context (deterministic truncation order).
3. Parse strict JSON response (`commands`, `reason`, `expected_outcome`) and discard stale jobs.
4. Persist/serve latest valid suggestion via `GET /api/v0/suggestions/latest`.

Expected outcome:
- Suggestions are stable, parse-safe, and never overwritten by stale responses.

### P1-T12: Implement frontend terminal and suggestion UX
Instructions:
1. Build terminal-first layout: center terminal, right map placeholder, bottom suggestions bar.
2. Add input box with history and enqueue action.
3. Show queue-full inline status + toast.
4. Render suggestion buttons and send selected commands through enqueue API.

Expected outcome:
- User can play from browser and execute suggestions by click with visible queue state.

### P1-T13: Build parser/decoder golden fixtures
Instructions:
1. Add fixture files under `backend/testdata/parser/` listed in `plan.md`.
2. Include Telnet negotiation trace and non-newline prompt variants.
3. Create regression test harness that validates decoder + parser output against fixtures.

Expected outcome:
- Every parser/decoder change is verified against a stable golden corpus.

### P1-T14: Add integration and queue behavior tests
Instructions:
1. Add WS integration test for connect/send/receive/disconnect.
2. Add burst enqueue test (10 rapid commands) validating order and interval.
3. Add reconnect policy test asserting no hidden replay of unsent commands.

Expected outcome:
- Core transport and queue safety behaviors are enforced by repeatable tests.

### P1-T15: Observability baseline
Instructions:
1. Add structured logs with session IDs and request IDs.
2. Add metrics endpoint exposing queue length, send latency, parse failures, LLM timing.
3. Add log events for disconnect with dropped-unsent command count.

Expected outcome:
- Runtime behavior is debuggable with baseline logs and metrics.

### P1-T16: Docs update (required)
Instructions:
1. Update `plan.md` with any implementation-level deviations from this phase plan.
2. Update `docs/roadmap.md` Phase 1 status and exit-criteria progress.
3. Update `README.md` setup/usage notes and any changed operational defaults.
4. Verify milestone names and AI mode labels remain identical across all three docs.

Expected outcome:
- Documentation remains synchronized with implementation outcomes and decisions.

## Phase Exit Checklist
- Browser connect/send/receive loop is stable with Telnet bridge.
- Queue safety defaults are enforced and visible in UX.
- CP1251 + artifact correction pass fixture regression.
- Suggestions render and execute safely from strict JSON output.
- OpenAPI + Orval workflow is active and reproducible.
- Required docs update task is completed.
