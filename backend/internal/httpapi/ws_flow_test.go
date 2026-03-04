package httpapi

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/athanasius/arda-web-gateway/backend/internal/config"
	"golang.org/x/text/encoding/charmap"
)

type terminalEvent struct {
	Event      string `json:"event"`
	SessionID  string `json:"session_id"`
	Connected  bool   `json:"connected"`
	Text       string `json:"text"`
	QueueDepth int    `json:"queue_depth"`
	QueueMax   int    `json:"queue_max"`
}

type upstreamCommand struct {
	command string
	at      time.Time
}

func TestWebSocketGatewayFlowSmoke(t *testing.T) {
	t.Parallel()

	upstream := newFakeUpstream(t)
	defer upstream.Close()

	handler := NewRouter(config.Config{
		Host:              "127.0.0.1",
		Port:              "8080",
		BuildVersion:      "test",
		BuildCommit:       "abc123",
		QueueSendInterval: 25 * time.Millisecond,
		QueueMaxDepth:     20,
	}, slog.Default())

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()

	baseURL := "http://" + listener.Addr().String()

	conn := dialWebSocket(t, baseURL, "/api/v0/ws/terminal")
	defer conn.Close()

	first := readEvent(t, conn)
	if first.Event != "session.status" {
		t.Fatalf("expected first event session.status, got %q", first.Event)
	}

	status := postJSON(t, baseURL+"/api/v0/session/connect", map[string]any{
		"host": "127.0.0.1",
		"port": upstream.Port(),
	})
	if status != http.StatusOK {
		t.Fatalf("connect status: expected 200, got %d", status)
	}

	connectedEvent := readUntilEvent(t, conn, "session.connected", 2*time.Second)
	if !strings.Contains(connectedEvent.Text, "connected to") {
		t.Fatalf("unexpected connected text: %q", connectedEvent.Text)
	}

	status = postJSON(t, baseURL+"/api/v0/commands/enqueue", map[string]any{
		"command": "look",
	})
	if status != http.StatusOK {
		t.Fatalf("enqueue status: expected 200, got %d", status)
	}

	acceptedEvent := readUntilEvent(t, conn, "queue.accepted", 2*time.Second)
	if acceptedEvent.QueueDepth < 1 {
		t.Fatalf("expected accepted event queue depth to be set")
	}

	gotCommand := upstream.WaitCommand(t, 2*time.Second)
	if gotCommand != "look" {
		t.Fatalf("expected upstream command look, got %q", gotCommand)
	}

	outputEvent := readUntilEvent(t, conn, "terminal.output", 2*time.Second)
	if !strings.Contains(outputEvent.Text, "землю") {
		t.Fatalf("expected terminal output from upstream, got %q", outputEvent.Text)
	}

	status = postJSON(t, baseURL+"/api/v0/session/disconnect", map[string]any{})
	if status != http.StatusOK {
		t.Fatalf("disconnect status: expected 200, got %d", status)
	}

	_ = readUntilEvent(t, conn, "session.disconnected", 2*time.Second)
}

func TestWebSocketGatewayAcceptsBlankEnterCommand(t *testing.T) {
	t.Parallel()

	upstream := newFakeUpstream(t)
	defer upstream.Close()

	handler := NewRouter(config.Config{
		Host:              "127.0.0.1",
		Port:              "8080",
		BuildVersion:      "test",
		BuildCommit:       "abc123",
		QueueSendInterval: 10 * time.Millisecond,
		QueueMaxDepth:     20,
	}, slog.Default())

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()

	baseURL := "http://" + listener.Addr().String()

	conn := dialWebSocket(t, baseURL, "/api/v0/ws/terminal")
	defer conn.Close()
	_ = readEvent(t, conn)

	status := postJSON(t, baseURL+"/api/v0/session/connect", map[string]any{
		"host": "127.0.0.1",
		"port": upstream.Port(),
	})
	if status != http.StatusOK {
		t.Fatalf("connect status: expected 200, got %d", status)
	}
	_ = readUntilEvent(t, conn, "session.connected", 2*time.Second)

	status = postJSON(t, baseURL+"/api/v0/commands/enqueue", map[string]any{
		"command": "",
	})
	if status != http.StatusOK {
		t.Fatalf("enqueue status for blank command: expected 200, got %d", status)
	}
	_ = readUntilEvent(t, conn, "queue.accepted", 2*time.Second)

	gotCommand := upstream.WaitCommand(t, 2*time.Second)
	if gotCommand != "" {
		t.Fatalf("expected blank upstream command, got %q", gotCommand)
	}
}

