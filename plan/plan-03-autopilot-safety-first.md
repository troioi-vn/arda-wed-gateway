# Phase 3 Plan — Autopilot (Safety-First)

## Goal
Introduce bounded autonomous execution with strict safeguards, transparent control, and immediate user override.

## Non-Goals
- Fully unattended long-running gameplay.
- Hidden decision-making without auditability.
- Bypassing queue/rate-limit rules.

## Safety Invariants (must always hold)
- All autopilot commands must pass through the same queue/rate-limit path as manual commands.
- Immediate kill switch must stop new autopilot decisions and command dispatch.
- Autopilot must halt on risk triggers (low HP policy, loop detection, repeated failures).

## Task List

### P3-T01: Define autopilot finite-state model
Instructions:
1. Define explicit autopilot states (`idle`, `planning`, `executing`, `paused`, `halted`, `error`).
2. Define allowed transitions and guard conditions.
3. Add transition audit logging.

Expected outcome:
- Autopilot behavior is deterministic, inspectable, and testable via state transitions.

### P3-T02: Define goal contract and validation
Instructions:
1. Create structured goal input schema (goal text, optional constraints, timeout budget).
2. Validate and reject unsafe/invalid goal payloads before run start.
3. Persist run metadata with unique run IDs.

Expected outcome:
- Every autopilot run starts from validated, traceable configuration.

### P3-T03: Implement bounded decision loop
Instructions:
1. Execute think-act-observe cycle with strict max step count per run.
2. Enforce wall-clock timeout and inactivity timeout.
3. Stop loop when no valid next action exists.

Expected outcome:
- Autopilot cannot run indefinitely and exits cleanly on bounds.

### P3-T04: Implement command safety gate
Instructions:
1. Add allow/deny validation before enqueue (forbidden command patterns, optional allowlist).
2. Reject ambiguous multi-command payloads unless explicitly supported.
3. Record safety-gate decision reason for each command.

Expected outcome:
- Unsafe or malformed commands are blocked before reaching upstream.

### P3-T05: Integrate low-HP retreat policy
Instructions:
1. Define configurable low-HP threshold policy using parser state.
2. On threshold breach, pause or halt run and emit safety reason.
3. Optionally enqueue predefined safe retreat sequence only if policy allows.

Expected outcome:
- Autopilot avoids reckless behavior when character state is dangerous.

### P3-T06: Implement loop and repetition detection
Instructions:
1. Track recent command signatures and outcomes in rolling window.
2. Trigger pause/halt when repeated ineffective cycles exceed threshold.
3. Surface loop-detection reason to UI and logs.

Expected outcome:
- Autopilot self-stops on runaway repetitive patterns.

### P3-T07: Add immediate kill switch API + UI control
Instructions:
1. Add backend endpoint/event for kill switch activation.
2. Add always-visible UI kill switch control in React.
3. Ensure kill switch is idempotent and works during any autopilot state.

Expected outcome:
- User can stop autopilot instantly and reliably from UI.

### P3-T08: Add pause/resume semantics
Instructions:
1. Implement manual pause and resume controls with state preservation.
2. Prevent resume when safety halt reason requires explicit restart.
3. Show current autopilot state and last transition reason in UI.

Expected outcome:
- User retains transparent, fine-grained control over autopilot execution.

### P3-T09: Persist autopilot run ledger
Instructions:
1. Store run timeline with decisions, commands, safety events, and stop reasons.
2. Link ledger records to session ID and goal ID.
3. Add read endpoint for latest run summary.

Expected outcome:
- Autopilot actions are auditable for debugging and trust.

### P3-T10: Add strict model-output parsing for autopilot decisions
Instructions:
1. Define JSON schema for autopilot decision outputs.
2. Reject outputs missing required fields or violating policy.
3. On parse failure, increment error counters and apply bounded retry policy.

Expected outcome:
- Invalid LLM outputs cannot silently drive autonomous actions.

### P3-T11: Add autopilot observability package
Instructions:
1. Add metrics for run starts/stops, safety halts, loop detections, and step durations.
2. Add structured logs with run ID correlation.
3. Add alert-friendly counters for repeated safety-trigger events.

Expected outcome:
- Operators can monitor safety behavior and diagnose failures rapidly.

### P3-T12: Add safety-focused unit tests
Instructions:
1. Test state-machine transition validity and illegal transition rejection.
2. Test low-HP trigger behavior and kill switch preemption.
3. Test loop detection thresholds and halt behavior.
4. Test command safety-gate allow/deny logic.

Expected outcome:
- Core safety invariants are protected by automated tests.

### P3-T13: Add autopilot integration tests
Instructions:
1. Run bounded autopilot scenario against simulator upstream.
2. Validate stop on loop, stop on low HP, and stop on kill switch.
3. Validate command path still respects queue interval and limits.

Expected outcome:
- Autopilot behavior is proven safe in end-to-end test conditions.

### P3-T14: Docs update (required)
Instructions:
1. Update `plan.md` with finalized autopilot safety rules and execution bounds.
2. Update `docs/roadmap.md` Phase 3 progress and safety exit-criteria status.
3. Update `README.md` with user-facing autopilot controls and safeguards.
4. Verify milestone names and AI mode labels remain identical across all three docs.

Expected outcome:
- Documentation clearly explains autonomy boundaries, safeguards, and controls.

## Phase Exit Checklist
- Autopilot runs only within explicit safety and time/step bounds.
- Kill switch reliably halts execution immediately.
- Loop and low-HP protections are active and tested.
- All autopilot actions are auditable via logs/ledger.
- Required docs update task is completed.
