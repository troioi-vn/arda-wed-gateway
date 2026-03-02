# AGENTS.md

Project conventions for coding agents working in the `arda-web-gateway` repository.

## Current Project Phase (IMPORTANT)
We are intentionally in an extended **planning-first** phase.
- Prefer refining scope, contracts, milestones, and risks before coding.
- Avoid jumping into implementation unless the user explicitly asks for code.
- Keep plans concrete enough that implementation tasks can be executed without ambiguity.

## Developer Learning Goal (CRITICAL)
**The developer (Athanasius) is actively using this project to learn Golang.** 
- **Explain Go Idioms:** Whenever writing Go code, explain the concepts being used (e.g., Goroutines, Channels, Mutexes, Interfaces, and idiomatic Error handling).
- **No Black-Box Edits:** Do not just silently drop huge chunks of Go code. Break down the "why" behind the implementation. 
- **Share "Aha!" Moments:** Point out interesting language features, concurrency patterns, or elegant standard library uses after completing a coding task.
- **Discuss First:** When in planning mode, discuss technical approaches before writing code. In coding mode, simply do it, but explain the mechanics afterward.
- **Git Protocol:** The developer handles commits. Do not commit unless explicitly asked.

## Documentation Sources of Truth
- `plan.md` contains detailed iterative planning decisions.
- `docs/roadmap.md` contains high-level roadmap and phase outcomes.
- `README.md` explains project purpose, architecture, and onboarding context.
- Keep these three docs aligned when scope changes.
- Keep milestone names and AI mode labels identical across all three docs.

## Project Goal
Build a safe, observable Arda MUD web gateway that:
- Runs a React + Tailwind + shadcn/ui Frontend acting as a classic MUD terminal.
- Runs a Golang Backend handling TCP Telnet connection to `86.110.194.3:7000`.
- Proxies MUD output and user input via WebSockets.
- Translates CP1251 encoding and fixes the specific "SMAUG `Я` artifact" (lowercase Russian words displaying capital `Я`).
- Tracks state in SQLite and provides context to OpenRouter for AI assistance modes.

## Architecture Expectations
- **Immersion First:** The primary terminal should take center stage. No visual HP/Mana bars. Use right-side panels for dynamically generated Map Tiles, and a bottom panel for LLM-suggested action buttons.
- **Go Concurrency:** Utilize Goroutines and Channels to decouple the Telnet read loop, WebSocket broadcasting, and slow OpenRouter API calls.
- **Command Queuing:** The backend MUST buffer and rate-limit commands (from fast typing or macros) to prevent flooding the MUD server.
- **State Generation:** Store parsed state (inventory, location, objects) in SQLite. Dynamically generate `character.md` and `map.md` raw strings in-memory to bundle into OpenRouter prompts. Do not physically write these to disk.
- **State Snapshot Baseline (M1-06):** `GET /api/v0/state/snapshot` returns parser-derived fields for `location`, `prompt` tuple (`hp_current`, `hp_max`, `ma_current`, `ma_max`, `mv_current`, `mv_max`, `exp`), `status_tags`, `equipment`, and `updated_at`.
- **API Contract First:** Define backend HTTP contract in OpenAPI first; generate frontend API client and docs via Orval.

## Runtime Defaults
- SQLite path env var is `APP_SQLITE_PATH` (default: `tmp/gateway.sqlite`).

## LLM Operating Regimes
1. **Auto-Suggest Mode:** Triggers strictly on new text updates. Provides context-aware action buttons on the UI below the terminal.
2. **Background LLM Mode:** Runs on a timer (e.g., 30s) or accumulated log context. Updates mapping and state analysis without spamming the UI with action suggestions.
3. **Autopilot Mode:** Goal-oriented execution loop. Requires strict safety guards (anti-looping, low-HP retreat) and a React-accessible kill switch to pause execution immediately.
4. **Macro System:** Support repeating chains of successful LLM actions natively through the UI without recalling the API.

## Decision Contract (OpenRouter)
LLM output must be structured strictly (typically JSON depending on the prompt) to guarantee predictable parsing into frontend actionable buttons and macro sequences:
- Suggested `commands` array
- `reason` for actions
- `expected_outcome`

## Planning Quality Bar
When updating plans, include:
- Explicit goals and non-goals.
- Acceptance criteria per milestone.
- Risks and mitigations.
- Testing strategy tied to each capability.
- Operational defaults (queue policy, failure behavior, observability).

## Testing Expectations
- Validate WebSocket channel broadcasting.
- Ensure the Go parser handles Telnet ANSI streams and Russian generic decoding correctly.
- Cover fixture-driven parser extraction for prompt tuple, aura/status tags, equipment slots, and room/location title parsing.
- Test command queuing delay (e.g., simulating 10 rapid inputs).
- Validate OpenAPI contract consistency and generated-client workflow.
