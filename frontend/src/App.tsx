import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import {
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

const DEFAULT_HOST = "86.110.194.3";
const DEFAULT_PORT = 7000;

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

  const wsRef = useRef<WebSocket | null>(null);
  const terminalEndRef = useRef<HTMLDivElement | null>(null);
  const toastSeqRef = useRef(0);

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
          setInlineStatus(
            `Queue full (${payload.queue_depth ?? 0}/${payload.queue_max ?? 20})`
          );
          if (payload.queue_reject_code === ErrorBodyCode.QUEUE_FULL) {
            pushToast(
              `QUEUE_FULL: ${payload.queue_depth ?? 0}/${payload.queue_max ?? 20} pending`
            );
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
    if (!command) {
      return;
    }

    const response = await postCommandsEnqueue({ command });
    if (response.status === 200) {
      setQueueDepth(response.data.data.queue_depth);
      setQueueMax(response.data.data.queue_max);
      setInputValue("");
      setInlineStatus(`Queued (${response.data.data.queue_depth}/${response.data.data.queue_max})`);
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

    pushToast(response.data.error?.message ?? "Failed to enqueue command");
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
            disabled={!connected}
          />
          <button type="submit" disabled={!connected}>
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
        <header className="pane-header">Queue + Notices</header>
        <div className="panel-body queue-panel">
          <div>Connection: {connected ? "connected" : "disconnected"}</div>
          <div>Queue: {queueLabel}</div>
          <div className={inlineStatus.includes("Queue full") ? "inline-status error" : "inline-status"}>
            {inlineStatus}
          </div>
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

export default App;