func TestWebSocketGatewayDecodesCP1251Output(t *testing.T) {
	t.Parallel()

	upstream := newFakeUpstream(t)
	defer upstream.Close()

	handler := NewRouter(config.Config{
		Host:              "127.0.0.1",
		Port:              "8080",
		BuildVersion:      "test",
		BuildCommit:       "abc123",
		QueueSendInterval: 10 * time.Millisecond,
		QueueMaxDepth:     20,
	}, slog.Default())

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()

	baseURL := "http://" + listener.Addr().String()
	conn := dialWebSocket(t, baseURL, "/api/v0/ws/terminal")
	defer conn.Close()
	_ = readEvent(t, conn)

	status := postJSON(t, baseURL+"/api/v0/session/connect", map[string]any{
		"host": "127.0.0.1",
		"port": upstream.Port(),
	})
	if status != http.StatusOK {
		t.Fatalf("connect status: expected 200, got %d", status)
	}
	_ = readUntilEvent(t, conn, "session.connected", 2*time.Second)

	status = postJSON(t, baseURL+"/api/v0/commands/enqueue", map[string]any{
		"command": "look",
	})
	if status != http.StatusOK {
		t.Fatalf("enqueue status: expected 200, got %d", status)
	}
	_ = readUntilEvent(t, conn, "queue.accepted", 2*time.Second)

	output := readUntilEvent(t, conn, "terminal.output", 2*time.Second)
	if !strings.Contains(output.Text, "землю") {
		t.Fatalf("expected decoded cp1251 text, got %q", output.Text)
	}

	status = postJSON(t, baseURL+"/api/v0/session/disconnect", map[string]any{})
	if status != http.StatusOK {
		t.Fatalf("disconnect status: expected 200, got %d", status)
	}
}

func TestWebSocketGatewayEncodesCP1251Input(t *testing.T) {
	t.Parallel()

	upstream := newFakeUpstream(t)
	defer upstream.Close()

	handler := NewRouter(config.Config{
		Host:              "127.0.0.1",
		Port:              "8080",
		BuildVersion:      "test",
		BuildCommit:       "abc123",
		QueueSendInterval: 10 * time.Millisecond,
		QueueMaxDepth:     20,
	}, slog.Default())

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()

	baseURL := "http://" + listener.Addr().String()
	conn := dialWebSocket(t, baseURL, "/api/v0/ws/terminal")
	defer conn.Close()
	_ = readEvent(t, conn)

	status := postJSON(t, baseURL+"/api/v0/session/connect", map[string]any{
		"host": "127.0.0.1",
		"port": upstream.Port(),
	})
	if status != http.StatusOK {
		t.Fatalf("connect status: expected 200, got %d", status)
	}
	_ = readUntilEvent(t, conn, "session.connected", 2*time.Second)

	status = postJSON(t, baseURL+"/api/v0/commands/enqueue", map[string]any{
		"command": "справка",
	})
	if status != http.StatusOK {
		t.Fatalf("enqueue status: expected 200, got %d", status)
	}
	_ = readUntilEvent(t, conn, "queue.accepted", 2*time.Second)

	gotCommand := upstream.WaitCommand(t, 2*time.Second)
	if gotCommand != "справка" {
		t.Fatalf("expected cp1251-encoded upstream command, got %q", gotCommand)
	}
}

func TestWebSocketQueueBurstPacingAndRejects(t *testing.T) {
	t.Parallel()

	const (
		queueInterval = 120 * time.Millisecond
		queueMaxDepth = 3
		totalBurst    = 10
	)

	upstream := newFakeUpstream(t)
	defer upstream.Close()

	handler := NewRouter(config.Config{
		Host:              "127.0.0.1",
		Port:              "8080",
		BuildVersion:      "test",
		BuildCommit:       "abc123",
		QueueSendInterval: queueInterval,
		QueueMaxDepth:     queueMaxDepth,
	}, slog.Default())

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()

	baseURL := "http://" + listener.Addr().String()
	conn := dialWebSocket(t, baseURL, "/api/v0/ws/terminal")
	defer conn.Close()
	_ = readEvent(t, conn)

	status := postJSON(t, baseURL+"/api/v0/session/connect", map[string]any{
		"host": "127.0.0.1",
		"port": upstream.Port(),
	})
	if status != http.StatusOK {
		t.Fatalf("connect status: expected 200, got %d", status)
	}
	_ = readUntilEvent(t, conn, "session.connected", 2*time.Second)

	accepted := 0
	rejected := 0
	for i := 0; i < totalBurst; i++ {
		command := fmt.Sprintf("cmd-%d", i)
		status = postJSON(t, baseURL+"/api/v0/commands/enqueue", map[string]any{"command": command})
		switch status {
		case http.StatusOK:
			accepted++
		case http.StatusTooManyRequests:
			rejected++
		default:
			t.Fatalf("unexpected enqueue status for %s: %d", command, status)
		}
	}

	if accepted != queueMaxDepth {
		t.Fatalf("expected %d accepted commands before queue filled, got %d", queueMaxDepth, accepted)
	}
	if rejected != totalBurst-queueMaxDepth {
		t.Fatalf("expected %d rejected commands, got %d", totalBurst-queueMaxDepth, rejected)
	}

	sent := make([]upstreamCommand, 0, queueMaxDepth)
	for i := 0; i < queueMaxDepth; i++ {
		sent = append(sent, upstream.WaitCommandWithTimestamp(t, 2*time.Second))
	}

	for i := 0; i < queueMaxDepth; i++ {
		want := fmt.Sprintf("cmd-%d", i)
		if sent[i].command != want {
			t.Fatalf("unexpected upstream command order at %d: want %q, got %q", i, want, sent[i].command)
		}
	}

	for i := 1; i < len(sent); i++ {
		delta := sent[i].at.Sub(sent[i-1].at)
		if delta < queueInterval-20*time.Millisecond {
			t.Fatalf("queue pacing too fast at index %d: delta=%s interval=%s", i, delta, queueInterval)
		}
	}
}

