# Before Debug Plan: Error Logging + Error Handling (Points 1-5)

## Purpose
Harden runtime error handling and observability for the five prioritized findings from the debugging review:
1. Frontend network/parse failures are not handled in key user actions.
2. Backend JSON response write errors are swallowed.
3. Command queue drops failed sends without retry/client visibility.
4. Backend handlers return `INTERNAL_ERROR` without logging root causes.
5. Request logging lacks status/request ID/error correlation.

## Goals
- Eliminate unhandled promise rejections in frontend command/session flows.
- Ensure backend write failures and internal handler failures are logged with context.
- Make queue send failures observable and non-silent.
- Improve request-level traceability (`request_id`, status, latency, and failure reason).
- Add tests that lock in error-path behavior.

## Non-Goals
- Redesigning API envelope schema or OpenAPI shape beyond additive details.
- Changing product UX flows outside error visibility/robustness.
- Implementing full retry orchestration for every subsystem beyond queue send policy scope.

## Workstream 1: Frontend async error handling (Point 1)
### Plan
- Add `try/catch` around `handleConnectClick`, `handleDisconnectClick`, and enqueue call sites.
- Keep existing status-code handling for structured API errors.
- In `catch`, surface a stable user message and include richer context for debug logging (e.g., `console.error` with action label and error object).
- Normalize unknown thrown values (`unknown`) into a user-safe message function.

### Acceptance criteria
- No unhandled promise rejection in connect/disconnect/enqueue/suggestion-click flows.
- User always gets feedback toast/status on transport failure.
- Existing successful behavior remains unchanged.

### Testing strategy
- Frontend tests for rejected API promises:
  - connect request rejects -> toast/status shown.
  - disconnect request rejects -> toast/status shown.
  - enqueue request rejects -> no crash; toast/status shown; input focus behavior preserved.

### Risks and mitigations
- Risk: Duplicate toasts/status churn on mixed HTTP + thrown errors.
- Mitigation: Centralize error-to-message mapping and call once per action.

## Workstream 2: Backend JSON write-path logging (Point 2)
### Plan
- Replace silent `_ = json.NewEncoder(...).Encode(...)` behavior with checked error handling.
- Log failed JSON writes with request metadata (`request_id`, route context when available, status intended).
- Keep response semantics unchanged where headers are already written; focus on observability.

### Acceptance criteria
- JSON encode/write failure is always logged.
- No panic introduced by write failures.

### Testing strategy
- Unit test with custom `http.ResponseWriter` that fails on `Write`:
  - verify no panic.
  - verify logger records failure.

### Risks and mitigations
- Risk: Double-write behavior if code attempts fallback body after partial write.
- Mitigation: Never attempt second response body after encode failure; log only.

## Workstream 3: Queue send failure policy + observability (Point 3)
### Plan
- Define explicit policy for sender failures:
  - Option A: bounded retry with backoff for transient errors.
  - Option B: no retry but emit explicit failure event + increment failure metric.
- Implement chosen policy with deterministic behavior.
- Emit a websocket event for command send failure (session-scoped, with queue metadata).
- Add dedicated metric(s): send failures total, optional retries total.

### Acceptance criteria
- Failed sends are no longer silent.
- Operators can detect failures from logs/metrics.
- Client receives actionable signal when command was not sent.

### Testing strategy
- Queue tests using failing sender:
  - failure increments metric.
  - expected event/log emitted.
  - retry behavior (if enabled) follows configured max attempts.

### Risks and mitigations
- Risk: Retry can reorder commands or increase latency.
- Mitigation: Preserve FIFO semantics by retrying the same head command before popping next; document limits.
- Risk: Event flood under unstable upstream.
- Mitigation: add rate-limit/coalescing for repeated failure events if needed.

## Workstream 4: Internal error root-cause logging in handlers (Point 4)
### Plan
- Before returning generic `INTERNAL_ERROR`, log original `err` with handler name and `request_id`.
- Apply to connect/disconnect/enqueue/state snapshot failure paths.
- Keep client-facing messages generic to avoid leaking internals.

### Acceptance criteria
- Every handler branch that returns `INTERNAL_ERROR` emits one structured log line with root cause.
- `request_id` is present for correlation.

### Testing strategy
- Handler tests with failing manager/state stubs:
  - status code remains correct.
  - logger captures expected structured fields.

### Risks and mitigations
- Risk: Logging sensitive values.
- Mitigation: Log error and operational context only; avoid raw secrets/payload dumps.

## Workstream 5: Request logging correlation improvements (Point 5)
### Plan
- Upgrade middleware logging to include response status and request ID.
- Introduce response-writer wrapper to capture status code and bytes written.
- Ensure request ID is generated once per request and shared across handler + middleware logs.
- Log at `Info` for non-2xx/3xx; keep successful requests at `Debug` (or configurable level).

### Acceptance criteria
- Request completion logs include: method, path, status, elapsed_ms, request_id.
- Error responses are easily joinable with handler logs.

### Testing strategy
- Middleware tests:
  - handler without explicit `WriteHeader` still reports 200.
  - explicit error status reports correct status.
  - request ID appears in completion log.

### Risks and mitigations
- Risk: noisy logs at scale.
- Mitigation: log-level split by status class and keep payload-free structured fields.

## Delivery Sequence
1. Workstream 4 (root-cause logs) + Workstream 2 (JSON write logging) first for immediate backend visibility.
2. Workstream 5 middleware correlation.
3. Workstream 1 frontend async guards.
4. Workstream 3 queue failure policy + metrics/events (largest behavior change).
5. Final regression pass and contract sanity checks.

## Definition of Done
- All five workstreams implemented with tests.
- No unhandled frontend promise rejections during manual smoke flows.
- Backend logs can trace one failed request end-to-end via `request_id`.
- Queue send failures are visible in logs + metrics + client signal.
