package httpapi

import (
	"log/slog"
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
	manager     *gateway.Manager
	metrics     *gateway.Metrics
	state       *state.Service
	suggestions *suggestions.Service
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
	return withRequestLogging(r.logger, mux)
}

func (r *Router) nextRequestID() string {
	seq := atomic.AddUint64(&r.counter, 1)
	return time.Now().UTC().Format("20060102T150405.000Z07:00") + "-" + itoa(seq)
}

func withRequestLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, req)
		logger.Info("http request completed", "method", req.Method, "path", req.URL.Path, "elapsed_ms", time.Since(start).Milliseconds())
	})
}

func itoa(v uint64) string {
	return strconv.FormatUint(v, 10)
}
