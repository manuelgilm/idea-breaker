package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"aibreak/internal/domain"
	"aibreak/internal/engine"
	"aibreak/internal/llm"
	"aibreak/internal/registry/sqlite"
)

func newTestService(t *testing.T, provider llm.Provider) *Service {
	t.Helper()
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return New(store, engine.New(provider))
}

type fakeProvider struct {
	fn func(ctx context.Context, req llm.Request) (llm.Response, error)
}

func (f fakeProvider) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	return f.fn(ctx, req)
}

// scripted returns a score keyed off the "marker-<id>" token in the system
// prompt.
func scripted(scores map[string]int) llm.Provider {
	return fakeProvider{fn: func(ctx context.Context, req llm.Request) (llm.Response, error) {
		sys := req.Messages[0].Content
		for id, score := range scores {
			if strings.Contains(sys, "marker-"+id) {
				return llm.Response{Content: fmt.Sprintf(`{"score":%d,"rationale":"ok"}`, score)}, nil
			}
		}
		return llm.Response{}, fmt.Errorf("unscripted: %q", sys)
	}}
}

func TestCreateIdeaValidation(t *testing.T) {
	s := newTestService(t, scripted(nil))
	_, _, err := s.CreateIdea(context.Background(), "", "body", nil)
	assert.ErrorIs(t, err, ErrValidation)
}

func TestCreateIdeaDuplicateTitleWarning(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t, scripted(nil))

	_, warning, err := s.CreateIdea(ctx, "Note App", "", nil)
	require.NoError(t, err)
	assert.Empty(t, warning, "no warning on first create")

	_, warning, err = s.CreateIdea(ctx, "  note APP ", "", nil)
	require.NoError(t, err)
	assert.Equal(t, duplicateTitleWarning, warning)
}

func TestUpdatePersona(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t, scripted(nil))

	_, err := s.CreatePersona(ctx, "critic", "Critic", "be critical", 1, "1.0.0")
	require.NoError(t, err)

	t.Run("partial merge preserves unprovided fields", func(t *testing.T) {
		name := "Critic v2"
		p, err := s.UpdatePersona(ctx, "critic", domain.PersonaPatch{Name: &name, Version: strptr("2.0.0")})
		require.NoError(t, err)
		assert.Equal(t, "Critic v2", p.Name)
		assert.Equal(t, "be critical", p.SystemPrompt, "prompt preserved")
		assert.Equal(t, 1.0, p.Weight, "weight preserved")
		assert.Equal(t, "2.0.0", p.Version)
	})

	t.Run("empty name rejected", func(t *testing.T) {
		empty := ""
		_, err := s.UpdatePersona(ctx, "critic", domain.PersonaPatch{Name: &empty, Version: strptr("3.0.0")})
		assert.ErrorIs(t, err, ErrValidation)
	})

	t.Run("empty prompt rejected", func(t *testing.T) {
		empty := ""
		_, err := s.UpdatePersona(ctx, "critic", domain.PersonaPatch{SystemPrompt: &empty, Version: strptr("3.0.0")})
		assert.ErrorIs(t, err, ErrValidation)
	})

	t.Run("negative weight rejected", func(t *testing.T) {
		neg := -1.0
		_, err := s.UpdatePersona(ctx, "critic", domain.PersonaPatch{Weight: &neg, Version: strptr("3.0.0")})
		assert.ErrorIs(t, err, ErrValidation)
	})

	t.Run("missing version rejected", func(t *testing.T) {
		name := "Nope"
		_, err := s.UpdatePersona(ctx, "critic", domain.PersonaPatch{Name: &name})
		assert.ErrorIs(t, err, ErrValidation)
	})

	t.Run("unchanged version rejected", func(t *testing.T) {
		all, err := s.ListPersonas(ctx)
		require.NoError(t, err)
		var current string
		for _, p := range all {
			if p.ID == "critic" {
				current = p.Version
			}
		}
		require.NotEmpty(t, current)
		name := "Nope"
		_, err = s.UpdatePersona(ctx, "critic", domain.PersonaPatch{Name: &name, Version: &current})
		assert.ErrorIs(t, err, ErrValidation)
	})

	t.Run("built-in rejected", func(t *testing.T) {
		prompt := "new prompt"
		_, err := s.UpdatePersona(ctx, "skeptic", domain.PersonaPatch{SystemPrompt: &prompt, Version: strptr("9.0.0")})
		assert.ErrorIs(t, err, ErrConflict)
	})

	t.Run("unknown id", func(t *testing.T) {
		name := "Nope"
		_, err := s.UpdatePersona(ctx, "nope", domain.PersonaPatch{Name: &name, Version: strptr("1.0.0")})
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func strptr(s string) *string { return &s }

func TestUpdateIdeaPartial(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t, scripted(nil))

	idea, _, err := s.CreateIdea(ctx, "X", "original body", []string{"a"})
	require.NoError(t, err)

	title := "Y"
	updated, err := s.UpdateIdea(ctx, idea.ID, domain.IdeaPatch{Title: &title})
	require.NoError(t, err)
	assert.Equal(t, "Y", updated.Title)
	assert.Equal(t, "original body", updated.Body, "body preserved")

	empty := ""
	_, err = s.UpdateIdea(ctx, idea.ID, domain.IdeaPatch{Title: &empty})
	assert.ErrorIs(t, err, ErrValidation)

	_, err = s.UpdateIdea(ctx, "nope", domain.IdeaPatch{})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCreatePersonaValidation(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t, scripted(nil))

	_, err := s.CreatePersona(ctx, "Bad Slug", "x", "p", 1, "")
	assert.ErrorIs(t, err, ErrValidation)

	_, err = s.CreatePersona(ctx, "ok", "x", "p", -1, "")
	assert.ErrorIs(t, err, ErrValidation)

	_, err = s.CreatePersona(ctx, "ok", "", "p", 1, "")
	assert.ErrorIs(t, err, ErrValidation)

	p, err := s.CreatePersona(ctx, "ok", "x", "p", 1, "")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", p.Version, "version defaults to 1.0.0")

	_, err = s.CreatePersona(ctx, "ok", "x", "p", 1, "")
	assert.ErrorIs(t, err, ErrConflict, "duplicate id")
}

func TestAddFeedbackValidation(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t, scripted(nil))
	idea, _, err := s.CreateIdea(ctx, "X", "", nil)
	require.NoError(t, err)

	_, err = s.AddFeedback(ctx, idea.ID, "", 3, "", "")
	assert.ErrorIs(t, err, ErrValidation)

	_, err = s.AddFeedback(ctx, idea.ID, "a", 6, "", "")
	assert.ErrorIs(t, err, ErrValidation)

	_, err = s.AddFeedback(ctx, "nope", "a", 3, "", "")
	assert.ErrorIs(t, err, ErrNotFound)

	f, err := s.AddFeedback(ctx, idea.ID, "a", 3, "note", "market")
	require.NoError(t, err)
	assert.Equal(t, "a", f.Author)
}

