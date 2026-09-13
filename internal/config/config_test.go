package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDBPath(t *testing.T) {
	p := defaultDBPath()

	if !strings.HasSuffix(p, filepath.Join("aibreak", "aibreak.db")) {
		t.Fatalf("defaultDBPath() = %q, want suffix %q", p, filepath.Join("aibreak", "aibreak.db"))
	}

	if _, err := os.UserConfigDir(); err == nil {
		if !filepath.IsAbs(p) {
			t.Fatalf("defaultDBPath() = %q, want an absolute path", p)
		}
	}
}

func TestGeminiAPIKeyEnv(t *testing.T) {
	t.Setenv("AIBREAK_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("GEMINI_API_KEY", "sk-gemini-test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.GeminiAPIKey != "sk-gemini-test" {
		t.Fatalf("GeminiAPIKey = %q, want %q", cfg.GeminiAPIKey, "sk-gemini-test")
	}
}
