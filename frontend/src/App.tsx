import { CSSProperties, FormEvent, ReactNode, useEffect, useMemo, useRef, useState } from "react";
import {
  getSuggestionsLatest,
  postCommandsEnqueue,
  postSessionConnect,
  postSessionDisconnect,
} from "./lib/api/generated/gateway";
import { ErrorBodyCode } from "./lib/api/generated/model";
import tmpTriggersRaw from "./tmp-triggers.json";

type TerminalEvent = {
  event: string;
  session_id: string;
  connected?: boolean;
  text?: string;
  queue_depth?: number;
  queue_max?: number;
  queue_reject_code?: string;
  timestamp?: string;
};

type Toast = {
  id: number;
  message: string;
};

type SuggestionView = {
  commands: string[];
  reason: string;
  expectedOutcome: string;
  generatedAt: string;
};

type SuggestionStatus = "idle" | "loading" | "ready" | "stale" | "error";
type TmpTrigger = {
  id: string;
  trigger: string;
  actions: string[];
};

const DEFAULT_HOST = "86.110.194.3";
const DEFAULT_PORT = 7000;
const SUGGESTION_POLL_INTERVAL_MS = 1600;

type AnsiState = {
  bold: boolean;
  fg: string | null;
  bg: string | null;
};

const ANSI_FG_COLORS: Record<number, string> = {
  30: "#0f0f0f",
  31: "#e06c75",
  32: "#98c379",
  33: "#e5c07b",
  34: "#61afef",
  35: "#c678dd",
  36: "#56b6c2",
  37: "#d7dae0",
  90: "#5c6370",
  91: "#f7768e",
  92: "#9ece6a",
  93: "#e0af68",
  94: "#7aa2f7",
  95: "#bb9af7",
  96: "#7dcfff",
  97: "#f4f4f8",
};

const ANSI_BG_COLORS: Record<number, string> = {
  40: "#0f0f0f",
  41: "#742f3b",
  42: "#39522f",
  43: "#5f4f2d",
  44: "#2f4662",
  45: "#4d3562",
  46: "#2f575d",
  47: "#b9bec8",
  100: "#434852",
  101: "#9e4b5a",
  102: "#557749",
  103: "#8b7545",
  104: "#4f668d",
  105: "#6f568f",
  106: "#4b7b80",
  107: "#d9dde5",
};

