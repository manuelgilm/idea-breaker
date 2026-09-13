package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"aibreak/internal/engine"
	"aibreak/internal/llm"
	"aibreak/internal/registry/sqlite"
	"aibreak/internal/service"
)

type fixedProvider struct{}

func (fixedProvider) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	if strings.Contains(req.Messages[0].Content, "SYNTHESIZER") {
		return llm.Response{Content: `{"verdict":"promising","summary":"Good upside, some risk."}`}, nil
	}
	return llm.Response{Content: `{"score":3,"rationale":"ok"}`}, nil
}

type cliEnv struct {
	t   *testing.T
	svc *service.Service
}

func newCLIEnv(t *testing.T) *cliEnv {
	t.Helper()
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return &cliEnv{t: t, svc: service.New(store, engine.New(fixedProvider{}))}
}

// run executes a fresh root command (sharing the store-backed service) and
// returns stdout/stderr and the error.
func (e *cliEnv) run(args ...string) (string, string, error) {
	root := New(e.svc)
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errOut.String(), err
}

func TestRegistryAdd(t *testing.T) {
	env := newCLIEnv(t)
	out, _, err := env.run("registry", "add", "--title", "X")
	require.NoError(t, err)
	assert.NotEmpty(t, out)
}

func TestRegistryAddMissingTitle(t *testing.T) {
	env := newCLIEnv(t)
	_, _, err := env.run("registry", "add")
	assert.Error(t, err)
}

func TestRegistryAddDuplicateTitleWarning(t *testing.T) {
	env := newCLIEnv(t)
	_, _, err := env.run("registry", "add", "--title", "Note App")
	require.NoError(t, err)

	out, stderr, err := env.run("registry", "add", "--title", "note app")
	require.NoError(t, err)
	assert.NotEmpty(t, out)
	assert.Contains(t, stderr, "warning")
}

func TestRegistryListTagFilter(t *testing.T) {
	env := newCLIEnv(t)
	_, _, err := env.run("registry", "add", "--title", "Alpha", "--tag", "foo")
	require.NoError(t, err)
	_, _, err = env.run("registry", "add", "--title", "Beta", "--tag", "bar")
	require.NoError(t, err)

	out, _, err := env.run("registry", "list", "--tag", "foo")
	require.NoError(t, err)
	assert.Contains(t, out, "Alpha")
	assert.NotContains(t, out, "Beta")
}

func TestRegistryGet(t *testing.T) {
	env := newCLIEnv(t)
	out, _, err := env.run("registry", "add", "--title", "X", "--body", "body")
	require.NoError(t, err)
	id := out[:len(out)-1] // strip newline

	out, _, err = env.run("registry", "get", id)
	require.NoError(t, err)
	assert.Contains(t, out, "X")
	assert.Contains(t, out, "body")
}

func TestRegistryEdit(t *testing.T) {
	env := newCLIEnv(t)
	out, _, err := env.run("registry", "add", "--title", "X")
	require.NoError(t, err)
	id := out[:len(out)-1]

	_, _, err = env.run("registry", "edit", id, "--title", "Y")
	require.NoError(t, err)

	out, _, err = env.run("registry", "get", id)
	require.NoError(t, err)
	assert.Contains(t, out, "Y")
}

func TestEvaluate(t *testing.T) {
	env := newCLIEnv(t)
	out, _, err := env.run("registry", "add", "--title", "X")
	require.NoError(t, err)
	id := out[:len(out)-1]

	out, _, err = env.run("evaluate", id)
	require.NoError(t, err)
	assert.Contains(t, out, "Total")
	// fixedProvider scores every persona 3/5, so spread is 0 (consensus).
	assert.Contains(t, out, "spread: 0/5 (consensus)")
}

func TestPersonaEdit(t *testing.T) {
	env := newCLIEnv(t)
	out, _, err := env.run("persona", "add", "--name", "Critic", "--prompt", "be critical")
	require.NoError(t, err)
	id := strings.TrimSpace(out)
	assert.NotEmpty(t, id)

	_, _, err = env.run("persona", "edit", id, "--name", "Critic v2")
	require.NoError(t, err)

	out, _, err = env.run("persona", "list")
	require.NoError(t, err)
	assert.Contains(t, out, "Critic v2")

	_, _, err = env.run("persona", "edit", id, "--name", "")
	assert.Error(t, err, "empty name is rejected")

	_, _, err = env.run("persona", "edit", "skeptic", "--name", "Nope")
	assert.Error(t, err, "built-ins are not editable")

	_, _, err = env.run("persona", "edit", "nope", "--name", "Nope")
	assert.Error(t, err, "unknown id")
}

func TestEvaluateSummary(t *testing.T) {
	env := newCLIEnv(t)
	out, _, err := env.run("registry", "add", "--title", "X")
	require.NoError(t, err)
	id := out[:len(out)-1]

	out, _, err = env.run("evaluate", id, "--summary")
	require.NoError(t, err)
	assert.Contains(t, out, "Total")
	assert.Contains(t, out, "Verdict: promising")
	assert.Contains(t, out, "Good upside, some risk.")
}

func TestHistoryShowsVerdict(t *testing.T) {
	env := newCLIEnv(t)
	out, _, err := env.run("registry", "add", "--title", "X")
	require.NoError(t, err)
	id := out[:len(out)-1]

	_, _, err = env.run("evaluate", id, "--summary")
	require.NoError(t, err)

	out, _, err = env.run("history", id)
	require.NoError(t, err)
	assert.Contains(t, out, "promising")
	assert.Contains(t, out, "spread:")
}

func TestHistory(t *testing.T) {
	env := newCLIEnv(t)
	out, _, err := env.run("registry", "add", "--title", "X")
	require.NoError(t, err)
	id := out[:len(out)-1]

	_, _, err = env.run("evaluate", id)
	require.NoError(t, err)

	out, _, err = env.run("history", id)
	require.NoError(t, err)
	assert.Contains(t, out, "run")
}

func TestFeedbackAddMissingAuthor(t *testing.T) {
	env := newCLIEnv(t)
	out, _, err := env.run("registry", "add", "--title", "X")
	require.NoError(t, err)
	id := out[:len(out)-1]

	_, _, err = env.run("feedback", "add", id, "--score", "3")
	assert.Error(t, err)
}

func TestResourceAddInvalidURL(t *testing.T) {
	env := newCLIEnv(t)
	out, _, err := env.run("registry", "add", "--title", "X")
	require.NoError(t, err)
	id := out[:len(out)-1]

	_, _, err = env.run("resource", "add", id, "--url", "not-a-url")
	assert.Error(t, err)
}
