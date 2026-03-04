package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/athanasius/arda-web-gateway/backend/internal/state"
	"github.com/athanasius/arda-web-gateway/backend/internal/suggestions"
)

type fakeSuggestionClient struct{}

func (f fakeSuggestionClient) RequestSuggestion(ctx context.Context, prompt string) (string, error) {
	_ = ctx
	_ = prompt
	return `{"commands":["look"],"reason":"scan room","expected_outcome":"new context"}`, nil
}

type snapshotProvider struct{}

func (s snapshotProvider) Snapshot() (state.Snapshot, bool, error) {
	return state.Snapshot{}, false, nil
}

type blockingSuggestionClient struct {
	started chan struct{}
}

func (b blockingSuggestionClient) RequestSuggestion(ctx context.Context, prompt string) (string, error) {
	_ = prompt
	b.started <- struct{}{}
	<-ctx.Done()
	return "", ctx.Err()
}

func TestSuggestionsLatestEndpoint(t *testing.T) {
	t.Parallel()

	svc := suggestions.NewService(slog.Default(), fakeSuggestionClient{}, snapshotProvider{}, 10*time.Millisecond, 20)
	svc.IngestTerminal("s-1", "you see a room")

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if _, found := svc.Latest(); found {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	r := &Router{logger: slog.Default(), suggestions: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/v0/suggestions/latest", nil)
	rr := httptest.NewRecorder()

	r.handleSuggestionsLatest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	body, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	var payload struct {
		Data struct {
			Commands        []string `json:"commands"`
			Reason          string   `json:"reason"`
			ExpectedOutcome string   `json:"expected_outcome"`
			InProgress      bool     `json:"in_progress"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload.Data.InProgress {
		t.Fatal("expected in_progress=false after suggestion completes")
	}
	if len(payload.Data.Commands) == 0 || payload.Data.Commands[0] != "look" {
		t.Fatalf("unexpected suggestion payload: %+v", payload.Data)
	}
}

func TestSuggestionsCancelEndpoint(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 1)
	svc := suggestions.NewService(slog.Default(), blockingSuggestionClient{started: started}, snapshotProvider{}, 1*time.Millisecond, 20)
	svc.IngestTerminal("s-1", "you see a room")

	select {
	case <-started:
	case <-time.After(1 * time.Second):
		t.Fatal("expected suggestion request to start")
	}

	r := &Router{logger: slog.Default(), suggestions: svc}
	req := httptest.NewRequest(http.MethodPost, "/api/v0/suggestions/cancel", nil)
	rr := httptest.NewRecorder()
	r.handleSuggestionsCancel(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		status := svc.Status()
		if !status.InProgress {
			if status.LastError == "" {
				t.Fatal("expected cancellation to be recorded as last error")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for cancellation to complete")
}
