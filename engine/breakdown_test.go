package engine

import "testing"

func TestParseBreakdownValid(t *testing.T) {
	raw := `{"summary":"s","key_points":["a","b"],"risks":["c"],"dependencies":["d"],"score":40}`
	got, err := parseBreakdown(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Summary != "s" || len(got.KeyPoints) != 2 || len(got.Risks) != 1 || len(got.Dependencies) != 1 || got.Score != 40 {
		t.Errorf("unexpected breakdown: %+v", got)
	}
}

func TestParseBreakdownFencedAndPreamble(t *testing.T) {
	for _, raw := range []string{
		"```json\n{\"summary\":\"s\",\"key_points\":[\"a\"],\"score\":20}\n```",
		`Review: {"summary":"s","key_points":["a"],"score":20}`,
	} {
		got, err := parseBreakdown(raw)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", raw, err)
		}
		if got.Score != 20 {
			t.Errorf("expected score 20, got %d", got.Score)
		}
	}
}

func TestParseBreakdownClampsScore(t *testing.T) {
	for _, tc := range []struct {
		name     string
		raw      string
		expected int
	}{
		{name: "over", expected: 100, raw: `{"summary":"s","score":140}`},
		{name: "under", expected: 0, raw: `{"summary":"s","score":-9}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseBreakdown(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Score != tc.expected {
				t.Errorf("expected score %d, got %d", tc.expected, got.Score)
			}
		})
	}
}

func TestParseBreakdownInvalid(t *testing.T) {
	if _, err := parseBreakdown("not json"); err == nil {
		t.Fatal("expected error for non-JSON input")
	}
}