func TestWebSocketReconnectDoesNotReplayDroppedCommands(t *testing.T) {
	t.Parallel()

	upstream := newFakeUpstream(t)
	defer upstream.Close()

	handler := NewRouter(config.Config{
		Host:              "127.0.0.1",
		Port:              "8080",
		BuildVersion:      "test",
		BuildCommit:       "abc123",
		QueueSendInterval: 300 * time.Millisecond,
		QueueMaxDepth:     20,
	}, slog.Default())

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()

	baseURL := "http://" + listener.Addr().String()
	conn := dialWebSocket(t, baseURL, "/api/v0/ws/terminal")
	defer conn.Close()
	_ = readEvent(t, conn)

	status := postJSON(t, baseURL+"/api/v0/session/connect", map[string]any{
		"host": "127.0.0.1",
		"port": upstream.Port(),
	})
	if status != http.StatusOK {
		t.Fatalf("connect status: expected 200, got %d", status)
	}
	_ = readUntilEvent(t, conn, "session.connected", 2*time.Second)

	if status = postJSON(t, baseURL+"/api/v0/commands/enqueue", map[string]any{"command": "first"}); status != http.StatusOK {
		t.Fatalf("enqueue first status: expected 200, got %d", status)
	}
	if status = postJSON(t, baseURL+"/api/v0/commands/enqueue", map[string]any{"command": "second"}); status != http.StatusOK {
		t.Fatalf("enqueue second status: expected 200, got %d", status)
	}

	if got := upstream.WaitCommand(t, 2*time.Second); got != "first" {
		t.Fatalf("expected first upstream command, got %q", got)
	}

	status = postJSON(t, baseURL+"/api/v0/session/disconnect", map[string]any{})
	if status != http.StatusOK {
		t.Fatalf("disconnect status: expected 200, got %d", status)
	}

	disconnected := readUntilEvent(t, conn, "session.disconnected", 2*time.Second)
	if !strings.Contains(disconnected.Text, "dropped_unsent=1") {
		t.Fatalf("expected dropped_unsent=1 in disconnect event, got %q", disconnected.Text)
	}

	status = postJSON(t, baseURL+"/api/v0/session/connect", map[string]any{
		"host": "127.0.0.1",
		"port": upstream.Port(),
	})
	if status != http.StatusOK {
		t.Fatalf("reconnect status: expected 200, got %d", status)
	}
	_ = readUntilEvent(t, conn, "session.connected", 2*time.Second)

	upstream.AssertNoCommand(t, 450*time.Millisecond)

	if status = postJSON(t, baseURL+"/api/v0/commands/enqueue", map[string]any{"command": "third"}); status != http.StatusOK {
		t.Fatalf("enqueue third status: expected 200, got %d", status)
	}
	if got := upstream.WaitCommand(t, 2*time.Second); got != "third" {
		t.Fatalf("expected third upstream command after reconnect, got %q", got)
	}
}

func readUntilEvent(t *testing.T, conn net.Conn, eventType string, timeout time.Duration) terminalEvent {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		event := readEvent(t, conn)
		if event.Event == eventType {
			return event
		}
	}

	t.Fatalf("event %q not received before timeout", eventType)
	return terminalEvent{}
}

func readEvent(t *testing.T, conn net.Conn) terminalEvent {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	payload := readServerFrame(t, conn)
	var event terminalEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("unmarshal websocket event: %v", err)
	}
	return event
}

