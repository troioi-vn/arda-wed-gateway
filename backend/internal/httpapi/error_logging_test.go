package httpapi

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/athanasius/arda-web-gateway/backend/internal/gateway"
	"github.com/athanasius/arda-web-gateway/backend/internal/state"
)

type failingResponseWriter struct {
	header http.Header
	status int
}

func (w *failingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *failingResponseWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("forced write failure")
}

type stubManager struct {
	connectFn    func(string, int) (gateway.Status, error)
	disconnectFn func() (gateway.Status, int, error)
	statusFn     func() gateway.Status
	enqueueFn    func(string) (gateway.Status, error)
}

func (m stubManager) Connect(host string, port int) (gateway.Status, error) {
	return m.connectFn(host, port)
}

func (m stubManager) Disconnect() (gateway.Status, int, error) {
	return m.disconnectFn()
}

func (m stubManager) Status() gateway.Status {
	return m.statusFn()
}

func (m stubManager) Enqueue(command string) (gateway.Status, error) {
	return m.enqueueFn(command)
}

func (m stubManager) Hub() *gateway.Hub {
	return nil
}

func (m stubManager) BuildStatusEvent() gateway.TerminalEvent {
	return gateway.TerminalEvent{}
}

type stubStateSnapshotter struct {
	snapshot state.Snapshot
	found    bool
	err      error
}

func (s stubStateSnapshotter) Snapshot() (state.Snapshot, bool, error) {
	return s.snapshot, s.found, s.err
}

func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return logger, &logs
}

func TestWriteJSONLogsFailure(t *testing.T) {
	t.Parallel()

	logger, logs := newTestLogger()
	writer := &failingResponseWriter{}

	writeSuccess(logger, writer, http.StatusOK, "req-1", map[string]any{"ok": true})

	if writer.status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, writer.status)
	}
	got := logs.String()
	if !strings.Contains(got, "http response json write failed") {
		t.Fatalf("expected json write failure log, got %s", got)
	}
	if !strings.Contains(got, "request_id=req-1") {
		t.Fatalf("expected request_id in log, got %s", got)
	}
	if !strings.Contains(got, "status=200") {
		t.Fatalf("expected status in log, got %s", got)
	}
}

func TestWithRequestLoggingIncludesRequestIDAndStatus(t *testing.T) {
	t.Parallel()

	logger, logs := newTestLogger()
	handler := withRequestLogging(logger, func() string { return "req-77" }, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if got := requestIDFromContext(req.Context()); got != "req-77" {
			t.Fatalf("expected request id in context, got %q", got)
		}
		_, _ = io.WriteString(w, "ok")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	got := logs.String()
	if !strings.Contains(got, "request_id=req-77") {
		t.Fatalf("expected request_id in completion log, got %s", got)
	}
	if !strings.Contains(got, "status=200") {
		t.Fatalf("expected status in completion log, got %s", got)
	}
}

func TestHandleSessionConnectLogsRootCauseOnInternalError(t *testing.T) {
	t.Parallel()

	logger, logs := newTestLogger()
	r := &Router{
		logger: logger,
		manager: stubManager{
			connectFn: func(_ string, _ int) (gateway.Status, error) {
				return gateway.Status{}, errors.New("dial exploded")
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v0/session/connect", strings.NewReader(`{"host":"example.org","port":7000}`))
	req = req.WithContext(withRequestID(req.Context(), "req-connect"))
	rec := httptest.NewRecorder()

	r.handleSessionConnect(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	got := logs.String()
	if !strings.Contains(got, "session connect failed") || !strings.Contains(got, "request_id=req-connect") || !strings.Contains(got, "dial exploded") {
		t.Fatalf("expected root-cause connect log, got %s", got)
	}
}

func TestHandleSessionDisconnectLogsRootCauseOnInternalError(t *testing.T) {
	t.Parallel()

	logger, logs := newTestLogger()
	r := &Router{
		logger: logger,
		manager: stubManager{
			disconnectFn: func() (gateway.Status, int, error) {
				return gateway.Status{SessionID: "s-1"}, 0, errors.New("close failed")
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v0/session/disconnect", nil)
	req = req.WithContext(withRequestID(req.Context(), "req-disconnect"))
	rec := httptest.NewRecorder()

	r.handleSessionDisconnect(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	got := logs.String()
	if !strings.Contains(got, "session disconnect failed") || !strings.Contains(got, "request_id=req-disconnect") || !strings.Contains(got, "close failed") {
		t.Fatalf("expected root-cause disconnect log, got %s", got)
	}
}

func TestHandleEnqueueLogsRootCauseOnInternalError(t *testing.T) {
	t.Parallel()

	logger, logs := newTestLogger()
	r := &Router{
		logger: logger,
		manager: stubManager{
			enqueueFn: func(_ string) (gateway.Status, error) {
				return gateway.Status{SessionID: "s-1"}, errors.New("queue panic")
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v0/commands/enqueue", strings.NewReader(`{"command":"look"}`))
	req = req.WithContext(withRequestID(req.Context(), "req-enqueue"))
	rec := httptest.NewRecorder()

	r.handleEnqueueCommand(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	got := logs.String()
	if !strings.Contains(got, "command enqueue failed") || !strings.Contains(got, "request_id=req-enqueue") || !strings.Contains(got, "queue panic") {
		t.Fatalf("expected root-cause enqueue log, got %s", got)
	}
}

func TestHandleStateSnapshotLogsRootCauseOnInternalError(t *testing.T) {
	t.Parallel()

	logger, logs := newTestLogger()
	r := &Router{
		logger: logger,
		state:  stubStateSnapshotter{err: errors.New("db read failed")},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v0/state/snapshot", nil)
	req = req.WithContext(withRequestID(req.Context(), "req-state"))
	rec := httptest.NewRecorder()

	r.handleStateSnapshot(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	got := logs.String()
	if !strings.Contains(got, "state snapshot load failed") || !strings.Contains(got, "request_id=req-state") || !strings.Contains(got, "db read failed") {
		t.Fatalf("expected root-cause state log, got %s", got)
	}
}