func TestAddResourceValidation(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t, scripted(nil))
	idea, _, err := s.CreateIdea(ctx, "X", "", nil)
	require.NoError(t, err)

	_, err = s.AddResource(ctx, idea.ID, "not-a-url", "", "", "")
	assert.ErrorIs(t, err, ErrValidation)

	_, err = s.AddResource(ctx, idea.ID, "https://x.com", "", "bogus", "")
	assert.ErrorIs(t, err, ErrValidation)

	_, err = s.AddResource(ctx, "nope", "https://x.com", "", "", "")
	assert.ErrorIs(t, err, ErrNotFound)

	r, err := s.AddResource(ctx, idea.ID, "https://x.com", "title", "article", "note")
	require.NoError(t, err)
	assert.Equal(t, "article", r.Kind)
}

func TestEvaluate(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t, scripted(map[string]int{"a": 3, "b": 5}))

	idea, _, err := s.CreateIdea(ctx, "X", "body", nil)
	require.NoError(t, err)
	_, err = s.CreatePersona(ctx, "a", "a", "marker-a", 1, "1.0.0")
	require.NoError(t, err)
	_, err = s.CreatePersona(ctx, "b", "b", "marker-b", 1, "1.0.0")
	require.NoError(t, err)

	score, err := s.Evaluate(ctx, idea.ID, []string{"a", "b"}, false)
	require.NoError(t, err)
	assert.Equal(t, 80.0, score.Total) // (3+5)/2 = 4 -> 80

	runs, err := s.ListRuns(ctx, idea.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, score.RunID, runs[0].RunID)
	require.Len(t, runs[0].Breakdown, 2)
}

func TestEvaluateEdgeCases(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t, scripted(map[string]int{"a": 3}))

	_, err := s.Evaluate(ctx, "nope", nil, false)
	assert.ErrorIs(t, err, ErrNotFound)

	idea, _, err := s.CreateIdea(ctx, "X", "", nil)
	require.NoError(t, err)
	_, err = s.CreatePersona(ctx, "a", "a", "marker-a", 1, "1.0.0")
	require.NoError(t, err)

	_, err = s.Evaluate(ctx, idea.ID, []string{"missing"}, false)
	assert.ErrorIs(t, err, ErrValidation)
}

// scriptedWithSynthesis behaves like scripted for persona calls and returns
// the given synthesis content for the synthesizer call.
func scriptedWithSynthesis(scores map[string]int, synthesis string) llm.Provider {
	return fakeProvider{fn: func(ctx context.Context, req llm.Request) (llm.Response, error) {
		sys := req.Messages[0].Content
		if strings.Contains(sys, "SYNTHESIZER") {
			return llm.Response{Content: synthesis}, nil
		}
		for id, score := range scores {
			if strings.Contains(sys, "marker-"+id) {
				return llm.Response{Content: fmt.Sprintf(`{"score":%d,"rationale":"ok"}`, score)}, nil
			}
		}
		return llm.Response{}, fmt.Errorf("unscripted: %q", sys)
	}}
}

func TestEvaluateWithSummary(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t, scriptedWithSynthesis(
		map[string]int{"a": 3, "b": 5},
		`{"verdict":"promising","summary":"Good upside, some risk."}`,
	))

	idea, _, err := s.CreateIdea(ctx, "X", "body", nil)
	require.NoError(t, err)
	_, err = s.CreatePersona(ctx, "a", "a", "marker-a", 1, "1.0.0")
	require.NoError(t, err)
	_, err = s.CreatePersona(ctx, "b", "b", "marker-b", 1, "1.0.0")
	require.NoError(t, err)

	score, err := s.Evaluate(ctx, idea.ID, []string{"a", "b"}, true)
	require.NoError(t, err)
	assert.Equal(t, "promising", score.Verdict)
	assert.Equal(t, "Good upside, some risk.", score.Summary)

	runs, err := s.ListRuns(ctx, idea.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, "promising", runs[0].Verdict)
	assert.Equal(t, "Good upside, some risk.", runs[0].Summary)
}