const ANSI_RESET_STATE: AnsiState = { bold: false, fg: null, bg: null };
const ANSI_SGR_PATTERN = /\x1b\[[0-9;]*m/g;
const TMP_TRIGGERS = normalizeTmpTriggers(tmpTriggersRaw);

function App() {
  const [sessionID, setSessionID] = useState("");
  const [connected, setConnected] = useState(false);
  const [queueDepth, setQueueDepth] = useState(0);
  const [queueMax, setQueueMax] = useState(20);
  const [inputValue, setInputValue] = useState("");
  const [terminalLines, setTerminalLines] = useState<string[]>([]);
  const [inlineStatus, setInlineStatus] = useState("Disconnected");
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [host, setHost] = useState(DEFAULT_HOST);
  const [port, setPort] = useState(DEFAULT_PORT);
  const [suggestion, setSuggestion] = useState<SuggestionView | null>(null);
  const [suggestionStatus, setSuggestionStatus] = useState<SuggestionStatus>("idle");
  const [suggestionError, setSuggestionError] = useState("");
  const [sendingCommand, setSendingCommand] = useState(false);
  const [isSuggestionOpen, setIsSuggestionOpen] = useState(false);
  const [suggestionsEnabled, setSuggestionsEnabled] = useState(true);
  const [suggestionCycle, setSuggestionCycle] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const terminalEndRef = useRef<HTMLDivElement | null>(null);
  const commandInputRef = useRef<HTMLInputElement | null>(null);
  const toastSeqRef = useRef(0);
  const suggestionRef = useRef<SuggestionView | null>(null);
  const triggerCommandsRef = useRef<string[]>([]);
  const processingTriggerQueueRef = useRef(false);

  useEffect(() => {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsURL = `${protocol}//${window.location.host}/api/v0/ws/terminal`;
    const ws = new WebSocket(wsURL);
    wsRef.current = ws;

    ws.onopen = () => {
      setInlineStatus("WebSocket connected");
    };

    ws.onclose = () => {
      setInlineStatus("WebSocket disconnected");
      setConnected(false);
    };

    ws.onerror = () => {
      pushToast("WebSocket error");
    };

    ws.onmessage = (event) => {
      let payload: TerminalEvent;
      try {
        payload = JSON.parse(event.data) as TerminalEvent;
      } catch {
        return;
      }

      if (payload.session_id) {
        setSessionID(payload.session_id);
      }
      if (typeof payload.queue_depth === "number") {
        setQueueDepth(payload.queue_depth);
      }
      if (typeof payload.queue_max === "number") {
        setQueueMax(payload.queue_max);
      }
      if (typeof payload.connected === "boolean") {
        setConnected(payload.connected);
      }

      switch (payload.event) {
        case "session.status":
          setInlineStatus(payload.connected ? "Session connected" : "Session disconnected");
          break;
        case "session.connected":
        case "session.disconnected":
          setInlineStatus(payload.text ?? payload.event);
          appendLine(`[${payload.event}] ${payload.text ?? ""}`);
          break;
        case "terminal.output":
          appendLine(payload.text ?? "");
          queueTemporaryTriggers(payload.text ?? "");
          break;
        case "queue.accepted":
          setInlineStatus(`Queued (${payload.queue_depth ?? 0}/${payload.queue_max ?? 20})`);
          appendLine(formatSentCommandEcho(payload.text ?? ""));
          break;
        case "queue.rejected":
          setInlineStatus(`Queue full (${payload.queue_depth ?? 0}/${payload.queue_max ?? 20})`);
          if (payload.queue_reject_code === ErrorBodyCode.QUEUE_FULL) {
            pushToast(`QUEUE_FULL: ${payload.queue_depth ?? 0}/${payload.queue_max ?? 20} pending`);
          }
          break;
        default:
          break;
      }
    };

    return () => {
      ws.close();
      wsRef.current = null;
    };
  }, []);

  useEffect(() => {
    terminalEndRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [terminalLines]);

  useEffect(() => {
    suggestionRef.current = suggestion;
  }, [suggestion]);

  useEffect(() => {
    if (!suggestionsEnabled) {
      setSuggestionStatus("idle");
      return;
    }

    let canceled = false;

    async function pollSuggestions(initial: boolean) {
      const currentSuggestion = suggestionRef.current;
      let staleTimeoutID: number | undefined;
      if (initial) {
        setSuggestionStatus(currentSuggestion ? "stale" : "loading");
      } else if (currentSuggestion) {
        staleTimeoutID = window.setTimeout(() => {
          if (!canceled) {
            setSuggestionStatus((current) => (current === "ready" ? "stale" : current));
          }
        }, 350);
      }

      try {
        const response = await getSuggestionsLatest();
        if (staleTimeoutID) {
          window.clearTimeout(staleTimeoutID);
        }
        if (canceled) {
          return;
        }

        if (response.status !== 200) {
          setSuggestionStatus("error");
          setSuggestionError(response.data.error?.message ?? "Failed to fetch suggestions");
          return;
        }

        const payload = response.data.data;
        if (!isSuggestionPayload(payload)) {
          setSuggestionStatus((current) =>
            currentSuggestion ? "stale" : current === "loading" ? "idle" : current
          );
          return;
        }

        setSuggestionError("");
        setSuggestion({
          commands: payload.commands,
          reason: payload.reason,
          expectedOutcome: payload.expected_outcome,
          generatedAt: payload.generated_at,
        });
        setSuggestionStatus("ready");
      } catch {
        if (staleTimeoutID) {
          window.clearTimeout(staleTimeoutID);
        }
        if (canceled) {
          return;
        }
        setSuggestionStatus("error");
        setSuggestionError("Suggestions unavailable");
      }
    }

    void pollSuggestions(true);
    const interval = window.setInterval(() => {
      void pollSuggestions(false);
    }, SUGGESTION_POLL_INTERVAL_MS);

    return () => {
      canceled = true;
      window.clearInterval(interval);
    };
  }, [suggestionCycle, suggestionsEnabled]);

  const queueLabel = useMemo(() => `${queueDepth}/${queueMax}`, [queueDepth, queueMax]);

  async function handleConnectClick() {
    const response = await postSessionConnect({ host, port });
    if (response.status !== 200) {
      pushToast("Session connect failed");
      return;
    }
    setConnected(true);
    setSessionID(response.data.data.session_id);
    setQueueDepth(response.data.data.queue_depth);
    setQueueMax(response.data.data.queue_max ?? queueMax);
    setInlineStatus(`Connected to ${host}:${port}`);
  }

  async function handleDisconnectClick() {
    const response = await postSessionDisconnect();
    if (response.status !== 200) {
      pushToast("Session disconnect failed");
      return;
    }
    setConnected(false);
    setQueueDepth(response.data.data.queue_depth);
    setQueueMax(response.data.data.queue_max ?? queueMax);
    setInlineStatus("Disconnected");
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const command = inputValue;
    if (sendingCommand) {
      return;
    }

    const sent = await enqueueCommand(command);
    if (sent) {
      setInputValue("");
    }

    window.requestAnimationFrame(() => {
      commandInputRef.current?.focus();
    });
  }

  async function handleSuggestionClick(command: string) {
    if (!connected || sendingCommand) {
      return;
    }
    const sent = await enqueueCommand(command);
    if (sent) {
      restartSuggestionCycle();
    }
  }

  function handleSuggestionRefuse() {
    restartSuggestionCycle();
  }

  function handleSuggestionToggle() {
    if (suggestionsEnabled) {
      setSuggestionsEnabled(false);
      setSuggestion(null);
      setSuggestionError("");
      setSuggestionStatus("idle");
      return;
    }

    setSuggestionsEnabled(true);
    setSuggestionStatus("loading");
    setSuggestionCycle((current) => current + 1);
  }

  function restartSuggestionCycle() {
    setSuggestion(null);
    setSuggestionError("");
    if (!suggestionsEnabled) {
      setSuggestionStatus("idle");
      return;
    }
    setSuggestionStatus("loading");
    setSuggestionCycle((current) => current + 1);
  }

  async function enqueueCommand(command: string): Promise<boolean> {
    setSendingCommand(true);
    try {
      const response = await postCommandsEnqueue({ command });
      if (response.status === 200) {
        setQueueDepth(response.data.data.queue_depth);
        setQueueMax(response.data.data.queue_max);
        setInlineStatus(`Queued (${response.data.data.queue_depth}/${response.data.data.queue_max})`);
        return true;
      }

      const queueFull = response.data.error?.code === ErrorBodyCode.QUEUE_FULL;
      if (queueFull) {
        const details = response.data.error?.details as
          | { queue_depth?: number; queue_max?: number }
          | undefined;
        const nextDepth = details?.queue_depth ?? queueDepth;
        const nextMax = details?.queue_max ?? queueMax;
        setQueueDepth(nextDepth);
        setQueueMax(nextMax);
        setInlineStatus(`Queue full (${nextDepth}/${nextMax})`);
        pushToast(`QUEUE_FULL: ${nextDepth}/${nextMax} pending`);
        return false;
      }

      pushToast(response.data.error?.message ?? "Failed to enqueue command");
      return false;
    } finally {
      setSendingCommand(false);
    }
  }

  function queueTemporaryTriggers(rawOutput: string) {
    const normalizedOutput = stripAnsi(rawOutput);
    if (!normalizedOutput) {
      return;
    }

    const matchedTriggers = TMP_TRIGGERS.filter((candidate) => normalizedOutput.includes(candidate.trigger));
    if (matchedTriggers.length === 0) {
      return;
    }

    for (const trigger of matchedTriggers) {
      for (const command of trigger.actions) {
        triggerCommandsRef.current.push(command);
      }
    }
    void flushTriggerQueue();
  }

  async function flushTriggerQueue(): Promise<void> {
    if (processingTriggerQueueRef.current) {
      return;
    }
    processingTriggerQueueRef.current = true;

    try {
      while (triggerCommandsRef.current.length > 0) {
        const nextCommand = triggerCommandsRef.current.shift();
        if (!nextCommand) {
          continue;
        }
        await enqueueCommandFromTrigger(nextCommand);
      }
    } finally {
      processingTriggerQueueRef.current = false;
    }
  }

  async function enqueueCommandFromTrigger(command: string): Promise<void> {
    const response = await postCommandsEnqueue({ command });
    if (response.status === 200) {
      setQueueDepth(response.data.data.queue_depth);
      setQueueMax(response.data.data.queue_max);
      setInlineStatus(`Trigger queued (${response.data.data.queue_depth}/${response.data.data.queue_max})`);
      return;
    }

    const queueFull = response.data.error?.code === ErrorBodyCode.QUEUE_FULL;
    if (queueFull) {
      const details = response.data.error?.details as
        | { queue_depth?: number; queue_max?: number }
        | undefined;
      const nextDepth = details?.queue_depth ?? queueDepth;
      const nextMax = details?.queue_max ?? queueMax;
      setQueueDepth(nextDepth);
      setQueueMax(nextMax);
      setInlineStatus(`Queue full (${nextDepth}/${nextMax})`);
      pushToast(`QUEUE_FULL: ${nextDepth}/${nextMax} pending`);
      return;
    }

    pushToast(response.data.error?.message ?? "Failed to enqueue trigger command");
  }

  function appendLine(line: string) {
    if (!line) {
      return;
    }

    setTerminalLines((current) => {
      const next = [...current, line];
      if (next.length > 300) {
        return next.slice(next.length - 300);
      }
      return next;
    });
  }

  function pushToast(message: string) {
    const id = toastSeqRef.current + 1;
    toastSeqRef.current = id;
    setToasts((current) => [...current, { id, message }]);

    window.setTimeout(() => {
      setToasts((current) => current.filter((toast) => toast.id !== id));
    }, 2800);
  }

  return (
    <main className="layout">
      <section className="terminal-pane">
        <header className="pane-header">
          <span>Arda Terminal</span>
          <span className="meta">{sessionID || "session pending"}</span>
        </header>
        <div className="terminal-body">
          {terminalLines.map((line, index) => (
            <div className="terminal-line" key={`${index}-${line.slice(0, 16)}`}>
              {renderAnsiLine(line)}
            </div>
          ))}
          <div ref={terminalEndRef} />
        </div>
        <form className="command-form" onSubmit={handleSubmit}>
          <input
            ref={commandInputRef}
            className="command-input"
            value={inputValue}
            onChange={(event) => setInputValue(event.target.value)}
            placeholder={connected ? "Type command or press Enter to send blank line" : "Connect session to send"}
            disabled={!connected || sendingCommand}
          />
          <button type="submit" disabled={!connected || sendingCommand}>
            Send
          </button>
        </form>
      </section>
      <aside className="map-pane">
        <header className="pane-header">Session</header>
        <div className="panel-body session-controls">
          <label>
            Host
            <input value={host} onChange={(event) => setHost(event.target.value)} disabled={connected} />
          </label>
          <label>
            Port
            <input
              value={port}
              onChange={(event) => setPort(Number(event.target.value) || 0)}
              disabled={connected}
            />
          </label>
          <div className="button-row">
            <button type="button" onClick={handleConnectClick} disabled={connected}>
              Connect
            </button>
            <button type="button" onClick={handleDisconnectClick} disabled={!connected}>
              Disconnect
            </button>
          </div>
        </div>
      </aside>
      <section className={`suggestion-pane ${isSuggestionOpen ? "open" : "collapsed"}`}>
        <header className="pane-header">
          <span>Suggestions + Queue</span>
          <div className="pane-header-actions">
            <span className="meta">Queue {queueLabel}</span>
            <button
              type="button"
              className="collapse-toggle"
              onClick={handleSuggestionToggle}
              aria-pressed={suggestionsEnabled}
            >
              {suggestionsEnabled ? "AI On" : "AI Off"}
            </button>
            <button
              type="button"
              className="collapse-toggle"
              onClick={() => setIsSuggestionOpen((current) => !current)}
              aria-expanded={isSuggestionOpen}
            >
              {isSuggestionOpen ? "Collapse" : "Expand"}
            </button>
          </div>
        </header>
        {isSuggestionOpen && (
          <div className="panel-body queue-panel">
            <div>Connection: {connected ? "connected" : "disconnected"}</div>
            <div>Queue: {queueLabel}</div>
            <div className={inlineStatus.includes("Queue full") ? "inline-status error" : "inline-status"}>
              {inlineStatus}
            </div>

            <div className="suggestion-status" data-testid="suggestion-status">
              {renderSuggestionStatus(suggestionsEnabled, suggestionStatus, suggestionError, suggestion)}
            </div>

            {suggestion && (
              <div className="suggestion-block">
                <div className="suggestion-reason">Reason: {suggestion.reason}</div>
                <div className="suggestion-outcome">Expected: {suggestion.expectedOutcome}</div>
                <div className="suggestion-meta">
                  Generated: {new Date(suggestion.generatedAt).toLocaleTimeString()}
                </div>
                <div className="suggestion-actions">
                  {suggestion.commands.map((command, index) => (
                    <button
                      key={`${command}-${index}`}
                      type="button"
                      className="suggestion-button"
                      onClick={() => handleSuggestionClick(command)}
                      disabled={!connected || sendingCommand}
                    >
                      {command}
                    </button>
                  ))}
                  <button
                    type="button"
                    className="suggestion-button"
                    onClick={handleSuggestionRefuse}
                    disabled={sendingCommand}
                  >
                    Refuse
                  </button>
                </div>
              </div>
            )}

            <div className="toast-stack">
              {toasts.map((toast) => (
                <div key={toast.id} className="toast">
                  {toast.message}
                </div>
              ))}
            </div>
          </div>
        )}
      </section>
    </main>
  );
}

function renderSuggestionStatus(
  enabled: boolean,
  status: SuggestionStatus,
  error: string,
  suggestion: SuggestionView | null
): string {
  if (!enabled) {
    return "Suggestions disabled";
  }
  if (status === "loading") {
    return "Suggestions loading...";
  }
  if (status === "stale") {
    return suggestion ? "Refreshing suggestions... showing latest stable set." : "Suggestions warming up...";
  }
  if (status === "error") {
    return error || "Suggestions unavailable";
  }
  if (status === "ready") {
    return "Suggestions ready";
  }
  return "No suggestions yet";
}

function formatSentCommandEcho(command: string): string {
  if (command.length === 0) {
    return ">";
  }
  return `> ${command}`;
}

function renderAnsiLine(line: string): ReactNode {
  const matches = [...line.matchAll(/\x1b\[([0-9;]*)m/g)];
  if (matches.length === 0) {
    return line;
  }

  const parts: ReactNode[] = [];
  let cursor = 0;
  let state: AnsiState = { ...ANSI_RESET_STATE };

  for (let i = 0; i < matches.length; i += 1) {
    const match = matches[i];
    const matchText = match[0];
    const matchIndex = match.index ?? -1;
    if (matchIndex < cursor) {
      continue;
    }

    const chunk = line.slice(cursor, matchIndex);
    if (chunk) {
      parts.push(
        <span key={`chunk-${i}-${cursor}`} style={ansiStateToStyle(state)}>
          {chunk}
        </span>
      );
    }

    state = applyAnsiCodes(state, match[1] ?? "");
    cursor = matchIndex + matchText.length;
  }

  const tail = line.slice(cursor);
  if (tail) {
    parts.push(
      <span key={`tail-${cursor}`} style={ansiStateToStyle(state)}>
        {tail}
      </span>
    );
  }

  return <>{parts}</>;
}

function applyAnsiCodes(current: AnsiState, codesRaw: string): AnsiState {
  const next: AnsiState = { ...current };
  const codes =
    codesRaw.trim().length === 0
      ? [0]
      : codesRaw
          .split(";")
          .map((part) => Number(part))
          .filter((code) => Number.isInteger(code));

  for (const code of codes) {
    if (code === 0) {
      next.bold = false;
      next.fg = null;
      next.bg = null;
      continue;
    }

    if (code === 1) {
      next.bold = true;
      continue;
    }

    if (code === 22) {
      next.bold = false;
      continue;
    }

    if (code === 39) {
      next.fg = null;
      continue;
    }

    if (code === 49) {
      next.bg = null;
      continue;
    }

    if (ANSI_FG_COLORS[code]) {
      next.fg = ANSI_FG_COLORS[code];
      continue;
    }

    if (ANSI_BG_COLORS[code]) {
      next.bg = ANSI_BG_COLORS[code];
    }
  }

  return next;
}

function ansiStateToStyle(state: AnsiState): CSSProperties {
  return {
    color: state.fg ?? undefined,
    backgroundColor: state.bg ?? undefined,
    fontWeight: state.bold ? 700 : undefined,
  };
}

function isSuggestionPayload(value: unknown): value is {
  commands: string[];
  reason: string;
  expected_outcome: string;
  generated_at: string;
} {
  if (!value || typeof value !== "object") {
    return false;
  }

  const candidate = value as {
    commands?: unknown;
    reason?: unknown;
    expected_outcome?: unknown;
    generated_at?: unknown;
  };

  return (
    Array.isArray(candidate.commands) &&
    candidate.commands.every((item) => typeof item === "string") &&
    typeof candidate.reason === "string" &&
    typeof candidate.expected_outcome === "string" &&
    typeof candidate.generated_at === "string"
  );
}

export default App;

function normalizeTmpTriggers(rawValue: unknown): TmpTrigger[] {
  if (!rawValue || typeof rawValue !== "object") {
    return [];
  }

  const root = rawValue as { triggers?: unknown };
  if (!Array.isArray(root.triggers)) {
    return [];
  }

  return root.triggers.flatMap((entry): TmpTrigger[] => {
    if (!entry || typeof entry !== "object") {
      return [];
    }

    const trigger = entry as { id?: unknown; trigger?: unknown; actions?: unknown };
    if (
      typeof trigger.id !== "string" ||
      typeof trigger.trigger !== "string" ||
      !Array.isArray(trigger.actions) ||
      !trigger.actions.every((action) => typeof action === "string")
    ) {
      return [];
    }

    const trimmedNeedle = trigger.trigger.trim();
    const filteredActions = trigger.actions.map((action) => action.trim()).filter((action) => action.length > 0);
    if (trimmedNeedle.length === 0 || filteredActions.length === 0) {
      return [];
    }

    return [{ id: trigger.id, trigger: trimmedNeedle, actions: filteredActions }];
  });
}

function stripAnsi(line: string): string {
  return line.replace(ANSI_SGR_PATTERN, "");
}
