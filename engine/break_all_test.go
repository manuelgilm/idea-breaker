package engine

import (
	"context"
	"errors"
	"testing"
)

// stubCaller is a test double: it returns a predictable response per persona
// and can be configured to fail.
type stubCaller struct {
	err error
}

func (s stubCaller) Break(ctx context.Context, p Persona, idea string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		if s.err != nil {
			return "", s.err
		}
		return "response for " + p.Name, nil
	}
}

var _ Caller = stubCaller{}

func TestBreakAllOrdered(t *testing.T) {
	ctx := context.Background()
	personas := []Persona{
		{Name: "pessimist"},
		{Name: "optimist"},
		{Name: "architect"},
	}

	results := BreakAll(ctx, stubCaller{}, "idea", personas)

	if len(results) != len(personas) {
		t.Fatalf("expected %d results, got %d", len(personas), len(results))
	}
	for i, r := range results {
		if r.Persona.Name != personas[i].Name {
			t.Errorf("result %d: expected persona %q, got %q", i, personas[i].Name, r.Persona.Name)
		}
		if r.Err != nil {
			t.Errorf("result %d: unexpected error %v", i, r.Err)
		}
		if r.Response != "response for "+personas[i].Name {
			t.Errorf("result %d: unexpected response %q", i, r.Response)
		}
	}
}

func TestBreakAllCollectsErrors(t *testing.T) {
	ctx := context.Background()
	personas := []Persona{
		{Name: "pessimist"},
		{Name: "optimist"},
		{Name: "architect"},
	}
	boom := errors.New("boom")

	results := BreakAll(ctx, stubCaller{err: boom}, "idea", personas)

	if len(results) != len(personas) {
		t.Fatalf("expected %d results, got %d", len(personas), len(results))
	}
	for i, r := range results {
		if r.Persona.Name != personas[i].Name {
			t.Errorf("result %d: expected persona %q, got %q", i, personas[i].Name, r.Persona.Name)
		}
		if !errors.Is(r.Err, boom) {
			t.Errorf("result %d: expected error %v, got %v", i, boom, r.Err)
		}
	}
}
