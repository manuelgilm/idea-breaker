package desktop

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"aibreak/internal/engine"
	"aibreak/internal/llm"
	"aibreak/internal/registry"
	"aibreak/internal/registry/sqlite"
	"aibreak/internal/service"
)

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

func newTestApp(t *testing.T, provider llm.Provider) *App {
	t.Helper()
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return New(service.New(store, engine.New(provider)))
}

func TestListAndGetIdea(t *testing.T) {
	ctx := context.Background()
	app := newTestApp(t, scripted(nil))

	svc := app.svc
	idea, _, err := svc.CreateIdea(ctx, "Test idea", "", nil)
	require.NoError(t, err)

	ideas, err := app.ListIdeas(ctx, "")
	require.NoError(t, err)
	require.Len(t, ideas, 1)
	assert.Equal(t, idea.ID, ideas[0].ID)

	got, err := app.GetIdea(ctx, idea.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test idea", got.Title)

	_, err = app.GetIdea(ctx, "nope")
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestEvaluateAndListRuns(t *testing.T) {
	ctx := context.Background()
	app := newTestApp(t, scripted(map[string]int{"critic": 4}))

	svc := app.svc
	idea, _, err := svc.CreateIdea(ctx, "Test idea", "", nil)
	require.NoError(t, err)
	_, err = svc.CreatePersona(ctx, "critic", "Critic", "marker-critic", 1, "1.0.0")
	require.NoError(t, err)

	score, err := app.Evaluate(ctx, idea.ID, []string{"critic"}, false)
	require.NoError(t, err)
	assert.Equal(t, 80.0, score.Total)
	assert.Equal(t, 1, score.Responded)

	runs, err := app.ListRuns(ctx, idea.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, score.RunID, runs[0].RunID)
}

func TestAddAndListFeedback(t *testing.T) {
	ctx := context.Background()
	app := newTestApp(t, scripted(nil))

	svc := app.svc
	idea, _, err := svc.CreateIdea(ctx, "Test idea", "", nil)
	require.NoError(t, err)

	fb, err := app.AddFeedback(ctx, idea.ID, "Alice", 4, "Solid.", "market")
	require.NoError(t, err)
	assert.Equal(t, "Alice", fb.Author)

	all, err := app.ListFeedback(ctx, idea.ID)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, fb.ID, all[0].ID)
}
