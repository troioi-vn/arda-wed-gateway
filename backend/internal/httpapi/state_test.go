package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/athanasius/arda-web-gateway/backend/internal/config"
)

type stateSnapshotResponse struct {
	Data struct {
		Location string `json:"location"`
		Prompt   struct {
			HPCurrent int `json:"hp_current"`
			HPMax     int `json:"hp_max"`
			MACurrent int `json:"ma_current"`
			MAMax     int `json:"ma_max"`
			MVCurrent int `json:"mv_current"`
			MVMax     int `json:"mv_max"`
			EXP       int `json:"exp"`
		} `json:"prompt"`
	} `json:"data"`
}

func TestStateSnapshotEndpointUsesPersistedState(t *testing.T) {
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
		SQLitePath:        t.TempDir() + "/gateway.sqlite",
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

	status = postJSON(t, baseURL+"/api/v0/commands/enqueue", map[string]any{"command": "look"})
	if status != http.StatusOK {
		t.Fatalf("enqueue status: expected 200, got %d", status)
	}
	_ = readUntilEvent(t, conn, "terminal.output", 2*time.Second)

	response := getStateSnapshot(t, baseURL+"/api/v0/state/snapshot")
	if response.Data.Location == "" {
		t.Fatalf("expected location in snapshot")
	}
}

func getStateSnapshot(t *testing.T, endpoint string) stateSnapshotResponse {
	t.Helper()

	res, err := http.Get(endpoint)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("snapshot status: expected 200, got %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read snapshot body: %v", err)
	}

	var payload stateSnapshotResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal snapshot body: %v", err)
	}
	return payload
}
