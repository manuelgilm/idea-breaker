// Package app wires configuration, provider, engine, and store into a service.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"aibreak/internal/config"
	"aibreak/internal/engine"
	"aibreak/internal/llm"
	"aibreak/internal/llm/gemini"
	"aibreak/internal/llm/openai"
	"aibreak/internal/registry/sqlite"
	"aibreak/internal/service"
)

// defaultGeminiModel is used when the provider is Gemini and no explicit model
// is configured.
const defaultGeminiModel = "gemini-3.8-flash"

// Build assembles the service, its provider, and its store from configuration.
// The caller is responsible for closing the returned store.
func Build(cfg config.Config) (*service.Service, llm.KeyedProvider, *sqlite.Store, error) {
	if cfg.DBPath != "" && cfg.DBPath != ":memory:" {
		if err := ensureDir(filepath.Dir(cfg.DBPath)); err != nil {
			return nil, nil, nil, err
		}
	}

	store, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return nil, nil, nil, err
	}

	var provider llm.KeyedProvider
	var model string
	switch cfg.LLMProvider {
	case "openai":
		provider = openai.New(cfg.APIKey)
		model = cfg.Model
		if model == "" {
			model = "gpt-4o-mini"
		}
	case "gemini":
		provider = gemini.New(cfg.GeminiAPIKey)
		model = cfg.GeminiModel
		if model == "" {
			model = defaultGeminiModel
		}
	default:
		store.Close()
		return nil, nil, nil, fmt.Errorf("unsupported llm provider %q", cfg.LLMProvider)
	}

	evaluator := engine.New(provider,
		engine.WithModel(model),
		engine.WithTemperature(cfg.Temperature),
		engine.WithMaxTokens(cfg.MaxTokens),
		engine.WithRetries(cfg.Retries),
		engine.WithTimeout(cfg.Timeout),
	)

	return service.New(store, evaluator), provider, store, nil
}

// BuildDesktop assembles the service with a switchable multi-provider router
// (OpenAI and Gemini) so the desktop app can switch providers at runtime. The
// active provider starts at cfg.LLMProvider.
func BuildDesktop(cfg config.Config) (*service.Service, *llm.Switchable, *sqlite.Store, error) {
	if cfg.DBPath != "" && cfg.DBPath != ":memory:" {
		if err := ensureDir(filepath.Dir(cfg.DBPath)); err != nil {
			return nil, nil, nil, err
		}
	}

	store, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return nil, nil, nil, err
	}

	openaiModel := cfg.Model
	if openaiModel == "" {
		openaiModel = "gpt-4o-mini"
	}
	geminiModel := cfg.GeminiModel
	if geminiModel == "" {
		geminiModel = defaultGeminiModel
	}

	router := llm.NewSwitchable(
		map[string]llm.KeyedProvider{
			"openai": openai.New(cfg.APIKey),
			"gemini": gemini.New(cfg.GeminiAPIKey),
		},
		map[string]string{
			"openai": openaiModel,
			"gemini": geminiModel,
		},
		cfg.LLMProvider,
	)

	evaluator := engine.New(router,
		engine.WithTemperature(cfg.Temperature),
		engine.WithMaxTokens(cfg.MaxTokens),
		engine.WithRetries(cfg.Retries),
		engine.WithTimeout(cfg.Timeout),
	)

	return service.New(store, evaluator), router, store, nil
}

func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// ApplySettings loads the persisted provider settings (active provider and
// per-provider models) into the router, overriding the config/env defaults.
// It is used by the web service at startup so provider switches made through
// the API survive a restart.
func ApplySettings(ctx context.Context, svc *service.Service, router *llm.Switchable) error {
	if p, err := svc.GetSetting(ctx, "active_provider"); err != nil {
		return err
	} else if p != "" {
		router.SetActive(p)
	}
	for _, name := range router.Names() {
		if m, err := svc.GetSetting(ctx, name+".model"); err != nil {
			return err
		} else if m != "" {
			router.SetModel(name, m)
		}
	}
	return nil
}
