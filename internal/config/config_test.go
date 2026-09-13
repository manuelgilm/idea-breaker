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
