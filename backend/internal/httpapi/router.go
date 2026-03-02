package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/athanasius/arda-web-gateway/backend/internal/config"
	"github.com/athanasius/arda-web-gateway/backend/internal/gateway"
)

type Router struct {
	cfg     config.Config
	logger  *slog.Logger
	counter uint64
	manager *gateway.Manager
	metrics *gateway.Metrics
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
	r := &Router{
		cfg:     cfg,
		logger:  logger,
		metrics: metrics,
		manager: gateway.NewManager(queueInterval, queueMaxDepth, logger, metrics),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/health", r.handleHealth)
	mux.HandleFunc("GET /api/v0/session/status", r.handleSessionStatus)
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
