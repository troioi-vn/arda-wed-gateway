# Phase 4 Plan — Macro System

## Goal
Enable users to save, manage, and replay successful command chains safely, reducing redundant LLM calls for repeated play patterns.

## Non-Goals
- Replacing manual command entry.
- Running macros outside normal queue/rate-limit controls.
- Hidden or non-auditable automation behavior.

## Task List

### P4-T01: Define macro data model
Instructions:
1. Define macro schema (name, description, command list, tags, source, created/updated timestamps).
2. Add validation limits (max commands per macro, max command length, naming rules).
3. Add schema migration plan for future macro metadata fields.

Expected outcome:
- Macro persistence model is explicit, bounded, and forward-compatible.

### P4-T02: Implement macro storage and CRUD API
Instructions:
1. Add storage tables and repository methods.
2. Add endpoints for create, list, get, update, delete macros.
3. Use standardized success/error envelope and stable error codes.

Expected outcome:
- Macro definitions can be managed reliably through documented API surface.

### P4-T03: Add macro source attribution
Instructions:
1. Track whether macro was created manually, from suggestion actions, or from autopilot run.
2. Store link references to originating suggestion/run IDs when available.
3. Expose attribution fields in list/detail API responses.

Expected outcome:
- Users can trace macro provenance and confidence context.

### P4-T04: Implement macro execution engine
Instructions:
1. Expand macro into command sequence and submit through existing queue pipeline.
2. Preserve command order and per-command execution status.
3. Respect all existing queue/rate-limit settings (no bypass).

Expected outcome:
- Macro replay behaves like safe batched manual input with full queue protection.

### P4-T05: Add macro execution controls
Instructions:
1. Add start, pause, resume, cancel controls for active macro run.
2. Stop execution on disconnect and report unsent remainder.
3. Add clear run status model (`queued`, `running`, `paused`, `completed`, `failed`, `canceled`).

Expected outcome:
- Macro runs are controllable and transparent at runtime.

### P4-T06: Implement macro safety checks
Instructions:
1. Validate each command through the same allow/deny safety gate used elsewhere.
2. Block execution if macro contains forbidden commands, return actionable error details.
3. Add optional dry-run validation endpoint.

Expected outcome:
- Unsafe macro content is prevented before execution reaches upstream.

### P4-T07: Add idempotency and duplicate-trigger protection
Instructions:
1. Add run request IDs for macro execution start operations.
2. Prevent accidental double-start from rapid repeated clicks.
3. Return existing run reference when duplicate start is detected.

Expected outcome:
- Users do not unintentionally launch duplicate macro executions.

### P4-T08: Add frontend macro management UI
Instructions:
1. Add macro library panel with list/search/filter.
2. Add create/edit/delete UI forms with inline validation.
3. Add run controls and real-time status display.

Expected outcome:
- Users can manage and run macros directly from UI with clear feedback.

### P4-T09: Add quick-save from suggestions/autopilot outputs
Instructions:
1. Add UI action to convert a suggestion command set into macro draft.
2. Add UI action to save successful autopilot sequence as macro draft.
3. Prefill metadata (title, source attribution, tags) for user confirmation.

Expected outcome:
- Successful repeated patterns can be captured quickly without manual re-entry.

### P4-T10: Add macro observability
Instructions:
1. Add metrics for macro starts/completions/failures/cancellations.
2. Add metrics for average macro length and execution duration.
3. Add structured logs linking macro ID, run ID, and command outcomes.

Expected outcome:
- Macro reliability and usage are measurable for operational tuning.

### P4-T11: Add macro unit tests
Instructions:
1. Test macro schema validation and boundary limits.
2. Test execution ordering and pause/resume/cancel transitions.
3. Test safety gate enforcement for forbidden commands.
4. Test duplicate-trigger protection behavior.

Expected outcome:
- Macro core behavior is regression-protected by focused unit coverage.

### P4-T12: Add macro integration tests
Instructions:
1. Execute macro replay through queue with simulated upstream.
2. Validate status transitions and failure behavior on disconnect.
3. Validate no queue policy bypass occurs during macro runs.

Expected outcome:
- End-to-end macro replay is predictable, safe, and policy-compliant.

### P4-T13: OpenAPI and client generation updates
Instructions:
1. Extend OpenAPI spec with macro CRUD and execution endpoints.
2. Regenerate frontend client with Orval.
3. Replace manual frontend calls with generated client functions.

Expected outcome:
- Macro API remains contract-first and frontend/backend stay synchronized.

### P4-T14: Docs update (required)
Instructions:
1. Update `plan.md` with finalized macro constraints, execution semantics, and failure policy.
2. Update `docs/roadmap.md` Phase 4 outcome and exit-criteria progress.
3. Update `README.md` with user-facing macro workflows and safety notes.
4. Verify milestone names and AI mode labels remain identical across all three docs.

Expected outcome:
- Documentation stays aligned with macro capabilities and operational behavior.

## Phase Exit Checklist
- Macro CRUD and replay are stable and policy-compliant.
- Macro execution uses existing queue/rate-limit and safety-gate paths.
- UI supports creation, management, and controlled replay.
- OpenAPI and generated client include macro surface.
- Required docs update task is completed.
