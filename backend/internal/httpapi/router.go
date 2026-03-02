package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/athanasius/arda-web-gateway/backend/internal/config"
)

type Router struct {
	cfg     config.Config
	logger  *slog.Logger
	counter uint64
}

func NewRouter(cfg config.Config, logger *slog.Logger) http.Handler {
	r := &Router{
		cfg:    cfg,
		logger: logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/health", r.handleHealth)
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
