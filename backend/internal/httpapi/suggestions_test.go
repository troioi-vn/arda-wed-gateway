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
		Data suggestions.Suggestion `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(payload.Data.Commands) == 0 || payload.Data.Commands[0] != "look" {
		t.Fatalf("unexpected suggestion payload: %+v", payload.Data)
	}
}
