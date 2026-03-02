# Arda Web Gateway - Development Plan

## Overview
A web-based intelligent gateway connecting a modern React frontend via WebSocket to a Backend service, which negotiates a Telnet connection to the older MUD application (`86.110.194.3:7000`). It functions as a classic client to the user but integrates LLM-based intelligent "suggesting" and full "autopilot" modes via OpenRouter.

## Tech Stack
- **Frontend:** React + TailwindCSS + shadcn/ui (for terminal-like UI, map, and action panels).
- **Backend:** **Golang** (chosen for real-time networking, excellent WebSockets, and low-latency TCP proxying/concurrency).
- **Communication:** WebSockets (Frontend <-> Backend), Telnet TCP (Backend <-> MUD Server).
- **AI/LLM:** OpenRouter API (strict JSON inference)
- **State Storage:** **SQLite database** backing in-memory tracking. We dynamically generate `character.md` and `map.md` strings for the LLM context, rather than managing raw text on disk. 
- **Infrastructure:** Docker and `docker-compose.yaml` (Database, Backend, Frontend).

## Phase 1: Basic MUD Client
**Goal:** Establish a 1:1 playable Telnet surrogate in the browser.
1. **Scaffold Services:** Spin up the React App (`Vite` + `React`) and Go backend server inside a `docker-compose` setup.
2. **Go Backend Proxy:** Connect backend to the MUD via TCP `net.Dial`. Implement CP1251 generic decoding, handling the SMAUG artifact (`Я` -> `я`).
3. **WebSockets Layer:** Stream raw Telnet ansi-colored strings down the WebSocket, pass input strings up to the MUD.
4. **React Interface:** 
   - Build a primary terminal taking center stage to preserve immersion (no visual HP/Mana bars).
   - Add a map tile grid panel on the right.
   - Add LLM "Suggested Actions" as clickable buttons on the bottom.
   - Implement command input box with history (`ArrowUp/ArrowDown`).
   - Add local storage for saving MUD credentials (`localStorage` or simple indexedDB).

## Phase 2: Context Parsing & Auto-Mapping
**Goal:** Track the state of the game efficiently and store in SQLite. 
1. **Parser Engine Rewrite (Go):** Migrate Regex tools (`hp/mana/move`, `location`, `inventory`, `combat state`) to the Golang backend.
2. **State Syncing (Database):** As events happen, record them to the DB. Expose a `/state` REST endpoint or WebSocket sub-channel.
3. **Map Generator Engine:** Introduce in-memory map graph (persisted to SQLite). It records locations, descriptions, and exits.
4. **Dynamic MD Generation:** On LLM execution, generate `character.md` and `map.md` cleanly formatted strings straight from the database to inject into prompts.
5. **WebSocket Sync:** Send string representation blocks to the React UI sidebars (exclusively Map tile data, LLM decisions, and basic text feeds to preserve classic MUD immersion)

## Phase 3: Suggestion Engine
**Goal:** Use OpenRouter to suggest the next best moves based on current context.
1. **Two UI Regimes:**
   - **Auto-Suggest Mode:** Triggers the LLM strictly on text updates to suggest context-aware buttons on the bottom row.
   - **Background LLM Mode:** LLM only runs on a silent timer or after significant context buildup (e.g. 30 sec) for state analysis without filling up the UI with action spam.
2. **Go Backend Engine:**
   - Listen for new prompts and triggers (e.g. `You received a new quest: ...` or `You enter a new room.`).
   - Bundle dynamically generated `character.md`, `map.md`, and the last 15-20 lines of server text into a system prompt.
   - Call OpenRouter API.
   - Parse strict JSON responses from the LLM expecting `[commands...]`.
3. **Presentation:** Send suggested commands down the socket. The Frontend displays them as clickable buttons beside or above the input row.
4. **Macros Feature:** Allow the UI to label repeated suggestion chains as "Macros" for fast reuse without another LLM call.

## Phase 4: Autopilot Mode
**Goal:** Build a fully autonomous, safely-looped task execution system.
1. **Input:** User sets a high-level goal text field ("Kill the dragon").
2. **Execution Loop (Go channels/workers):**
   - Backend evaluates `character.md`, `map.md` representation, last logs + the **Goal**.
   - Issues a command.
   - Waits for parser to confirm outcome or timeout.
   - Issues next command.
3. **Safety Guards:**
   - Detect loops (e.g. failing to move 3 times).
   - Detect catastrophic risk (low HP -> run away or pause).
   - "Halt/Pause" immediate kill switch accessible from React frontend via WS message payload.


## Developer Note
**I as a developer want to learn Go through this project.** When generating the `AGENTS.md` for the new repo, this fact must be explicitly stated so future AI coding agents prioritize teaching moments, explaining idiomatic Go principles (Channels, Goroutines, robust Error handling), and avoid doing silent black-box edits in Go.
