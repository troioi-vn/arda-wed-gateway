# Phase 2 Plan — Reliable State + Background LLM

## Goal
Stabilize parser-to-state persistence and add low-noise background analysis that improves context without interrupting terminal flow.

## Non-Goals
- Full autonomous action execution.
- Replacing Auto-Suggest as the primary interactive mode.
- Heavy visualization redesign.

## Task List

### P2-T01: Audit and harden parser event model
Instructions:
1. Define canonical parser event types for room, exits, character stats, inventory, combat conditions, and status tags.
2. Add normalization rules for Russian aura/state prefixes and equipment slot labels.
3. Document event schema in code comments and developer docs.

Expected outcome:
- Parser emits consistent events consumable by state persistence and context builders.

### P2-T02: Implement controlled SQLite write worker
Instructions:
1. Route all DB writes through one serialized worker path.
2. Use transactional batches where appropriate to reduce lock churn.
3. Expose internal metrics for write queue depth and transaction latency.

Expected outcome:
- Write contention is controlled and observable under sustained parser load.

### P2-T03: Enforce SQLite runtime concurrency defaults
Instructions:
1. Enable WAL mode at DB initialization.
2. Set explicit busy timeout.
3. Validate behavior under concurrent reads plus active writer worker.

Expected outcome:
- Database remains stable with concurrent access and expected stale-by-one-event semantics.

### P2-T04: Build state projection layer
Instructions:
1. Add query layer to materialize state snapshot from normalized tables.
2. Guarantee deterministic ordering for inventory, exits, and recent events.
3. Keep API contract stable for existing `GET /api/v0/state/snapshot` consumers.

Expected outcome:
- Snapshot payload is consistent across runs and resilient to incremental parser changes.

### P2-T05: Implement in-memory context assembly
Instructions:
1. Generate `character.md` content string from current character and equipment state.
2. Generate `map.md` content string from rooms/exits/events context.
3. Keep both strings in memory only; do not write markdown files to disk.

Expected outcome:
- Background and suggestion LLM prompts receive richer structured context safely.

### P2-T06: Build Background LLM scheduler
Instructions:
1. Add timer-based job trigger (default cadence configurable, example: 30s).
2. Add guard so only one background job runs at a time.
3. Skip job when session disconnected or context is insufficient.

Expected outcome:
- Background analysis runs predictably without flooding provider or UI.

### P2-T07: Add background prompt budget policy
Instructions:
1. Reuse deterministic truncation strategy from MVP suggestion pipeline.
2. Prioritize newest critical events and current state over old history.
3. Log truncation decisions for debugging and reproducibility.

Expected outcome:
- Background prompts remain bounded, stable, and debuggable.

### P2-T08: Process background output into state annotations
Instructions:
1. Parse structured analysis output using strict schema validation.
2. Store derived annotations separately from raw parser state to avoid data corruption.
3. Add source metadata (provider, model, timestamp, job id).

Expected outcome:
- LLM-derived insights are traceable, isolated, and safely merged into read models.

### P2-T09: UI integration for low-noise background updates
Instructions:
1. Surface background analysis in secondary UI areas only (not as intrusive action prompts).
2. Avoid auto-focus shifts, modal interruptions, or terminal scroll jumps.
3. Provide timestamp and freshness indicator for last background analysis.

Expected outcome:
- Users gain context value without terminal immersion breakage.

### P2-T10: Failure policy and fallback behavior
Instructions:
1. On background timeout/error, keep last valid analysis visible.
2. Log structured failure reason and increment metrics.
3. Apply retry policy with bounded backoff and max attempts.

Expected outcome:
- Background mode degrades gracefully and never blocks core play loop.

### P2-T11: Add targeted unit tests
Instructions:
1. Test parser event normalization and projection determinism.
2. Test SQLite writer serialization and timeout behavior.
3. Test context-string generation for missing/partial state scenarios.
4. Test background output schema validation failure paths.

Expected outcome:
- Core Phase 2 logic is protected against regressions by focused unit coverage.

### P2-T12: Add integration tests for background flow
Instructions:
1. Simulate active session with periodic background jobs.
2. Validate no UI-spam channel events are emitted.
3. Validate state remains queryable while background writes occur.

Expected outcome:
- End-to-end background mode is stable under realistic concurrent activity.

### P2-T13: Observability extensions
Instructions:
1. Add metrics for background job duration, success/failure, and queue skips.
2. Add structured logs with correlation IDs linking source text chunk to analysis result.
3. Add dashboard-ready metric names and labels for Phase 2 operations.

Expected outcome:
- Operators can quickly diagnose background mode performance and failure patterns.

### P2-T14: Docs update (required)
Instructions:
1. Update `plan.md` with finalized scheduler defaults, fallback policy, and schema notes.
2. Update `docs/roadmap.md` Phase 2 outcome/exit progress.
3. Update `README.md` with Background LLM behavior and operational notes.
4. Verify milestone names and AI mode labels remain identical across all three docs.

Expected outcome:
- Documentation accurately reflects Phase 2 architecture and operational behavior.

## Phase Exit Checklist
- Parser-to-SQLite pipeline is stable with controlled writes and WAL defaults.
- In-memory `character.md` and `map.md` context strings are active in prompts.
- Background LLM updates are useful and non-intrusive.
- Tests and observability cover the new concurrency and scheduler paths.
- Required docs update task is completed.
