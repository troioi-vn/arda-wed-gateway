# AGENTS.md

Project conventions for coding agents working in the `arda-web-gateway` repository.

## Developer Learning Goal (CRITICAL)
**The developer (Athanasius) is actively using this project to learn Golang.** 
- **Explain Go Idioms:** Whenever writing Go code, explain the concepts being used (e.g., Goroutines, Channels, Mutexes, Interfaces, and idiomatic Error handling).
- **No Black-Box Edits:** Do not just silently drop huge chunks of Go code. Break down the "why" behind the implementation. 
- **Share "Aha!" Moments:** Point out interesting language features, concurrency patterns, or elegant standard library uses after completing a coding task.
- **Discuss First:** When in planning mode, discuss technical approaches before writing code. In coding mode, simply do it, but explain the mechanics afterward.
- **Git Protocol:** The developer handles commits. Do not commit unless explicitly asked.

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

## Testing Expectations
- Validate WebSocket channel broadcasting.
- Ensure the Go parser handles Telnet ANSI streams and Russian generic decoding correctly.
- Test command queuing delay (e.g., simulating 10 rapid inputs).
