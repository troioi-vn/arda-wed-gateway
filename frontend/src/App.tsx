import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import {
  getSuggestionsLatest,
  postCommandsEnqueue,
  postSessionConnect,
  postSessionDisconnect,
} from "./lib/api/generated/gateway";
import { ErrorBodyCode } from "./lib/api/generated/model";

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

const DEFAULT_HOST = "86.110.194.3";
const DEFAULT_PORT = 7000;
const SUGGESTION_POLL_INTERVAL_MS = 1600;

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

  const wsRef = useRef<WebSocket | null>(null);
  const terminalEndRef = useRef<HTMLDivElement | null>(null);
  const toastSeqRef = useRef(0);
  const suggestionRef = useRef<SuggestionView | null>(null);

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
          break;
        case "queue.accepted":
          setInlineStatus(`Queued (${payload.queue_depth ?? 0}/${payload.queue_max ?? 20})`);
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
  }, []);

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
    const command = inputValue.trim();
    if (!command || sendingCommand) {
      return;
    }

    const sent = await enqueueCommand(command);
    if (sent) {
      setInputValue("");
    }
  }

  async function handleSuggestionClick(command: string) {
    if (!connected || sendingCommand) {
      return;
    }
    await enqueueCommand(command);
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
              {line}
            </div>
          ))}
          <div ref={terminalEndRef} />
        </div>
        <form className="command-form" onSubmit={handleSubmit}>
          <input
            className="command-input"
            value={inputValue}
            onChange={(event) => setInputValue(event.target.value)}
            placeholder={connected ? "Type command and press Enter" : "Connect session to send"}
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
      <section className="suggestion-pane">
        <header className="pane-header">Suggestions + Queue</header>
        <div className="panel-body queue-panel">
          <div>Connection: {connected ? "connected" : "disconnected"}</div>
          <div>Queue: {queueLabel}</div>
          <div className={inlineStatus.includes("Queue full") ? "inline-status error" : "inline-status"}>
            {inlineStatus}
          </div>

          <div className="suggestion-status" data-testid="suggestion-status">
            {renderSuggestionStatus(suggestionStatus, suggestionError, suggestion)}
          </div>

          {suggestion && (
            <div className="suggestion-block">
              <div className="suggestion-reason">Reason: {suggestion.reason}</div>
              <div className="suggestion-outcome">Expected: {suggestion.expectedOutcome}</div>
              <div className="suggestion-meta">Generated: {new Date(suggestion.generatedAt).toLocaleTimeString()}</div>
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
      </section>
    </main>
  );
}

function renderSuggestionStatus(
  status: SuggestionStatus,
  error: string,
  suggestion: SuggestionView | null
): string {
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
