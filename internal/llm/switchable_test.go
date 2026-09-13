package llm

import (
	"context"
	"testing"
)

type recordingProvider struct {
	model  string
	called bool
}

func (p *recordingProvider) Complete(ctx context.Context, req Request) (Response, error) {
	p.called = true
	p.model = req.Model
	return Response{Content: "ok"}, nil
}

func (p *recordingProvider) SetAPIKey(string) {}

func TestSwitchableRoutesAndOverridesModel(t *testing.T) {
	openai := &recordingProvider{}
	gemini := &recordingProvider{}
	s := NewSwitchable(
		map[string]KeyedProvider{"openai": openai, "gemini": gemini},
		map[string]string{"openai": "gpt-4o-mini", "gemini": "gemini-2.5-flash"},
		"openai",
	)

	resp, err := s.Complete(context.Background(), Request{Model: "ignored"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "ok" || !openai.called || gemini.called {
		t.Fatalf("expected openai called, got openai.called=%v gemini.called=%v", openai.called, gemini.called)
	}
	if openai.model != "gpt-4o-mini" {
		t.Fatalf("model = %q, want gpt-4o-mini", openai.model)
	}

	s.SetActive("gemini")
	if _, err := s.Complete(context.Background(), Request{Model: "ignored"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !gemini.called || gemini.model != "gemini-2.5-flash" {
		t.Fatalf("expected gemini called with gemini-2.5-flash, got called=%v model=%q", gemini.called, gemini.model)
	}
	if s.Active() != "gemini" {
		t.Fatalf("Active() = %q, want gemini", s.Active())
	}
}

func TestSwitchableNamesSortedAndDefaultActive(t *testing.T) {
	s := NewSwitchable(
		map[string]KeyedProvider{"gemini": &recordingProvider{}, "openai": &recordingProvider{}},
		map[string]string{"gemini": "gemini-2.5-flash"},
		"",
	)
	names := s.Names()
	if len(names) != 2 || names[0] != "gemini" || names[1] != "openai" {
		t.Fatalf("Names() = %v, want [gemini openai]", names)
	}
	if s.Active() != "gemini" {
		t.Fatalf("Active() = %q, want default gemini", s.Active())
	}
	if s.Model("gemini") != "gemini-2.5-flash" {
		t.Fatalf("Model(gemini) = %q", s.Model("gemini"))
	}
}

func TestSwitchableSetModel(t *testing.T) {
	openai := &recordingProvider{}
	gemini := &recordingProvider{}
	s := NewSwitchable(
		map[string]KeyedProvider{"openai": openai, "gemini": gemini},
		map[string]string{"openai": "gpt-4o-mini", "gemini": "gemini-3.8-flash"},
		"openai",
	)

	if s.Model("openai") != "gpt-4o-mini" {
		t.Fatalf("Model(openai) = %q", s.Model("openai"))
	}
	s.SetModel("openai", "gpt-4o")
	if s.Model("openai") != "gpt-4o" {
		t.Fatalf("Model(openai) after SetModel = %q, want gpt-4o", s.Model("openai"))
	}

	// The routed request uses the new model.
	if _, err := s.Complete(context.Background(), Request{Model: "ignored"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if openai.model != "gpt-4o" {
		t.Fatalf("routed model = %q, want gpt-4o", openai.model)
	}

	// Unknown provider is a no-op.
	s.SetModel("nope", "x")
	if s.Model("openai") != "gpt-4o" {
		t.Fatalf("Model(openai) = %q, want gpt-4o", s.Model("openai"))
	}
}
