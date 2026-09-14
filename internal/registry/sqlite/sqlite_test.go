package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"aibreak/internal/domain"
	"aibreak/internal/registry"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func newID() string { return ulid.Make().String() }

func now() time.Time { return time.Now().UTC() }

func sampleIdea(title string) domain.Idea {
	return domain.Idea{ID: newID(), Title: title, Body: "body", Tags: []string{"t1", "t2"}, Created: now(), Updated: now()}
}

func TestIdeaRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	idea := sampleIdea("X")
	created, err := s.CreateIdea(ctx, idea)
	require.NoError(t, err)
	assert.Equal(t, idea.ID, created.ID)

	got, err := s.GetIdea(ctx, idea.ID)
	require.NoError(t, err)
	assert.Equal(t, idea.Title, got.Title)
	assert.Equal(t, idea.Body, got.Body)
	assert.Equal(t, idea.Tags, got.Tags)

	updated := got
	updated.Title = "Y"
	updated.Updated = now()
	res, err := s.UpdateIdea(ctx, updated)
	require.NoError(t, err)
	assert.Equal(t, "Y", res.Title)
	assert.Equal(t, idea.Body, res.Body, "body preserved on partial-equivalent update")

	err = s.DeleteIdea(ctx, idea.ID)
	require.NoError(t, err)

	_, err = s.GetIdea(ctx, idea.ID)
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestIdeaNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, err := s.GetIdea(ctx, "nope")
	assert.ErrorIs(t, err, registry.ErrNotFound)
	err = s.DeleteIdea(ctx, "nope")
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestFindIdeasByTitle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, title := range []string{"Note App", "note app", "Other"} {
		i := sampleIdea(title)
		_, err := s.CreateIdea(ctx, i)
		require.NoError(t, err)
	}

	got, err := s.FindIdeasByTitle(ctx, "  NOTE APP ")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestListIdeasTagFilter(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	a := sampleIdea("A")
	a.Tags = []string{"foo", "bar"}
	b := sampleIdea("B")
	b.Tags = []string{"baz"}
	_, err := s.CreateIdea(ctx, a)
	require.NoError(t, err)
	_, err = s.CreateIdea(ctx, b)
	require.NoError(t, err)

	all, err := s.ListIdeas(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 2)

	foo, err := s.ListIdeas(ctx, "foo")
	require.NoError(t, err)
	require.Len(t, foo, 1)
	assert.Equal(t, "A", foo[0].Title)
}

func TestIdeaDeleteCascades(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	idea := sampleIdea("X")
	_, err := s.CreateIdea(ctx, idea)
	require.NoError(t, err)

	score := domain.FeasibilityScore{RunID: newID(), IdeaID: idea.ID, Total: 60, Requested: 1, Responded: 1, Created: now()}
	ev := domain.Evaluation{RunID: score.RunID, IdeaID: idea.ID, PersonaID: "p", PersonaVersion: 1, Weight: 1, Status: domain.StatusSuccess, Score: 3, Created: now()}
	require.NoError(t, s.SaveRun(ctx, score, []domain.Evaluation{ev}))

	_, err = s.AddFeedback(ctx, domain.Feedback{ID: newID(), IdeaID: idea.ID, Author: "a", Score: 4, Created: now()})
	require.NoError(t, err)
	_, err = s.AddResource(ctx, domain.Resource{ID: newID(), IdeaID: idea.ID, URL: "https://x", Created: now()})
	require.NoError(t, err)

	require.NoError(t, s.DeleteIdea(ctx, idea.ID))

	runs, err := s.ListRuns(ctx, idea.ID)
	require.NoError(t, err)
	assert.Empty(t, runs)

	fb, err := s.ListFeedback(ctx, idea.ID)
	require.NoError(t, err)
	assert.Empty(t, fb)

	res, err := s.ListResources(ctx, idea.ID)
	require.NoError(t, err)
	assert.Empty(t, res)
}

// TestMigrationV1ToV2 builds a database with only the v1 schema applied,
// then opens it through the real path to verify the v2 migration adds the
// summary/verdict columns without touching existing rows.
func TestMigrationV1ToV2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	require.NoError(t, err)
	_, err = db.Exec(migrations[0])
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (1, ?)`,
		time.Now().UTC().Format(time.RFC3339))
	require.NoError(t, err)

	ideaID, runID := newID(), newID()
	_, err = db.Exec(`INSERT INTO ideas (id, title, body, tags, created_at, updated_at) VALUES (?, 'X', '', '[]', ?, ?)`,
		ideaID, now().Format(time.RFC3339), now().Format(time.RFC3339))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO runs (run_id, idea_id, total, requested, responded, created_at) VALUES (?, ?, 60, 1, 1, ?)`,
		runID, ideaID, now().Format(time.RFC3339))
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, err := Open(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Close())
		require.NoError(t, os.Remove(path))
	}()

	runs, err := s.ListRuns(context.Background(), ideaID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, 60.0, runs[0].Total)
	assert.Empty(t, runs[0].Summary)
	assert.Empty(t, runs[0].Verdict)
}

