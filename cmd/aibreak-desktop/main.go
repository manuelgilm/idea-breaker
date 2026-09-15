package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"aibreak/internal/app"
	"aibreak/internal/config"
	"aibreak/internal/desktop"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(1)
	}

	svc, router, store, err := app.BuildDesktop(cfg)
	if err != nil {
		logger.Error("build", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	appl := desktop.New(svc, router, desktop.WithFallbackKeys(map[string]string{
		"openai": cfg.APIKey,
		"gemini": cfg.GeminiAPIKey,
	}))
	if err := appl.ApplySettings(); err != nil {
		logger.Warn("apply settings", "err", err)
	}
	if err := appl.ApplyDefaultKey(); err != nil {
		logger.Warn("apply default api key", "err", err)
	}

	err = wails.Run(&options.App{
		Title:  "aibreak",
		Width:  1100,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: appl.Startup,
		Bind: []interface{}{
			appl,
		},
	})
	if err != nil {
		logger.Error("wails", "err", err)
		os.Exit(1)
	}
}
