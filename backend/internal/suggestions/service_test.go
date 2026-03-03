package suggestions

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/athanasius/arda-web-gateway/backend/internal/state"
)

type fakeSnapshotProvider struct {
	snapshot state.Snapshot
	found    bool
	err      error
}

func (f fakeSnapshotProvider) Snapshot() (state.Snapshot, bool, error) {
	return f.snapshot, f.found, f.err
}

type fakeClient struct {
	mu        sync.Mutex
	prompts   []string
	responses []func(prompt string) (string, error)
}

func (f *fakeClient) RequestSuggestion(ctx context.Context, prompt string) (string, error) {
	_ = ctx
	f.mu.Lock()
	idx := len(f.prompts)
	f.prompts = append(f.prompts, prompt)
	var fn func(prompt string) (string, error)
	if idx < len(f.responses) {
		fn = f.responses[idx]
	}
	f.mu.Unlock()

	if fn == nil {
		return `{"commands":["look"],"reason":"default","expected_outcome":"observe"}`, nil
	}
	return fn(prompt)
}

func (f *fakeClient) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.prompts)
}

func TestDebounceFiresSingleSuggestionRequest(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	svc := NewService(
		slog.Default(),
		client,
		fakeSnapshotProvider{found: true, snapshot: state.Snapshot{Location: "Room A"}},
		40*time.Millisecond,
		20,
	)

	svc.IngestTerminal("s-1", "first line")
	time.Sleep(10 * time.Millisecond)
	svc.IngestTerminal("s-1", "second line")

	waitForLatest(t, svc, 1*time.Second)

	if got := client.CallCount(); got != 1 {
		t.Fatalf("expected exactly one suggestion call, got %d", got)
	}
}

func TestStaleResponseIsDiscarded(t *testing.T) {
	t.Parallel()

	firstStarted := make(chan struct{}, 1)
	releaseFirst := make(chan struct{})

	client := &fakeClient{
		responses: []func(prompt string) (string, error){
			func(prompt string) (string, error) {
				firstStarted <- struct{}{}
				<-releaseFirst
				return `{"commands":["north"],"reason":"stale","expected_outcome":"move"}`, nil
			},
			func(prompt string) (string, error) {
				return `{"commands":["look"],"reason":"fresh","expected_outcome":"scan"}`, nil
			},
		},
	}

	svc := NewService(slog.Default(), client, fakeSnapshotProvider{found: true}, 20*time.Millisecond, 20)

	svc.IngestTerminal("s-1", "line 1")
	select {
	case <-firstStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("first suggestion request did not start")
	}

	svc.IngestTerminal("s-1", "line 2")
	waitForLatest(t, svc, 1*time.Second)
	close(releaseFirst)
	time.Sleep(80 * time.Millisecond)

	latest, found := svc.Latest()
	if !found {
		t.Fatal("expected latest suggestion")
	}
	if len(latest.Commands) == 0 || latest.Commands[0] != "look" {
		t.Fatalf("expected fresh suggestion to win, got %+v", latest)
	}
}

func TestInvalidJSONKeepsPreviousSuggestion(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		responses: []func(prompt string) (string, error){
			func(prompt string) (string, error) {
				return `{"commands":["look"],"reason":"valid","expected_outcome":"read room"}`, nil
			},
			func(prompt string) (string, error) {
				return `{"commands":"wrong-shape"}`, nil
			},
		},
	}

	svc := NewService(slog.Default(), client, fakeSnapshotProvider{found: true}, 20*time.Millisecond, 20)
	svc.IngestTerminal("s-1", "line 1")
	waitForLatest(t, svc, 1*time.Second)

	before, _ := svc.Latest()
	if before.Reason != "valid" {
		t.Fatalf("expected first valid reason, got %+v", before)
	}

	svc.IngestTerminal("s-1", "line 2")
	time.Sleep(120 * time.Millisecond)

	after, found := svc.Latest()
	if !found {
		t.Fatal("expected latest suggestion to remain available")
	}
	if after.Reason != "valid" || after.ExpectedOutcome != "read room" {
		t.Fatalf("expected previous suggestion to remain after invalid json, got %+v", after)
	}
}

func waitForLatest(t *testing.T, svc *Service, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, found := svc.Latest(); found {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for latest suggestion")
}
