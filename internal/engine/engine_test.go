package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"aibreak/internal/domain"
	"aibreak/internal/llm"
)

type fakeProvider struct {
	fn func(ctx context.Context, req llm.Request) (llm.Response, error)
}

func (f fakeProvider) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	return f.fn(ctx, req)
}

func persona(id string, weight float64) domain.Persona {
	return domain.Persona{
		ID:           id,
		Name:         id,
		SystemPrompt: "marker-" + id,
		Weight:       weight,
		Version:      "1.0.0",
	}
}

func idea() domain.Idea {
	return domain.Idea{ID: "idea-1", Title: "An idea"}
}

// scripted maps a persona id to a JSON score; ids absent from the map fail
// with a non-retryable auth error.
func scripted(scores map[string]int, failures map[string]error) llm.Provider {
	return fakeProvider{fn: func(ctx context.Context, req llm.Request) (llm.Response, error) {
		sys := req.Messages[0].Content
		for id, score := range scores {
			if strings.Contains(sys, "marker-"+id) {
				return llm.Response{Content: fmt.Sprintf(`{"score":%d,"rationale":"ok"}`, score)}, nil
			}
		}
		for id, err := range failures {
			if strings.Contains(sys, "marker-"+id) {
				return llm.Response{}, err
			}
		}
		return llm.Response{}, fmt.Errorf("unscripted system prompt: %q", sys)
	}}
}

func TestEvaluateAggregates(t *testing.T) {
	t.Run("default weights", func(t *testing.T) {
		p := scripted(map[string]int{"skeptic": 2, "optimist": 4, "engineer": 3}, nil)
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("skeptic", 1), persona("optimist", 1), persona("engineer", 1)}, false)
		require.NoError(t, err)
		assert.Equal(t, 60.0, score.Total)
		assert.Equal(t, 3, score.Requested)
		assert.Equal(t, 3, score.Responded)
	})

	t.Run("zero-weight persona contributes nothing", func(t *testing.T) {
		p := scripted(map[string]int{"a": 5, "b": 0}, nil)
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("a", 1), persona("b", 0)}, false)
		require.NoError(t, err)
		// Only "a" counts: 100 * (5*1 / (5*1)) = 100.
		assert.Equal(t, 100.0, score.Total)
	})

	t.Run("failed persona excluded but counted in coverage", func(t *testing.T) {
		p := scripted(map[string]int{"good": 4}, map[string]error{"bad": llm.ErrAuth})
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("good", 1), persona("bad", 1)}, false)
		require.NoError(t, err)
		assert.Equal(t, 2, score.Requested)
		assert.Equal(t, 1, score.Responded)
		assert.Equal(t, 80.0, score.Total) // 100 * (4/5)
	})

	t.Run("all personas fail", func(t *testing.T) {
		p := scripted(nil, map[string]error{"a": llm.ErrAuth, "b": llm.ErrProvider})
		e := New(p)
		_, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("a", 1), persona("b", 1)}, false)
		assert.ErrorIs(t, err, ErrNoResults)
	})

	t.Run("all successful personas zero weight", func(t *testing.T) {
		p := scripted(map[string]int{"a": 3, "b": 4}, nil)
		e := New(p)
		_, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("a", 0), persona("b", 0)}, false)
		assert.ErrorIs(t, err, ErrZeroWeight)
	})

	t.Run("empty personas", func(t *testing.T) {
		e := New(scripted(nil, nil))
		_, err := e.Evaluate(context.Background(), idea(), nil, false)
		assert.ErrorIs(t, err, ErrEmptyPersonas)
	})

	t.Run("spread all agree", func(t *testing.T) {
		p := scripted(map[string]int{"a": 3, "b": 3}, nil)
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("a", 1), persona("b", 1)}, false)
		require.NoError(t, err)
		assert.Equal(t, 0.0, score.Spread)
	})

	t.Run("spread 1 vs 5 is divided", func(t *testing.T) {
		p := scripted(map[string]int{"skeptic": 1, "optimist": 5, "engineer": 3}, nil)
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("skeptic", 1), persona("optimist", 1), persona("engineer", 1)}, false)
		require.NoError(t, err)
		assert.Equal(t, 4.0, score.Spread)
	})

	t.Run("spread single success excludes failures", func(t *testing.T) {
		p := scripted(map[string]int{"good": 4}, map[string]error{"bad": llm.ErrAuth})
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("good", 1), persona("bad", 1)}, false)
		require.NoError(t, err)
		assert.Equal(t, 0.0, score.Spread)
	})
}

