package httpapi

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/athanasius/arda-web-gateway/backend/internal/config"
	"github.com/athanasius/arda-web-gateway/backend/internal/gateway"
	"github.com/athanasius/arda-web-gateway/backend/internal/state"
	"github.com/athanasius/arda-web-gateway/backend/internal/suggestions"
)

type Router struct {
	cfg         config.Config
	logger      *slog.Logger
	counter     uint64
	manager     sessionManager
	metrics     *gateway.Metrics
	state       stateSnapshotter
	suggestions suggestionProvider
}

type sessionManager interface {
	Connect(host string, port int) (gateway.Status, error)
	Disconnect() (gateway.Status, int, error)
	Status() gateway.Status
	Enqueue(command string) (gateway.Status, error)
	Hub() *gateway.Hub
	BuildStatusEvent() gateway.TerminalEvent
}

type stateSnapshotter interface {
	Snapshot() (state.Snapshot, bool, error)
}

type suggestionProvider interface {
	Latest() (suggestions.Suggestion, bool)
}

func NewRouter(cfg config.Config, logger *slog.Logger) http.Handler {
	queueInterval := cfg.QueueSendInterval
	if queueInterval <= 0 {
		queueInterval = 500 * time.Millisecond
	}

	queueMaxDepth := cfg.QueueMaxDepth
	if queueMaxDepth <= 0 {
		queueMaxDepth = 20
	}

	metrics := gateway.NewMetrics()
	stateService := state.NewService(cfg.SQLitePath, logger)
	suggestionClient := suggestions.NewOpenRouterClient(cfg.OpenRouterBaseURL, cfg.OpenRouterModel, cfg.OpenRouterAPIKey, cfg.OpenRouterTimeout)
	suggestionService := suggestions.NewService(logger, suggestionClient, stateService, cfg.SuggestDebounce, cfg.SuggestRecentLines)
	r := &Router{
		cfg:         cfg,
		logger:      logger,
		metrics:     metrics,
		state:       stateService,
		suggestions: suggestionService,
		manager: gateway.NewManager(queueInterval, queueMaxDepth, logger, metrics, nil, func(sessionID, text string) {
			if err := stateService.Ingest(sessionID, text); err != nil {
				logger.Warn("state ingest failed", "session_id", sessionID, "error", err.Error())
			}
			suggestionService.IngestTerminal(sessionID, text)
		}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/health", r.handleHealth)
	mux.HandleFunc("GET /api/v0/session/status", r.handleSessionStatus)
	mux.HandleFunc("GET /api/v0/state/snapshot", r.handleStateSnapshot)
	mux.HandleFunc("GET /api/v0/suggestions/latest", r.handleSuggestionsLatest)
	mux.HandleFunc("POST /api/v0/session/connect", r.handleSessionConnect)
	mux.HandleFunc("POST /api/v0/session/disconnect", r.handleSessionDisconnect)
	mux.HandleFunc("POST /api/v0/commands/enqueue", r.handleEnqueueCommand)
	mux.HandleFunc("GET /api/v0/ws/terminal", r.handleTerminalWS)
	mux.HandleFunc("GET /metrics", r.handleMetrics)
	return withRequestLogging(r.logger, r.nextRequestID, mux)
}

func (r *Router) nextRequestID() string {
	seq := atomic.AddUint64(&r.counter, 1)
	return time.Now().UTC().Format("20060102T150405.000Z07:00") + "-" + itoa(seq)
}

func (r *Router) requestID(req *http.Request) string {
	if requestID := requestIDFromContext(req.Context()); requestID != "" {
		return requestID
	}
	return r.nextRequestID()
}

type requestIDKey struct{}

func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w}
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(data)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	if r.status == 0 {
		r.status = http.StatusSwitchingProtocols
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func withRequestLogging(logger *slog.Logger, nextRequestID func() string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestID := nextRequestID()
		req = req.WithContext(withRequestID(req.Context(), requestID))
		start := time.Now()
		rec := newStatusRecorder(w)
		next.ServeHTTP(rec, req)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}

		logAttrs := []any{
			"request_id", requestID,
			"method", req.Method,
			"path", req.URL.Path,
			"status", status,
			"bytes", rec.bytes,
			"elapsed_ms", time.Since(start).Milliseconds(),
		}
		if status >= http.StatusBadRequest {
			logger.Info("http request completed", logAttrs...)
			return
		}
		logger.Debug("http request completed", logAttrs...)
	})
}

func itoa(v uint64) string {
	return strconv.FormatUint(v, 10)
}
