package engine

import (
	"context"
	"strings"
	"testing"
)

func TestMockCallerPersona(t *testing.T) {
	got, err := MockCaller{}.Chat(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "summary") {
		t.Errorf("expected a persona breakdown JSON, got %q", got)
	}
}

func TestMockCallerSynthesizer(t *testing.T) {
	got, err := MockCaller{}.Chat(context.Background(), "system", "Idea: x\n\nPerspectives:\n- p: v")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "feedback") || !strings.Contains(got, "score") {
		t.Errorf("expected a synthesis JSON with feedback and score, got %q", got)
	}
}

func TestMockCallerRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := MockCaller{}.Chat(ctx, "system", "user")
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
}
