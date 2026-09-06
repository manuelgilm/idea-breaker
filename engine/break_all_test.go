package engine

import (
	"context"
	"errors"
	"testing"
)

const stubBreakdown = `{"summary":"verdict","key_points":["kp"],"risks":["risk"],"dependencies":["dep"],"score":50}`

// stubCaller is a test double: it returns a predictable JSON breakdown and can
// be configured to fail at the transport level.
type stubCaller struct {
	err  error
	text string
}

func (s stubCaller) Chat(ctx context.Context, system, user string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		if s.err != nil {
			return "", s.err
		}
		return s.text, nil
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

	results := BreakAll(ctx, stubCaller{text: stubBreakdown}, "idea", personas)

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
		if r.Breakdown.Summary != "verdict" {
			t.Errorf("result %d: unexpected summary %q", i, r.Breakdown.Summary)
		}
		if r.Breakdown.Score != 50 {
			t.Errorf("result %d: unexpected score %d", i, r.Breakdown.Score)
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

func TestBreakAllUnparseable(t *testing.T) {
	ctx := context.Background()
	personas := []Persona{{Name: "pessimist"}}

	results := BreakAll(ctx, stubCaller{text: "not json"}, "idea", personas)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected parse error for non-JSON response")
	}
}
