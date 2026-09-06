package engine

import (
	"context"
	"errors"
	"testing"
)

func TestSynthesizeParsesJSON(t *testing.T) {
	caller := stubCaller{text: `{"feedback":"looks promising","score":72}`}
	results := []Result{{Persona: Persona{Name: "pessimist"}, Breakdown: Breakdown{Summary: "it will fail"}}}

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

func TestSynthesizeMarkdownFence(t *testing.T) {
	caller := stubCaller{text: "```json\n{\"feedback\":\"ok\",\"score\":50}\n```"}
	got, err := Synthesize(context.Background(), caller, "synthesize", "an idea", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Feedback != "ok" {
		t.Errorf("expected feedback %q, got %q", "ok", got.Feedback)
	}
	if got.Score != 50 {
		t.Errorf("expected score 50, got %d", got.Score)
	}
}

func TestSynthesizePreamble(t *testing.T) {
	caller := stubCaller{text: `Review: {"feedback":"x","score":88}`}
	got, err := Synthesize(context.Background(), caller, "synthesize", "an idea", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Feedback != "x" {
		t.Errorf("expected feedback %q, got %q", "x", got.Feedback)
	}
	if got.Score != 88 {
		t.Errorf("expected score 88, got %d", got.Score)
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
			caller := stubCaller{text: tc.json}
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
	if _, err := Synthesize(context.Background(), stubCaller{err: errors.New("boom")}, "synthesize", "an idea", nil); err == nil {
		t.Fatal("expected error from caller failure")
	}
	if _, err := Synthesize(context.Background(), stubCaller{text: "not json"}, "synthesize", "an idea", nil); err == nil {
		t.Fatal("expected error from unparseable response")
	}
}
