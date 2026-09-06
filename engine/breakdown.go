package engine

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Breakdown is the structured output a persona returns, instead of free text.
type Breakdown struct {
	Summary      string   `json:"summary"`
	KeyPoints    []string `json:"key_points"`
	Risks        []string `json:"risks"`
	Dependencies []string `json:"dependencies"`
	Score        int      `json:"score"`
}

// parseBreakdown extracts the JSON object from the persona response and
// unmarshals it. The score is clamped to 0-100.
func parseBreakdown(raw string) (Breakdown, error) {
	var b Breakdown
	if err := json.Unmarshal([]byte(extractJSON(raw)), &b); err != nil {
		return Breakdown{}, fmt.Errorf("parse persona response %q: %w", raw, err)
	}
	if b.Score < 0 {
		b.Score = 0
	}
	if b.Score > 100 {
		b.Score = 100
	}
	return b, nil
}

// extractJSON returns the substring between the first '{' and last '}' in s.
// Models often wrap their JSON reply in prose or markdown code fences, so we
// slice out the object before parsing. If no braces are present, s is returned
// unchanged so the downstream parse error is clear.
func extractJSON(s string) string {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start == -1 || end == -1 || end < start {
		return s
	}
	return s[start : end+1]
}
