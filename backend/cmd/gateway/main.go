package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/athanasius/arda-web-gateway/backend/internal/config"
	"github.com/athanasius/arda-web-gateway/backend/internal/httpapi"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	logger.Info("gateway starting", "host", cfg.Host, "port", cfg.Port, "version", cfg.BuildVersion, "commit", cfg.BuildCommit)

	server := &http.Server{
		Addr:         cfg.Address(),
		Handler:      httpapi.NewRouter(cfg, logger),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("gateway server failed", "error", err.Error())
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("gateway shutting down")
	shutdownCtx, cancel := config.ShutdownContext()
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("gateway graceful shutdown failed", "error", err.Error())
		os.Exit(1)
	}

	logger.Info("gateway stopped")
}
