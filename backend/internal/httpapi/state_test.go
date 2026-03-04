package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
		StatusTags []string          `json:"status_tags"`
		Equipment  map[string]string `json:"equipment"`
		UpdatedAt  string            `json:"updated_at"`
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

func TestStateSnapshotEndpointFixtureRegressionFields(t *testing.T) {
	t.Parallel()

	location := loadParserFixture(t, "cp1251_plain_room.txt")
	status := loadParserFixture(t, "aura_state_prefix_variants_ru.txt")
	equipment := loadParserFixture(t, "equipment_slots_variants_ru.txt")
	prompt := loadParserFixture(t, "prompt_hp_ma_mv_exp_variants.txt")

	upstream := newFakeUpstream(t)
	upstream.SetResponse("snapshot", strings.Join([]string{location, status, equipment, prompt}, "\n"))
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

	statusCode := postJSON(t, baseURL+"/api/v0/session/connect", map[string]any{
		"host": "127.0.0.1",
		"port": upstream.Port(),
	})
	if statusCode != http.StatusOK {
		t.Fatalf("connect status: expected 200, got %d", statusCode)
	}
	_ = readUntilEvent(t, conn, "session.connected", 2*time.Second)

	statusCode = postJSON(t, baseURL+"/api/v0/commands/enqueue", map[string]any{"command": "snapshot"})
	if statusCode != http.StatusOK {
		t.Fatalf("enqueue status: expected 200, got %d", statusCode)
	}
	_ = readUntilEvent(t, conn, "terminal.output", 2*time.Second)

	response := getStateSnapshot(t, baseURL+"/api/v0/state/snapshot")
	if response.Data.Location != "Таверна \"Гарцующий пони\"" {
		t.Fatalf("location mismatch: got %q", response.Data.Location)
	}

	wantPrompt := struct {
		HPCurrent int `json:"hp_current"`
		HPMax     int `json:"hp_max"`
		MACurrent int `json:"ma_current"`
		MAMax     int `json:"ma_max"`
		MVCurrent int `json:"mv_current"`
		MVMax     int `json:"mv_max"`
		EXP       int `json:"exp"`
	}{
		HPCurrent: 120,
		HPMax:     150,
		MACurrent: 88,
		MAMax:     90,
		MVCurrent: 60,
		MVMax:     77,
		EXP:       4512,
	}
	if !reflect.DeepEqual(response.Data.Prompt, wantPrompt) {
		t.Fatalf("prompt mismatch\nwant: %#v\ngot: %#v", wantPrompt, response.Data.Prompt)
	}

	if len(response.Data.StatusTags) == 0 {
		t.Fatalf("expected status tags to be present")
	}
	if !containsString(response.Data.StatusTags, "В полете") {
		t.Fatalf("expected parsed status tags to include %q, got %#v", "В полете", response.Data.StatusTags)
	}

	if len(response.Data.Equipment) < 4 {
		t.Fatalf("expected multiple parsed equipment slots, got %#v", response.Data.Equipment)
	}
	if response.Data.Equipment["head"] != "рогатый шлем" {
		t.Fatalf("unexpected head slot: %#v", response.Data.Equipment["head"])
	}
	if response.Data.Equipment["body"] != "кольчуга странника" {
		t.Fatalf("unexpected body slot: %#v", response.Data.Equipment["body"])
	}
	if response.Data.Equipment["wield"] != "эльфийский клинок" {
		t.Fatalf("unexpected wield slot: %#v", response.Data.Equipment["wield"])
	}

	if strings.TrimSpace(response.Data.UpdatedAt) == "" {
		t.Fatalf("expected updated_at to be present")
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

func loadParserFixture(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "parser", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(content)
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
