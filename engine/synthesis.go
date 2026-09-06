package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Synthesis is the scored review produced from the persona outputs.
type Synthesis struct {
	Feedback string
	Score    int // 0-100
}

// Synthesize aggregates the idea and all persona results into a single scored
// review. A failure here is fatal: without a synthesis the run has no deliverable.
func Synthesize(ctx context.Context, caller Caller, synthPrompt, idea string, results []Result) (Synthesis, error) {
	user := buildSynthUser(idea, results)
	raw, err := caller.Chat(ctx, synthPrompt, user)
	if err != nil {
		return Synthesis{}, fmt.Errorf("synthesize: %w", err)
	}

	var out struct {
		Feedback string `json:"feedback"`
		Score    int    `json:"score"`
	}
	if err := json.Unmarshal([]byte(extractJSON(raw)), &out); err != nil {
		return Synthesis{}, fmt.Errorf("parse synthesis response %q: %w", raw, err)
	}
	if out.Score < 0 {
		out.Score = 0
	}
	if out.Score > 100 {
		out.Score = 100
	}
	return Synthesis{Feedback: out.Feedback, Score: out.Score}, nil
}

func buildSynthUser(idea string, results []Result) string {
	var b strings.Builder
	b.WriteString("Idea: ")
	b.WriteString(idea)
	b.WriteString("\n\nPerspectives:\n")
	for _, r := range results {
		b.WriteString("- ")
		b.WriteString(r.Persona.Name)
		if r.Err != nil {
			b.WriteString(": [error] ")
			b.WriteString(r.Err.Error())
			b.WriteString("\n")
			continue
		}
		bd := r.Breakdown
		b.WriteString(": ")
		b.WriteString(bd.Summary)
		b.WriteString("\n  key points: ")
		b.WriteString(strings.Join(bd.KeyPoints, "; "))
		b.WriteString("\n  risks: ")
		b.WriteString(strings.Join(bd.Risks, "; "))
		b.WriteString("\n  dependencies: ")
		b.WriteString(strings.Join(bd.Dependencies, "; "))
		b.WriteString("\n  score: ")
		fmt.Fprintf(&b, "%d\n", bd.Score)
	}
	return b.String()
}
