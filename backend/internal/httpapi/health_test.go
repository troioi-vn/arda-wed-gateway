package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/athanasius/arda-web-gateway/backend/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	handler := NewRouter(config.Config{
		Host:         "127.0.0.1",
		Port:         "8080",
		BuildVersion: "test",
		BuildCommit:  "abc123",
	}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v0/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got struct {
		Data struct {
			Status  string `json:"status"`
			Version string `json:"version"`
			Commit  string `json:"commit"`
		} `json:"data"`
		Meta struct {
			RequestID string `json:"request_id"`
			Timestamp string `json:"timestamp"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got.Data.Status != "ok" {
		t.Fatalf("expected status ok, got %q", got.Data.Status)
	}
	if got.Data.Version != "test" {
		t.Fatalf("expected version test, got %q", got.Data.Version)
	}
	if got.Data.Commit != "abc123" {
		t.Fatalf("expected commit abc123, got %q", got.Data.Commit)
	}
	if got.Meta.RequestID == "" {
		t.Fatalf("expected request_id to be set")
	}
	if got.Meta.Timestamp == "" {
		t.Fatalf("expected timestamp to be set")
	}
}
