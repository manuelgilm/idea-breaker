package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"aibreak/internal/api"
	"aibreak/internal/app"
	"aibreak/internal/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(1)
	}

	svc, _, store, err := app.Build(cfg)
	if err != nil {
		logger.Error("build", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.New(svc).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info("listening", "addr", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server", "err", err)
		os.Exit(1)
	}
}
