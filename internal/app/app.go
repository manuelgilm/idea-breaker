// Package app wires configuration, provider, engine, and store into a service.
package app

import (
	"fmt"
	"os"
	"path/filepath"

	"aibreak/internal/config"
	"aibreak/internal/engine"
	"aibreak/internal/llm/openai"
	"aibreak/internal/registry/sqlite"
	"aibreak/internal/service"
)

// Build assembles the service, its provider, and its store from configuration.
// The caller is responsible for closing the returned store.
func Build(cfg config.Config) (*service.Service, *openai.Provider, *sqlite.Store, error) {
	if cfg.DBPath != "" && cfg.DBPath != ":memory:" {
		if err := ensureDir(filepath.Dir(cfg.DBPath)); err != nil {
			return nil, nil, nil, err
		}
	}

	store, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return nil, nil, nil, err
	}

	provider := openai.New(cfg.APIKey)
	switch cfg.LLMProvider {
	case "openai":
		// default
	default:
		store.Close()
		return nil, nil, nil, fmt.Errorf("unsupported llm provider %q", cfg.LLMProvider)
	}

	evaluator := engine.New(provider,
		engine.WithModel(cfg.Model),
		engine.WithTemperature(cfg.Temperature),
		engine.WithMaxTokens(cfg.MaxTokens),
		engine.WithRetries(cfg.Retries),
		engine.WithTimeout(cfg.Timeout),
	)

	return service.New(store, evaluator), provider, store, nil
}

func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