func TestAgreement(t *testing.T) {
	assert.Equal(t, "consensus", Agreement(0))
	assert.Equal(t, "consensus", Agreement(1))
	assert.Equal(t, "mixed", Agreement(2))
	assert.Equal(t, "mixed", Agreement(3))
	assert.Equal(t, "divided", Agreement(4))
	assert.Equal(t, "divided", Agreement(5))
}

func TestEvaluateRecordsPersonaVersionAndWeight(t *testing.T) {
	p := scripted(map[string]int{"a": 3}, nil)
	e := New(p)
	score, err := e.Evaluate(context.Background(), idea(),
		[]domain.Persona{{ID: "a", Name: "a", SystemPrompt: "marker-a", Weight: 2, Version: "1.2.0"}}, false)
	require.NoError(t, err)
	require.Len(t, score.Breakdown, 1)
	assert.Equal(t, "1.2.0", score.Breakdown[0].PersonaVersion)
	assert.Equal(t, 2.0, score.Breakdown[0].Weight)
}

func TestEvaluateRetry(t *testing.T) {
	t.Run("retryable error then success", func(t *testing.T) {
		var mu sync.Mutex
		calls := 0
		p := fakeProvider{fn: func(ctx context.Context, req llm.Request) (llm.Response, error) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if calls == 1 {
				return llm.Response{}, llm.ErrProvider
			}
			return llm.Response{Content: `{"score":4,"rationale":"ok"}`}, nil
		}}
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(), []domain.Persona{persona("a", 1)}, false)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusSuccess, score.Breakdown[0].Status)
		assert.Equal(t, 2, calls)
	})

	t.Run("unparseable then success", func(t *testing.T) {
		var mu sync.Mutex
		calls := 0
		p := fakeProvider{fn: func(ctx context.Context, req llm.Request) (llm.Response, error) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if calls == 1 {
				return llm.Response{Content: `not json`}, nil
			}
			return llm.Response{Content: `{"score":3,"rationale":"ok"}`}, nil
		}}
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(), []domain.Persona{persona("a", 1)}, false)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusSuccess, score.Breakdown[0].Status)
		assert.Equal(t, 2, calls)
	})

	t.Run("non-retryable auth fails immediately", func(t *testing.T) {
		var mu sync.Mutex
		calls := 0
		p := fakeProvider{fn: func(ctx context.Context, req llm.Request) (llm.Response, error) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			return llm.Response{}, llm.ErrAuth
		}}
		e := New(p)
		_, err := e.Evaluate(context.Background(), idea(), []domain.Persona{persona("a", 1)}, false)
		assert.ErrorIs(t, err, ErrNoResults)
		assert.Equal(t, 1, calls)
	})
}

func TestParseEvaluation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		score   int
		wantErr bool
	}{
		{"valid", `{"score":3,"rationale":"ok"}`, 3, false},
		{"float score", `{"score":3.5,"rationale":"ok"}`, 0, true},
		{"out of range high", `{"score":6,"rationale":"ok"}`, 0, true},
		{"out of range low", `{"score":-1,"rationale":"ok"}`, 0, true},
		{"missing score", `{"rationale":"ok"}`, 0, true},
		{"extra key", `{"score":3,"rationale":"ok","extra":1}`, 0, true},
		{"non-json", `hello`, 0, true},
		{"trailing data", `{"score":3,"rationale":"ok"} extra`, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, rationale, err := parseEvaluation(tt.content)
			if tt.wantErr {
				assert.ErrorIs(t, err, errUnparseable)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.score, score)
			assert.Equal(t, "ok", rationale)
		})
	}
}

