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
	listener net.Listener
	commands chan string
	connMu   sync.Mutex
	conn     net.Conn
	wg       sync.WaitGroup
}

func newFakeUpstream(t *testing.T) *fakeUpstream {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake upstream: %v", err)
	}

	f := &fakeUpstream{
		listener: listener,
		commands: make(chan string, 8),
	}
	f.wg.Add(1)
	go f.run()
	return f
}

func (f *fakeUpstream) run() {
	defer f.wg.Done()

	conn, err := f.listener.Accept()
	if err != nil {
		return
	}
	f.connMu.Lock()
	f.conn = conn
	f.connMu.Unlock()
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.TrimSpace(line)
		f.commands <- command
		encoder := charmap.Windows1251.NewEncoder()
		payload, err := encoder.Bytes([]byte("вы видите землю\n"))
		if err != nil {
			return
		}
		_, _ = conn.Write(payload)
	}
}

func (f *fakeUpstream) Port() int {
	_, portStr, _ := net.SplitHostPort(f.listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return port
}

func (f *fakeUpstream) WaitCommand(t *testing.T, timeout time.Duration) string {
	t.Helper()

	select {
	case cmd := <-f.commands:
		return cmd
	case <-time.After(timeout):
		t.Fatal("timed out waiting for upstream command")
		return ""
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