func TestUpdatePersona(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	custom := domain.Persona{ID: "critic", Name: "Critic", SystemPrompt: "p", Weight: 1, Version: 1, Created: now()}
	_, err := s.CreatePersona(ctx, custom)
	require.NoError(t, err)

	updated, err := s.UpdatePersona(ctx, domain.Persona{ID: "critic", Name: "Critic v2", SystemPrompt: "p2", Weight: 2, Version: 2})
	require.NoError(t, err)
	assert.Equal(t, "Critic v2", updated.Name)
	assert.Equal(t, "p2", updated.SystemPrompt)
	assert.Equal(t, 2.0, updated.Weight)
	assert.Equal(t, 2, updated.Version)

	got, err := s.GetPersona(ctx, "critic")
	require.NoError(t, err)
	assert.Equal(t, 2, got.Version)

	_, err = s.UpdatePersona(ctx, domain.Persona{ID: "skeptic", Name: "x", Version: 9})
	assert.ErrorIs(t, err, registry.ErrConflict, "built-ins are not editable")

	_, err = s.UpdatePersona(ctx, domain.Persona{ID: "nope", Version: 1})
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestMigrationV2ToV3(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v2.db")

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	require.NoError(t, err)
	for i, m := range []string{migrations[0], migrations[1]} {
		_, err = db.Exec(m)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			i+1, time.Now().UTC().Format(time.RFC3339))
		require.NoError(t, err)
	}

	ideaID, runID := newID(), newID()
	_, err = db.Exec(`INSERT INTO ideas (id, title, body, tags, created_at, updated_at) VALUES (?, 'X', '', '[]', ?, ?)`,
		ideaID, now().Format(time.RFC3339), now().Format(time.RFC3339))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO runs (run_id, idea_id, total, requested, responded, summary, verdict, created_at) VALUES (?, ?, 60, 2, 2, 's', 'promising', ?)`,
		runID, ideaID, now().Format(time.RFC3339))
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, err := Open(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Close())
		require.NoError(t, os.Remove(path))
	}()

	runs, err := s.ListRuns(context.Background(), ideaID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, 60.0, runs[0].Total)
	assert.Equal(t, 0.0, runs[0].Spread, "pre-v3 rows default to spread 0")
}

func TestSaveRunPersistsSpread(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	idea := sampleIdea("X")
	_, err := s.CreateIdea(ctx, idea)
	require.NoError(t, err)

	runID := newID()
	score := domain.FeasibilityScore{RunID: runID, IdeaID: idea.ID, Total: 60, Requested: 2, Responded: 2, Spread: 4, Created: now()}
	evals := []domain.Evaluation{
		{RunID: runID, IdeaID: idea.ID, PersonaID: "a", PersonaVersion: 1, Weight: 1, Status: domain.StatusSuccess, Score: 1, Created: now()},
		{RunID: runID, IdeaID: idea.ID, PersonaID: "b", PersonaVersion: 1, Weight: 1, Status: domain.StatusSuccess, Score: 5, Created: now()},
	}
	require.NoError(t, s.SaveRun(ctx, score, evals))

	runs, err := s.ListRuns(ctx, idea.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, 4.0, runs[0].Spread)
}

func TestSaveRunPersistsSummaryVerdict(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	idea := sampleIdea("X")
	_, err := s.CreateIdea(ctx, idea)
	require.NoError(t, err)

	runID := newID()
	score := domain.FeasibilityScore{
		RunID: runID, IdeaID: idea.ID, Total: 75, Requested: 2, Responded: 2,
		Summary: "Good upside, some risk.", Verdict: domain.VerdictPromising,
		Created: now(),
	}
	evals := []domain.Evaluation{
		{RunID: runID, IdeaID: idea.ID, PersonaID: "a", PersonaVersion: 1, Weight: 1, Status: domain.StatusSuccess, Score: 3, Rationale: "ok", Created: now()},
	}
	require.NoError(t, s.SaveRun(ctx, score, evals))

	runs, err := s.ListRuns(ctx, idea.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, "Good upside, some risk.", runs[0].Summary)
	assert.Equal(t, domain.VerdictPromising, runs[0].Verdict)
}

func TestPersonaCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	custom := domain.Persona{ID: "investor", Name: "Investor", SystemPrompt: "p", Weight: 1.5, Version: 1, Created: now()}
	created, err := s.CreatePersona(ctx, custom)
	require.NoError(t, err)
	assert.Equal(t, "investor", created.ID)
	assert.Equal(t, 1, created.Version)
	assert.False(t, created.Created.IsZero())

	got, err := s.GetPersona(ctx, "investor")
	require.NoError(t, err)
	assert.Equal(t, "Investor", got.Name)

	all, err := s.ListPersonas(ctx)
	require.NoError(t, err)
	// built-ins + custom
	assert.Len(t, all, 4)

	// duplicate id -> conflict
	_, err = s.CreatePersona(ctx, custom)
	assert.ErrorIs(t, err, registry.ErrConflict)

	// built-in not deletable
	err = s.DeletePersona(ctx, "skeptic")
	assert.ErrorIs(t, err, registry.ErrConflict)

	// custom deletable
	require.NoError(t, s.DeletePersona(ctx, "investor"))
	_, err = s.GetPersona(ctx, "investor")
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

func TestSaveRunAndListRuns(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	idea := sampleIdea("X")
	_, err := s.CreateIdea(ctx, idea)
	require.NoError(t, err)

	runID := newID()
	score := domain.FeasibilityScore{RunID: runID, IdeaID: idea.ID, Total: 70, Requested: 2, Responded: 1, Created: now()}
	evals := []domain.Evaluation{
		{RunID: runID, IdeaID: idea.ID, PersonaID: "good", PersonaVersion: 1, Weight: 1, Status: domain.StatusSuccess, Score: 3, Rationale: "ok", Created: now()},
		{RunID: runID, IdeaID: idea.ID, PersonaID: "bad", PersonaVersion: 1, Weight: 1, Status: domain.StatusFailed, Error: "boom", Created: now()},
	}
	require.NoError(t, s.SaveRun(ctx, score, evals))

	runs, err := s.ListRuns(ctx, idea.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, 70.0, runs[0].Total)
	assert.Equal(t, 2, runs[0].Requested)
	assert.Equal(t, 1, runs[0].Responded)
	require.Len(t, runs[0].Breakdown, 2)
	// breakdown ordered by persona_id ascending
	assert.Equal(t, "bad", runs[0].Breakdown[0].PersonaID)
	assert.Equal(t, domain.StatusFailed, runs[0].Breakdown[0].Status)
	assert.Equal(t, "good", runs[0].Breakdown[1].PersonaID)
	assert.Equal(t, domain.StatusSuccess, runs[0].Breakdown[1].Status)
	assert.Equal(t, 3, runs[0].Breakdown[1].Score)
}

func TestProviderKeys(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	k1 := domain.APIKey{ID: newID(), Provider: "openai", Label: "work", Hint: "••••4567", IsDefault: true, Created: now()}
	k2 := domain.APIKey{ID: newID(), Provider: "openai", Label: "personal", Hint: "••••6543", Created: now()}

	created1, err := s.AddProviderKey(ctx, k1)
	require.NoError(t, err)
	assert.Equal(t, k1.ID, created1.ID)

	_, err = s.AddProviderKey(ctx, k2)
	require.NoError(t, err)

	keys, err := s.ListProviderKeys(ctx, "openai")
	require.NoError(t, err)
	require.Len(t, keys, 2)

	def, err := s.GetDefaultProviderKey(ctx, "openai")
	require.NoError(t, err)
	assert.Equal(t, k1.ID, def.ID)

	require.NoError(t, s.SetDefaultProviderKey(ctx, k2.ID))
	def, err = s.GetDefaultProviderKey(ctx, "openai")
	require.NoError(t, err)
	assert.Equal(t, k2.ID, def.ID)

	require.NoError(t, s.DeleteProviderKey(ctx, k1.ID))
	_, err = s.GetProviderKey(ctx, k1.ID)
	assert.ErrorIs(t, err, registry.ErrNotFound)
}

// TestMigrationV3ToV4 builds a v3 database with a semver persona version, then
// opens it through the real path to verify v4 converts version TEXT -> INTEGER.
func TestMigrationV3ToV4(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v3.db")

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	require.NoError(t, err)
	for i, m := range []string{migrations[0], migrations[1], migrations[2]} {
		_, err = db.Exec(m)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			i+1, now().Format(time.RFC3339))
		require.NoError(t, err)
	}
	_, err = db.Exec(`INSERT INTO personas (id, name, system_prompt, weight, version, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"custom", "Custom", "p", 1.0, "1.0.0", now().Format(time.RFC3339))
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, err := Open(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Close())
		require.NoError(t, os.Remove(path))
	}()

	p, err := s.GetPersona(context.Background(), "custom")
	require.NoError(t, err)
	assert.Equal(t, 1, p.Version, "semver '1.0.0' migrated to integer 1")
}

// TestMigrationV4ToV5 builds a v4 database, then opens it through the real
// path to verify v5 creates the provider_keys table.
func TestMigrationV4ToV5(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v4.db")

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	require.NoError(t, err)
	for i, m := range []string{migrations[0], migrations[1], migrations[2], migrations[3]} {
		_, err = db.Exec(m)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			i+1, now().Format(time.RFC3339))
		require.NoError(t, err)
	}
	require.NoError(t, db.Close())

	s, err := Open(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Close())
		require.NoError(t, os.Remove(path))
	}()

	keys, err := s.ListProviderKeys(context.Background(), "openai")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	v, err := s.GetSetting(ctx, "missing")
	require.NoError(t, err)
	assert.Empty(t, v, "absent key returns empty string")

	require.NoError(t, s.SetSetting(ctx, "openai.model", "gpt-4o"))
	v, err = s.GetSetting(ctx, "openai.model")
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o", v)

	// Upsert: same key overwrites.
	require.NoError(t, s.SetSetting(ctx, "openai.model", "gpt-4.1"))
	v, err = s.GetSetting(ctx, "openai.model")
	require.NoError(t, err)
	assert.Equal(t, "gpt-4.1", v)
}
