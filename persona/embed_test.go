package persona

import (
	"testing"
)

func TestEmbeddedSourceLoads(t *testing.T) {
	src := EmbeddedSource{}
	persons, err := src.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := make(map[string]bool)
	for _, p := range persons {
		names[p.Name] = true
		if p.SystemPrompt == "" {
			t.Errorf("persona %q has empty system prompt", p.Name)
		}
	}
	for _, want := range []string{"pessimist", "optimist", "architect"} {
		if !names[want] {
			t.Errorf("missing embedded persona %q", want)
		}
	}
}

func TestEmbeddedSourceExcludesSynthesizer(t *testing.T) {
	persons, err := EmbeddedSource{}.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range persons {
		if p.Name == "synthesizer" {
			t.Errorf("synthesizer must not be returned as a persona")
		}
	}
}

func TestEmbeddedSynthesizer(t *testing.T) {
	prompt, err := LoadEmbeddedSynthesizer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt == "" {
		t.Fatal("synthesizer prompt is empty")
	}
}
