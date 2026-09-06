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
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
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
		b.WriteString(": ")
		if r.Err != nil {
			b.WriteString("[error] ")
			b.WriteString(r.Err.Error())
		} else {
			b.WriteString(r.Response)
		}
		b.WriteString("\n")
	}
	return b.String()
}