func TestParseSynthesis(t *testing.T) {
	tests := []struct {
		name    string
		content string
		verdict string
		summary string
		wantErr bool
	}{
		{"valid", `{"verdict":"promising","summary":"Solid idea with risks."}`, "promising", "Solid idea with risks.", false},
		{"unknown verdict", `{"verdict":"maybe","summary":"x"}`, "", "", true},
		{"empty summary", `{"verdict":"pass","summary":""}`, "", "", true},
		{"whitespace summary", `{"verdict":"pass","summary":"   "}`, "", "", true},
		{"missing verdict", `{"summary":"x"}`, "", "", true},
		{"extra key", `{"verdict":"pass","summary":"x","score":3}`, "", "", true},
		{"non-json", `hello`, "", "", true},
		{"trailing data", `{"verdict":"pass","summary":"x"} extra`, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict, summary, err := parseSynthesis(tt.content)
			if tt.wantErr {
				assert.ErrorIs(t, err, errUnparseableSynthesis)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.verdict, verdict)
			assert.Equal(t, tt.summary, summary)
		})
	}
}

// synthScripted behaves like scripted for persona calls but returns the given
// synthesis content (or error) for the synthesizer call. Unmatched persona
// ids fail with a non-retryable auth error.
func synthScripted(scores map[string]int, synthesis string, synthErr error) llm.Provider {
	return fakeProvider{fn: func(ctx context.Context, req llm.Request) (llm.Response, error) {
		sys := req.Messages[0].Content
		if strings.Contains(sys, "SYNTHESIZER") {
			if synthErr != nil {
				return llm.Response{}, synthErr
			}
			return llm.Response{Content: synthesis}, nil
		}
		for id, score := range scores {
			if strings.Contains(sys, "marker-"+id) {
				return llm.Response{Content: fmt.Sprintf(`{"score":%d,"rationale":"ok"}`, score)}, nil
			}
		}
		return llm.Response{}, llm.ErrAuth
	}}
}

func TestEvaluateSummarize(t *testing.T) {
	t.Run("full run populates summary and verdict", func(t *testing.T) {
		p := synthScripted(map[string]int{"a": 3, "b": 5},
			`{"verdict":"promising","summary":"Good upside, some risk."}`, nil)
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("a", 1), persona("b", 1)}, true)
		require.NoError(t, err)
		assert.Equal(t, "promising", score.Verdict)
		assert.Equal(t, "Good upside, some risk.", score.Summary)
	})

	t.Run("not requested leaves fields empty", func(t *testing.T) {
		p := synthScripted(map[string]int{"a": 3},
			`{"verdict":"promising","summary":"unused"}`, nil)
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("a", 1)}, false)
		require.NoError(t, err)
		assert.Empty(t, score.Verdict)
		assert.Empty(t, score.Summary)
	})

	t.Run("partial run forces inconclusive", func(t *testing.T) {
		p := synthScripted(map[string]int{"good": 4},
			`{"verdict":"promising","summary":"Mostly good."}`, nil)
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("good", 1), persona("bad", 1)}, true)
		require.NoError(t, err)
		assert.Equal(t, 2, score.Requested)
		assert.Equal(t, 1, score.Responded)
		assert.Equal(t, "inconclusive", score.Verdict)
		assert.Equal(t, "Mostly good.", score.Summary)
	})

	t.Run("failed synthesis is best-effort", func(t *testing.T) {
		p := synthScripted(map[string]int{"a": 3}, `not json`, nil)
		e := New(p)
		score, err := e.Evaluate(context.Background(), idea(),
			[]domain.Persona{persona("a", 1)}, true)
		require.NoError(t, err)
		assert.Empty(t, score.Verdict)
		assert.Empty(t, score.Summary)
		assert.Equal(t, 1, score.Responded)
	})
}
