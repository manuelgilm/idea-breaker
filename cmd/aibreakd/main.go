package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aibreak/internal/api"
	"aibreak/internal/app"
	"aibreak/internal/config"
	"aibreak/internal/webui"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(1)
	}

	if !isLoopback(cfg.Addr) {
		logger.Warn("binding to a non-loopback address exposes the unauthenticated API", "addr", cfg.Addr)
	}

	svc, router, store, err := app.BuildDesktop(cfg)
	if err != nil {
		logger.Error("build", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := app.ApplySettings(context.Background(), svc, router); err != nil {
		logger.Warn("apply settings", "err", err)
	}

	server := &http.Server{
		Addr: cfg.Addr,
		Handler: webui.Handler(api.New(svc, api.WithProviderRouter(router), api.WithFallbackKeys(map[string]string{
			"openai": cfg.APIKey,
			"gemini": cfg.GeminiAPIKey,
		})).Handler()),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info("listening", "addr", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server", "err", err)
		os.Exit(1)
	}
}

// isLoopback reports whether addr binds to a loopback interface only.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
