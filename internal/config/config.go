// Package config loads runtime configuration with precedence
// flags > env vars > config file > defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

// Config holds all runtime settings.
type Config struct {
	LLMProvider  string
	APIKey       string
	GeminiAPIKey string
	Model        string
	Temperature  float64
	MaxTokens    int
	Timeout      time.Duration
	Retries      int
	DBPath       string
	Addr         string
}

// Defaults returns the default configuration.
func Defaults() Config {
	return Config{
		LLMProvider: "openai",
		Model:       "gpt-4o-mini",
		Temperature: 0,
		MaxTokens:   512,
		Timeout:     60 * time.Second,
		Retries:     1,
		DBPath:      defaultDBPath(),
		Addr:        "127.0.0.1:8080",
	}
}

// defaultDBPath returns an absolute, user-writable database path (the OS user
// config dir), falling back to a relative path if it cannot be resolved. A
// relative path is not safe for GUI apps, which are often launched from a
// read-only or root working directory.
func defaultDBPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "aibreak", "aibreak.db")
	}
	return "aibreak.db"
}

// fileConfig mirrors Config with pointers so missing TOML keys are detectable.
type fileConfig struct {
	LLMProvider  *string  `toml:"llm_provider"`
	APIKey       *string  `toml:"api_key"`
	GeminiAPIKey *string  `toml:"gemini_api_key"`
	Model        *string  `toml:"model"`
	Temperature  *float64 `toml:"temperature"`
	MaxTokens    *int     `toml:"max_tokens"`
	Timeout      *string  `toml:"timeout"`
	Retries      *int     `toml:"retries"`
	DBPath       *string  `toml:"db_path"`
	Addr         *string  `toml:"addr"`
}

// Load builds the configuration by layering defaults, the config file, and
// environment variables (in increasing precedence).
func Load() (Config, error) {
	cfg := Defaults()

	if err := loadFile(&cfg); err != nil {
		return cfg, err
	}
	if err := loadEnv(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func configFilePath() string {
	if p := os.Getenv("AIBREAK_CONFIG"); p != "" {
		return p
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "aibreak", "config.toml")
	}
	return ""
}

func loadFile(cfg *Config) error {
	path := configFilePath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}

	var fc fileConfig
	if err := toml.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if fc.LLMProvider != nil {
		cfg.LLMProvider = *fc.LLMProvider
	}
	if fc.APIKey != nil {
		cfg.APIKey = *fc.APIKey
	}
	if fc.GeminiAPIKey != nil {
		cfg.GeminiAPIKey = *fc.GeminiAPIKey
	}
	if fc.Model != nil {
		cfg.Model = *fc.Model
	}
	if fc.Temperature != nil {
		cfg.Temperature = *fc.Temperature
	}
	if fc.MaxTokens != nil {
		cfg.MaxTokens = *fc.MaxTokens
	}
	if fc.Timeout != nil {
		d, err := time.ParseDuration(*fc.Timeout)
		if err != nil {
			return fmt.Errorf("config timeout: %w", err)
		}
		cfg.Timeout = d
	}
	if fc.Retries != nil {
		cfg.Retries = *fc.Retries
	}
	if fc.DBPath != nil {
		cfg.DBPath = *fc.DBPath
	}
	if fc.Addr != nil {
		cfg.Addr = *fc.Addr
	}
	return nil
}

func loadEnv(cfg *Config) error {
	var err error
	setStr := func(key string, dst *string) {
		if v, ok := os.LookupEnv(key); ok {
			*dst = v
		}
	}
	setFloat := func(key string, dst *float64) error {
		if v, ok := os.LookupEnv(key); ok {
			f, e := strconv.ParseFloat(v, 64)
			if e != nil {
				return fmt.Errorf("%s: %w", key, e)
			}
			*dst = f
		}
		return nil
	}
	setInt := func(key string, dst *int) error {
		if v, ok := os.LookupEnv(key); ok {
			n, e := strconv.Atoi(v)
			if e != nil {
				return fmt.Errorf("%s: %w", key, e)
			}
			*dst = n
		}
		return nil
	}

	setStr("AIBREAK_LLM_PROVIDER", &cfg.LLMProvider)
	setStr("OPENAI_API_KEY", &cfg.APIKey)
	setStr("GEMINI_API_KEY", &cfg.GeminiAPIKey)
	setStr("AIBREAK_LLM_MODEL", &cfg.Model)
	setStr("AIBREAK_DB", &cfg.DBPath)
	setStr("AIBREAK_ADDR", &cfg.Addr)

	if err = setFloat("AIBREAK_LLM_TEMPERATURE", &cfg.Temperature); err != nil {
		return err
	}
	if err = setInt("AIBREAK_LLM_MAX_TOKENS", &cfg.MaxTokens); err != nil {
		return err
	}
	if err = setInt("AIBREAK_LLM_RETRIES", &cfg.Retries); err != nil {
		return err
	}
	if v, ok := os.LookupEnv("AIBREAK_LLM_TIMEOUT"); ok {
		d, e := time.ParseDuration(v)
		if e != nil {
			return fmt.Errorf("AIBREAK_LLM_TIMEOUT: %w", e)
		}
		cfg.Timeout = d
	}
	return nil
}
