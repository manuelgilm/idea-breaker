package desktop

import (
	"context"
	"errors"
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

// SetAPIKey satisfies KeySetter; the fake ignores the key.
func (f fakeProvider) SetAPIKey(string) {}

// memoryStore is an in-memory SecretStore for tests.
type memoryStore struct {
	kv map[string]string
}

func (m *memoryStore) key(service, account string) string { return service + "\x00" + account }

func (m *memoryStore) Get(service, account string) (string, error) {
	if v, ok := m.kv[m.key(service, account)]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}

func (m *memoryStore) Set(service, account, password string) error {
	m.kv[m.key(service, account)] = password
	return nil
}

func (m *memoryStore) Delete(service, account string) error {
	delete(m.kv, m.key(service, account))
	return nil
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
	ks, _ := provider.(KeySetter)
	return New(service.New(store, engine.New(provider)), ks,
		WithSecretStore(&memoryStore{kv: map[string]string{}}),
		WithModel("test-model"),
	)
}

func TestListAndGetIdea(t *testing.T) {
	app := newTestApp(t, scripted(nil))

	idea, _, err := app.svc.CreateIdea(context.Background(), "Test idea", "", nil)
	require.NoError(t, err)

	ideas, err := app.ListIdeas("")
	require.NoError(t, err)
	require.Len(t, ideas, 1)
	assert.Equal(t, idea.ID, ideas[0].ID)

	got, err := app.GetIdea(idea.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test idea", got.Title)

	_, err = app.GetIdea("nope")
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestListIdeaCards(t *testing.T) {
	app := newTestApp(t, scripted(map[string]int{"critic": 4}))

	svc := app.svc
	idea, _, err := svc.CreateIdea(context.Background(), "Scored idea", "body", []string{"tag"})
	require.NoError(t, err)
	critic, err := svc.CreatePersona(context.Background(), "Critic", "marker-critic", 1)
	require.NoError(t, err)

	cards, err := app.ListIdeaCards()
	require.NoError(t, err)
	require.Len(t, cards, 1)
	assert.Nil(t, cards[0].Score) // no runs yet

	_, err = svc.Evaluate(context.Background(), idea.ID, []string{critic.ID}, false)
	require.NoError(t, err)

	cards, err = app.ListIdeaCards()
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.NotNil(t, cards[0].Score)
	assert.Equal(t, 80.0, *cards[0].Score)
	assert.Equal(t, []string{"tag"}, cards[0].Tags)
}

func TestCreateUpdateDeleteIdea(t *testing.T) {
	app := newTestApp(t, scripted(nil))

	idea, err := app.CreateIdea("New", "body", []string{"a"})
	require.NoError(t, err)
	assert.Equal(t, "New", idea.Title)

	updated, err := app.UpdateIdea(idea.ID, "Renamed", "new body", []string{"b"})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.Title)
	assert.Equal(t, "new body", updated.Body)
	assert.Equal(t, []string{"b"}, updated.Tags)

	require.NoError(t, app.DeleteIdea(idea.ID))
	_, err = app.GetIdea(idea.ID)
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestEvaluateAndListRuns(t *testing.T) {
	app := newTestApp(t, scripted(map[string]int{"critic": 4}))

	idea, _, err := app.svc.CreateIdea(context.Background(), "Test idea", "", nil)
	require.NoError(t, err)
	critic, err := app.svc.CreatePersona(context.Background(), "Critic", "marker-critic", 1)
	require.NoError(t, err)

	score, err := app.Evaluate(idea.ID, []string{critic.ID}, false)
	require.NoError(t, err)
	assert.Equal(t, 80.0, score.Total)
	assert.Equal(t, 1, score.Responded)

	runs, err := app.ListRuns(idea.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, score.RunID, runs[0].RunID)
}

func TestListPersonasMarksBuiltins(t *testing.T) {
	app := newTestApp(t, scripted(nil))

	custom, err := app.svc.CreatePersona(context.Background(), "Custom", "prompt", 1)
	require.NoError(t, err)

	views, err := app.ListPersonas()
	require.NoError(t, err)
	require.Len(t, views, 4) // skeptic, optimist, engineer, custom

	builtin := map[string]bool{}
	for _, v := range views {
		builtin[v.ID] = v.Builtin
	}
	assert.True(t, builtin["skeptic"])
	assert.True(t, builtin["optimist"])
	assert.True(t, builtin["engineer"])
	assert.False(t, builtin[custom.ID])

	created, err := app.CreatePersona("Second", "prompt", 1)
	require.NoError(t, err)
	require.NoError(t, app.DeletePersona(created.ID))
}

func TestCreateAndUpdatePersona(t *testing.T) {
	app := newTestApp(t, scripted(nil))

	created, err := app.CreatePersona("Critic", "be critical", 1)
	require.NoError(t, err)
	assert.Equal(t, 1, created.Version)
	assert.NotEmpty(t, created.ID)
	assert.False(t, created.Builtin)

	updated, err := app.UpdatePersona(created.ID, "Critic v2", "be even more critical", 2)
	require.NoError(t, err)
	assert.Equal(t, "Critic v2", updated.Name)
	assert.Equal(t, 2, updated.Version, "version auto-increments on update")
}

func TestAddAndListFeedback(t *testing.T) {
	app := newTestApp(t, scripted(nil))

	idea, _, err := app.svc.CreateIdea(context.Background(), "Test idea", "", nil)
	require.NoError(t, err)

	fb, err := app.AddFeedback(idea.ID, "Alice", 4, "Solid.", "market")
	require.NoError(t, err)
	assert.Equal(t, "Alice", fb.Author)

	all, err := app.ListFeedback(idea.ID)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, fb.ID, all[0].ID)
}

func TestResources(t *testing.T) {
	app := newTestApp(t, scripted(nil))

	idea, _, err := app.svc.CreateIdea(context.Background(), "Test idea", "", nil)
	require.NoError(t, err)

	r, err := app.AddResource(idea.ID, "https://example.com", "Example", "article", "useful")
	require.NoError(t, err)
	assert.Equal(t, "Example", r.Title)

	resources, err := app.ListResources(idea.ID)
	require.NoError(t, err)
	require.Len(t, resources, 1)

	require.NoError(t, app.DeleteResource(r.ID))
	resources, err = app.ListResources(idea.ID)
	require.NoError(t, err)
	require.Empty(t, resources)
}

func TestProviderInfoAndAPIKeys(t *testing.T) {
	app := newTestApp(t, scripted(nil))

	info, err := app.GetProviderInfo()
	require.NoError(t, err)
	assert.Equal(t, "openai", info.Provider)
	assert.Equal(t, "test-model", info.Model)

	keys, err := app.ListAPIKeys()
	require.NoError(t, err)
	require.Empty(t, keys)

	k1, err := app.AddAPIKey("work", "sk-abc1234567")
	require.NoError(t, err)
	assert.True(t, k1.IsDefault, "first key is default")
	assert.Equal(t, "••••4567", k1.Hint)
	assert.Equal(t, "work", k1.Label)

	k2, err := app.AddAPIKey("personal", "sk-zzz9876543")
	require.NoError(t, err)
	assert.False(t, k2.IsDefault)

	keys, err = app.ListAPIKeys()
	require.NoError(t, err)
	require.Len(t, keys, 2)

	require.NoError(t, app.SetDefaultAPIKey(k2.ID))
	keys, err = app.ListAPIKeys()
	require.NoError(t, err)
	for _, k := range keys {
		assert.Equal(t, k.ID == k2.ID, k.IsDefault, "only the selected key is default")
	}

	require.NoError(t, app.DeleteAPIKey(k2.ID))
	keys, err = app.ListAPIKeys()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.True(t, keys[0].IsDefault, "remaining key promoted to default")
}
