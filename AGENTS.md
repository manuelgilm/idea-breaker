# Idea Breaker

CLI tool that breaks down an idea using AI personas (pessimist, optimist, architect, etc.) and synthesizes a scored review. See `init_project.md` for the full concept.

## Status

Early-stage. A cobra-based CLI exists that loads personas from YAML, fans out one real LLM call per persona in parallel, synthesizes a scored review, and writes JSON.

## Tech stack

Go 1.26.3 + `github.com/spf13/cobra` (v1.10.2) + `go.yaml.in/yaml/v3`. Module path: `github.com/manuelgilm/idea-breaker`. Binary/command name: `aibreak`.

## Commands

```bash
# env vars (the CLI requires them to run)
export OPENAI_API_KEY=sk-test
# optional: override the embedded personas with your own YAML dir
export AIBREAK_PERSONAS_DIR=/path/to/personas

# build the binary
go build -o bin/aibreak ./cmd/aibreak

# run offline (no API key needed; personas + synthesizer are mocked)
./bin/aibreak --mock --idea "some idea" --output /tmp/result.json

# run it (hits the real OpenAI endpoint; requires OPENAI_API_KEY)
./bin/aibreak --idea "some idea" --output /tmp/result.json

# verify (test, vet, build, smoke test)
go test ./... -race && \
  go vet ./... && go build -o bin/aibreak ./cmd/aibreak && \
  ./bin/aibreak --mock --idea "test" --output /tmp/test.json
```

Note: the smoke test uses `--mock`, which runs the full pipeline offline against deterministic mock responses and always exits 0 with valid JSON. A real run requires `OPENAI_API_KEY`, and the placeholder `sk-test` returns a 401 — per-persona 401s are captured in each persona's `error` field (not a hard failure), but a 401 during `Synthesize` **is** a hard failure (exit 1) since the scored review is the deliverable.

## Notes

- Entrypoint: `cmd/aibreak/main.go` calls `cmd.Execute()`; the `aibreak` command lives in `cmd/aibreak.go`.
- `engine` package owns the parallel fan-out: `BreakAll` (index-writes, ordered results, collect-all errors) calls a `Caller` interface and parses each persona response into `engine.Breakdown{Summary, KeyPoints, Risks, Dependencies, Score}` (score clamped 0-100; a parse failure is captured as that persona's `Err`). `engine.Synthesize` runs after `BreakAll` and returns `engine.Synthesis{Feedback, Score}` (score clamped 0-100); a synthesize error is a hard failure.
- `engine.Caller` is a single transport method `Chat(ctx, system, user)` shared by both the persona fan-out and the synthesizer. `engine.HTTPCaller` (via `NewHTTPCaller(apiKey, baseURL, model)`) implements it against an OpenAI-compatible endpoint; empty `baseURL`/`model` fall back to `https://api.openai.com` / `gpt-4o-mini`. Test doubles live unexported in `engine/*_test.go` (`stubCaller`); `engine/http_caller_test.go` covers `HTTPCaller` with `httptest`. Run engine tests with `go test ./engine/... -race`.
- `persona` package loads personas from YAML. By default they are embedded into the binary (`persona/fixtures/*.yaml`, read via `persona.EmbeddedSource` and `persona.LoadEmbeddedSynthesizer`). Set `AIBREAK_PERSONAS_DIR` to load from disk instead (`persona.FileSource.Dir` / `persona.LoadSynthesizer`). `persona.Source` is the seam for a future prompt-registry backend. `synthesizer.yaml` is excluded from `Load` so it is never treated as a persona.
- Required flags: `--idea` and `--output`. Omitting either returns an error. `--mock` runs the whole pipeline offline against deterministic `engine.MockCaller` responses (no API key needed) and is what the smoke test uses.
- The API key comes only from the `OPENAI_API_KEY` env var; a missing key returns an error. There is no `--api-key` flag and no hardcoded placeholder default.
- Persona prompts and the synthesizer prompt instruct the model to reply with a single JSON object (no code fences/prose); the engine tolerates fences/preamble via `extractJSON`. Personas produce `{summary, key_points, risks, dependencies, score}`.
- Output is indented JSON written to the `--output` path: `{idea, personas: [{name, summary, key_points, risks, dependencies, score, error?}], synthesis{feedback, score}, status}`. It does not include the API key.
- CI: `.github/workflows/ci.yml` runs `go test ./... -race` + `go vet ./...` on `master` pushes and PRs.
- Releasing: `.goreleaser.yml` + `.github/workflows/release.yml` build cross-platform self-contained binaries on a `v*` tag (triggered by `git tag vX.Y.Z && git push origin vX.Y.Z`). Local dry-run: `goreleaser release --snapshot --skip=publish`. GoReleaser is not vendored; install with `go install github.com/goreleaser/goreleaser/v2@latest`.

## Architecture: reusable engine

The `engine` package is the shared core of a future webservice, not just this CLI. Keep it free of CLI and HTTP concerns: it must not import `cmd`, cobra, or any concrete HTTP client, and its `Caller` interface takes no credential/API-key (auth is bound to the concrete caller at construction via `HTTPCaller.APIKey`). Consumers (the CLI and a future API) inject their own implementation of `engine.Caller` (real HTTP for the webservice, `HTTPCaller` for the CLI).
- `engine` must be importable by external repos, so keep it at the repo root as a public package — do NOT move it under `internal/` (that blocks external imports).
- The webservice will live in a separate git repo and import `github.com/manuelgilm/idea-breaker/engine` via `require`/`replace`. Do not split `engine` into its own module unless it actually needs separate versioning.