func dialWebSocket(t *testing.T, baseURL, path string) net.Conn {
	t.Helper()

	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("random key: %v", err)
	}
	secKey := base64.StdEncoding.EncodeToString(keyBytes)

	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n\r\n", path, u.Host, secKey)
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write handshake request: %v", err)
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read handshake status line: %v", err)
	}
	if !strings.Contains(statusLine, "101") {
		t.Fatalf("expected 101 handshake response, got %q", statusLine)
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read handshake header: %v", err)
		}
		if line == "\r\n" {
			break
		}
	}

	return &prefixedConn{Conn: conn, reader: reader}
}

func readServerFrame(t *testing.T, conn net.Conn) []byte {
	t.Helper()

	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("read frame header: %v", err)
	}

	payloadLen := int(header[1] & 0x7F)
	switch payloadLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(conn, ext); err != nil {
			t.Fatalf("read extended payload length (16): %v", err)
		}
		payloadLen = int(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(conn, ext); err != nil {
			t.Fatalf("read extended payload length (64): %v", err)
		}
		payloadLen = int(binary.BigEndian.Uint64(ext))
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatalf("read frame payload: %v", err)
	}
	return payload
}

func postJSON(t *testing.T, url string, payload map[string]any) int {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	res, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post request: %v", err)
	}
	defer res.Body.Close()
	return res.StatusCode
}

type prefixedConn struct {
	net.Conn
	reader *bufio.Reader
}

type fakeUpstream struct {
	listener        net.Listener
	commands        chan upstreamCommand
	connMu          sync.Mutex
	conn            net.Conn
	respMu          sync.RWMutex
	responses       map[string]string
	defaultResponse string
	wg              sync.WaitGroup
}

func newFakeUpstream(t *testing.T) *fakeUpstream {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake upstream: %v", err)
	}

	f := &fakeUpstream{
		listener:        listener,
		commands:        make(chan upstreamCommand, 32),
		responses:       map[string]string{},
		defaultResponse: "вы видите землю\n",
	}
	f.wg.Add(1)
	go f.run()
	return f
}

func (f *fakeUpstream) run() {
	defer f.wg.Done()

	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}

		f.connMu.Lock()
		f.conn = conn
		f.connMu.Unlock()

		f.handleConn(conn)
	}
}

func (f *fakeUpstream) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		decodedLine, err := charmap.Windows1251.NewDecoder().Bytes([]byte(line))
		if err != nil {
			decodedLine = []byte(line)
		}

		command := strings.TrimSpace(string(decodedLine))
		f.commands <- upstreamCommand{command: command, at: time.Now()}

		response := f.responseForCommand(command)
		if response == "" {
			continue
		}

		encoder := charmap.Windows1251.NewEncoder()
		payload, err := encoder.Bytes([]byte(response))
		if err != nil {
			return
		}
		if _, err := conn.Write(payload); err != nil {
			return
		}
	}
}

func (f *fakeUpstream) responseForCommand(command string) string {
	f.respMu.RLock()
	defer f.respMu.RUnlock()
	if response, ok := f.responses[command]; ok {
		return response
	}
	return f.defaultResponse
}

func (f *fakeUpstream) SetResponse(command, response string) {
	f.respMu.Lock()
	defer f.respMu.Unlock()
	f.responses[command] = response
}

func (f *fakeUpstream) Port() int {
	_, portStr, _ := net.SplitHostPort(f.listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return port
}

func (f *fakeUpstream) WaitCommand(t *testing.T, timeout time.Duration) string {
	t.Helper()

	return f.WaitCommandWithTimestamp(t, timeout).command
}

func (f *fakeUpstream) WaitCommandWithTimestamp(t *testing.T, timeout time.Duration) upstreamCommand {
	t.Helper()

	select {
	case cmd := <-f.commands:
		return cmd
	case <-time.After(timeout):
		t.Fatal("timed out waiting for upstream command")
		return upstreamCommand{}
	}
}

func (f *fakeUpstream) AssertNoCommand(t *testing.T, timeout time.Duration) {
	t.Helper()

	select {
	case cmd := <-f.commands:
		t.Fatalf("expected no upstream command within %s, but received %q", timeout, cmd.command)
	case <-time.After(timeout):
		return
	}
}

func (f *fakeUpstream) Close() {
	f.connMu.Lock()
	if f.conn != nil {
		_ = f.conn.Close()
	}
	f.connMu.Unlock()
	_ = f.listener.Close()
	f.wg.Wait()
}

func (c *prefixedConn) Read(p []byte) (int, error) {
	if c.reader != nil {
		if c.reader.Buffered() == 0 {
			c.reader = nil
		} else {
			return c.reader.Read(p)
		}
	}
	return c.Conn.Read(p)
}
