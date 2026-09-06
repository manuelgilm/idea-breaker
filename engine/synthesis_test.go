package engine

import (
	"context"
	"errors"
	"testing"
)

type synthStub struct {
	err  error
	text string
}

func (s synthStub) Chat(ctx context.Context, system, user string) (string, error) {
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

var _ Caller = synthStub{}

func TestSynthesizeParsesJSON(t *testing.T) {
	caller := synthStub{text: `{"feedback":"looks promising","score":72}`}
	results := []Result{{Persona: Persona{Name: "pessimist"}, Response: "it will fail"}}

	got, err := Synthesize(context.Background(), caller, "synthesize", "an idea", results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Feedback != "looks promising" {
		t.Errorf("expected feedback %q, got %q", "looks promising", got.Feedback)
	}
	if got.Score != 72 {
		t.Errorf("expected score 72, got %d", got.Score)
	}
}

func TestSynthesizeClampsScore(t *testing.T) {
	for _, tc := range []struct {
		name  string
		json  string
		score int
	}{
		{name: "over", json: `{"feedback":"x","score":150}`, score: 100},
		{name: "under", json: `{"feedback":"x","score":-5}`, score: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caller := synthStub{text: tc.json}
			got, err := Synthesize(context.Background(), caller, "synthesize", "an idea", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Score != tc.score {
				t.Errorf("expected score %d, got %d", tc.score, got.Score)
			}
		})
	}
}

func TestSynthesizeHardFailures(t *testing.T) {
	if _, err := Synthesize(context.Background(), synthStub{err: errors.New("boom")}, "synthesize", "an idea", nil); err == nil {
		t.Fatal("expected error from caller failure")
	}
	if _, err := Synthesize(context.Background(), synthStub{text: "not json"}, "synthesize", "an idea", nil); err == nil {
		t.Fatal("expected error from unparseable response")
	}
}
