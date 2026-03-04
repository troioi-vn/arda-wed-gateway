package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/athanasius/arda-web-gateway/backend/internal/config"
)

type contractEnvelope struct {
	Data json.RawMessage `json:"data"`
	Meta struct {
		RequestID string `json:"request_id"`
		Timestamp string `json:"timestamp"`
	} `json:"meta"`
}

func newContractTestHandler(t *testing.T) http.Handler {
	t.Helper()
	return NewRouter(config.Config{
		Host:         "127.0.0.1",
		Port:         "8080",
		BuildVersion: "test",
		BuildCommit:  "abc123",
		SQLitePath:   t.TempDir() + "/gateway.sqlite",
	}, slog.Default())
}

func TestContractSessionStatusEnvelope(t *testing.T) {
	t.Parallel()

	handler := newContractTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/session/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload struct {
		contractEnvelope
		Data struct {
			SessionID  string `json:"session_id"`
			Connected  bool   `json:"connected"`
			QueueDepth int    `json:"queue_depth"`
			QueueMax   int    `json:"queue_max"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal session status: %v", err)
	}

	if payload.Meta.RequestID == "" || payload.Meta.Timestamp == "" {
		t.Fatalf("expected meta.request_id and meta.timestamp to be populated")
	}
	if payload.Data.QueueMax <= 0 {
		t.Fatalf("expected queue_max > 0, got %d", payload.Data.QueueMax)
	}
}

func TestContractSessionConnectInvalidRequestError(t *testing.T) {
	t.Parallel()

	handler := newContractTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v0/session/connect", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Meta struct {
			RequestID string `json:"request_id"`
			Timestamp string `json:"timestamp"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal session connect error: %v", err)
	}

	if payload.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST error code, got %q", payload.Error.Code)
	}
	if payload.Meta.RequestID == "" || payload.Meta.Timestamp == "" {
		t.Fatalf("expected meta.request_id and meta.timestamp to be populated")
	}
}

func TestContractStateSnapshotEmptyShape(t *testing.T) {
	t.Parallel()

	handler := newContractTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/state/snapshot", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload contractEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal state snapshot: %v", err)
	}

	if string(payload.Data) != "{}" {
		t.Fatalf("expected empty object data, got %s", string(payload.Data))
	}
}

func TestContractSuggestionsLatestEmptyShape(t *testing.T) {
	t.Parallel()

	handler := newContractTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/suggestions/latest", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload contractEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal suggestions latest: %v", err)
	}

	if string(payload.Data) != "{}" {
		t.Fatalf("expected empty object data, got %s", string(payload.Data))
	}
}
