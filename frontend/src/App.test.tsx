import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";

vi.mock("./lib/api/generated/gateway", () => ({
  getSuggestionsLatest: vi.fn(),
  postCommandsEnqueue: vi.fn(),
  postSessionConnect: vi.fn(),
  postSessionDisconnect: vi.fn(),
}));

import {
  getSuggestionsLatest,
  postCommandsEnqueue,
  postSessionConnect,
  postSessionDisconnect,
} from "./lib/api/generated/gateway";

class MockWebSocket {
  static instances: MockWebSocket[] = [];

  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;

  constructor(_url: string) {
    MockWebSocket.instances.push(this);
  }

  close() {
    this.onclose?.(new CloseEvent("close"));
  }

  emitMessage(payload: object) {
    this.onmessage?.(
      new MessageEvent("message", {
        data: JSON.stringify(payload),
      })
    );
  }
}

describe("App suggestions", () => {
  beforeEach(() => {
    MockWebSocket.instances = [];
    vi.stubGlobal("WebSocket", MockWebSocket as unknown as typeof WebSocket);
    vi.spyOn(window, "setInterval").mockImplementation(
      (() => ({}) as unknown as ReturnType<typeof setInterval>)
    );

    vi.mocked(postSessionConnect).mockResolvedValue({
      status: 200,
      data: {
        data: { session_id: "s-1", connected: true, queue_depth: 0, queue_max: 20 },
        meta: { request_id: "r-1", timestamp: "2026-03-05T00:00:00Z" },
      },
      headers: new Headers(),
    });

    vi.mocked(postSessionDisconnect).mockResolvedValue({
      status: 200,
      data: {
        data: { session_id: "s-1", connected: false, queue_depth: 0, queue_max: 20 },
        meta: { request_id: "r-2", timestamp: "2026-03-05T00:00:00Z" },
      },
      headers: new Headers(),
    });

    vi.mocked(postCommandsEnqueue).mockResolvedValue({
      status: 200,
      data: {
        data: { accepted: true, queue_depth: 1, queue_max: 20 },
        meta: { request_id: "r-3", timestamp: "2026-03-05T00:00:00Z" },
      },
      headers: new Headers(),
    });

    vi.mocked(getSuggestionsLatest).mockResolvedValue({
      status: 200,
      data: {
        data: {
          commands: ["look", "score"],
          reason: "Scan room context",
          expected_outcome: "Understand immediate options",
          generated_at: "2026-03-05T10:15:00Z",
        },
        meta: { request_id: "r-4", timestamp: "2026-03-05T10:15:00Z" },
      },
      headers: new Headers(),
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("renders suggestion reason/outcome and command buttons", async () => {
    render(<App />);

    const expandButton = await screen.findByRole("button", { name: "Expand" });
    const user = userEvent.setup();
    await user.click(expandButton);

    expect(await screen.findByText("Reason: Scan room context")).toBeInTheDocument();
    expect(screen.getByText("Expected: Understand immediate options")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "look" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "score" })).toBeInTheDocument();
  });

  it("enqueues selected suggestion command when clicked", async () => {
    render(<App />);

    const ws = MockWebSocket.instances[0];
    act(() => {
      ws.emitMessage({
        event: "session.status",
        session_id: "s-1",
        connected: true,
        queue_depth: 0,
        queue_max: 20,
      });
    });

    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "Expand" }));
    const lookButton = await screen.findByRole("button", { name: "look" });
    await waitFor(() => expect(lookButton).toBeEnabled());
    await user.click(lookButton);

    expect(postCommandsEnqueue).toHaveBeenCalledWith({ command: "look" });
  });

  it("keeps suggestions panel collapsed by default", async () => {
    render(<App />);
    await waitFor(() => expect(getSuggestionsLatest).toHaveBeenCalled());
    expect(screen.getByRole("button", { name: "Expand" })).toBeInTheDocument();
    expect(screen.queryByText("Reason: Scan room context")).not.toBeInTheDocument();
  });

  it("submits empty command when Enter is pressed on blank input", async () => {
    render(<App />);

    const ws = MockWebSocket.instances[0];
    act(() => {
      ws.emitMessage({
        event: "session.status",
        session_id: "s-1",
        connected: true,
        queue_depth: 0,
        queue_max: 20,
      });
    });

    const input = screen.getByPlaceholderText("Type command or press Enter to send blank line");
    await waitFor(() => expect(input).toBeEnabled());

    const user = userEvent.setup();
    await user.type(input, "{enter}");

    expect(postCommandsEnqueue).toHaveBeenCalledWith({ command: "" });
    await waitFor(() => expect(input).toHaveFocus());
  });

  it("renders ANSI colored terminal output", async () => {
    render(<App />);

    const ws = MockWebSocket.instances[0];
    act(() => {
      ws.emitMessage({
        event: "terminal.output",
        session_id: "s-1",
        text: "\u001b[1;36mColor line\u001b[0m plain",
      });
    });

    const colored = await screen.findByText("Color line");
    expect(colored).toHaveStyle({ color: "#56b6c2", fontWeight: "700" });
    expect(screen.getByText((content) => content.includes("plain"))).toBeInTheDocument();
  });

  it("echoes sent commands to the terminal on queue accepted event", async () => {
    render(<App />);

    const ws = MockWebSocket.instances[0];
    act(() => {
      ws.emitMessage({
        event: "queue.accepted",
        session_id: "s-1",
        text: "look",
        queue_depth: 1,
        queue_max: 20,
      });
    });

    expect(await screen.findByText("> look")).toBeInTheDocument();
  });
});
