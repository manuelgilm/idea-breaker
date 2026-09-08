# Idea Breaker — Specification

## 1. Overview

Idea Breaker (`aibreak`) is a CLI tool that breaks down an idea using AI personas (pessimist, optimist, architect, etc.) and synthesizes a scored review.

The pipeline runs in two stages:

1. **Fan-out** — the idea is sent to several personas in parallel, each with its own system prompt.
2. **Synthesis** — the persona outputs are combined and fed to a synthesizer that produces a single verdict with a feasibility score.

The final result (idea, per-persona breakdowns, synthesis, status) is written as indented JSON to a `--output` path.

---

## 2. Current specification

### 2.1 Tech stack and module

| Item    | Value                                        |
| ------- | -------------------------------------------- |
| Language | Go `1.26.3`                                  |
| Module  | `github.com/manuelgilm/idea-breaker`         |
| CLI     | `github.com/spf13/cobra` `v1.10.2`           |
| YAML    | `go.yaml.in/yaml/v3` `v3.0.4`                |
| Binary  | `aibreak`                                    |

Entrypoint: `cmd/aibreak/main.go` → `cmd.Execute()`; the `aibreak` command lives in `cmd/aibreak.go`.

### 2.2 Architecture

Three packages:

- **`cmd`** — the cobra CLI. Parses flags, reads env vars, wires a `Caller` and a persona source, runs the engine, and writes JSON.
- **`engine`** — the reusable core. Free of CLI and HTTP concerns; importable by external repos.
- **`persona`** — loads personas (and the synthesizer prompt) from embedded fixtures or from disk.

```
cmd (CLI) ──▶ engine ──▶ Caller (HTTP / mock)
                  ▲
persona (Source) ──┘
```

Key interfaces and types:

- `engine.Caller` — single transport method `Chat(ctx, system, user) (string, error)`. Shared by both the persona fan-out and the synthesizer. Carries **no** credentials; auth is bound to the concrete caller at construction.
- `engine.Persona` — `{Name, SystemPrompt}`.
- `engine.Result` — `{Persona, Breakdown, Err}`.
- `engine.Breakdown` — `{Summary, KeyPoints, Risks, Dependencies, Score}`.
- `engine.Synthesis` — `{Feedback, Score}`.
- `persona.Source` — `Load() ([]engine.Persona, error)`; the seam for a future prompt-registry backend.

### 2.3 Engine behavior

- **`BreakAll(ctx, caller, idea, personas)`** fans out one API call per persona in parallel and returns results **in input order**. Errored calls are captured in `Result.Err` (collect-all policy), not dropped.
- **`parseBreakdown`** extracts the JSON object from the persona response via `extractJSON` (slices between the first `{` and last `}`, tolerating prose and code fences) and clamps `score` to `0–100`. A parse failure becomes that persona's `Err`.
- **`Synthesize(ctx, caller, synthPrompt, idea, results)`** builds a combined prompt from all persona results and returns `{feedback, score}` (score clamped `0–100`). A synthesize error is a **hard failure** — without it there is no deliverable.

### 2.4 HTTP caller

- `engine.HTTPCaller` implements `Caller` against an OpenAI-compatible endpoint `POST {BaseURL}/v1/chat/completions`.
- `NewHTTPCaller(apiKey, baseURL, model)` applies defaults for empty values: `https://api.openai.com` and `gpt-4o-mini`.
- Sends `Authorization: Bearer <apiKey>` and `Content-Type: application/json`.
- Non-200 status, empty `choices`, or a decode error all return an error.

### 2.5 CLI contract

Flags:

| Flag       | Required | Description                                                     |
| ---------- | -------- | --------------------------------------------------------------- |
| `--idea`   | yes      | The idea to break down                                          |
| `--output` | yes      | Path to the output JSON file                                    |
| `--mock`   | no       | Run offline with deterministic mock responses (no API key needed) |

Environment variables:

| Variable               | Required | Default | Purpose                                        |
| ---------------------- | -------- | ------- | ---------------------------------------------- |
| `OPENAI_API_KEY`       | yes*     | —       | Provider API key. Missing → error (no flag).   |
| `AIBREAK_PERSONAS_DIR` | no       | —       | Override embedded personas with a YAML directory |

\* Not required when `--mock` is set.

Error conditions:

- Missing `--idea` → error.
- Missing `--output` → error.
- Missing `OPENAI_API_KEY` (non-mock) → error.
- A 401 during `Synthesize` → hard failure (exit 1). Per-persona 401s are captured in each persona's `error` field.

There is no `--api-key` flag; credentials come only from the environment.

### 2.6 Personas

Personas are YAML files, one per persona, with shape:

```yaml
name: <persona name>
prompt: |
  ...
```

- Built-in personas are **embedded** into the binary (`persona/fixtures/*.yaml`): `pessimist`, `optimist`, `architect`.
- `synthesizer.yaml` holds the synthesizer prompt and is **excluded** from `Load`, so it is never treated as a persona.
- `persona.EmbeddedSource` reads embedded fixtures; `persona.FileSource{Dir}` reads `*.yaml`/`*.yml` files from a directory. Both return personas in filename order.
- Setting `AIBREAK_PERSONAS_DIR` switches the CLI to `FileSource` + `LoadSynthesizer` (no rebuild needed).
- Each persona prompt instructs the model to reply with a single JSON object `{summary, key_points, risks, dependencies, score}`.

### 2.7 Output schema

Indented JSON written to `--output`:

```json
{
  "idea": "a startup that delivers groceries by drone",
  "personas": [
    {
      "name": "pessimist",
      "summary": "High regulatory and delivery-cost risk.",
      "key_points": ["local regulations", "last-mile cost"],
      "risks": ["noise complaints", "weather downtime"],
      "dependencies": ["drone licensing", "insurance"],
      "score": 30
    }
  ],
  "synthesis": {
    "feedback": "Promising niche, but regulators and economics remain the gating risks.",
    "score": 52
  },
  "status": "complete"
}
```

- If a persona call fails, its entries are included with an `error` field instead of a breakdown.
- The API key is never included in the output.

### 2.8 Testing

- `engine` unit tests: `break_all_test.go`, `breakdown_test.go`, `synthesis_test.go`, `mock_test.go`, `http_caller_test.go` (uses `httptest`). Test doubles (`stubCaller`) live unexported in `engine/*_test.go`.
- `persona` tests: `embed_test.go` (loads, excludes synthesizer, synthesizer prompt non-empty).
- CI runs `go test ./... -race` + `go vet ./...`.
- The smoke test runs the built binary with `--mock` (full pipeline offline, deterministic, exits 0 with valid JSON).

### 2.9 CI/CD and release

- `.github/workflows/ci.yml` — on `master` push and PRs: `go test ./... -race` + `go vet ./...`.
- `.github/workflows/release.yml` — on `v*` tag push: GoReleaser builds cross-platform binaries (linux/darwin/windows × amd64/arm64) and publishes them.
- `.goreleaser.yml` — `project_name: aibreak`, `CGO_ENABLED=0`, archives with checksums, changelog filters.

---

## 3. Not-implemented features

Features identified from code seams, comments, and docs that are planned or implied but not yet built.

### 3.1 Webservice consumer

The `engine` package is documented as "the shared core of a future webservice" — the webservice is intended to live in a **separate git repo** that imports `github.com/manuelgilm/idea-breaker/engine`. No HTTP server or API layer exists in this repo.

- Where the seam is: `engine.Caller` is consumer-agnostic and carries no credentials; consumers inject their own implementation.

### 3.2 Prompt-registry backend

`persona.Source` is explicitly the seam for a future prompt-registry backend. Only two implementations exist today:

- `EmbeddedSource` (fixtures compiled into the binary)
- `FileSource` (a directory of YAML on disk)

A network-backed registry source is not implemented.

### 3.3 Configurable model / base URL / timeout / retry

`engine.NewHTTPCaller(apiKey, baseURL, model)` accepts a base URL and model, but the CLI hardcodes both to `""` (defaults). There is no way for a user to override:

- the model (`gpt-4o-mini`)
- the base URL (custom OpenAI-compatible endpoint)
- HTTP timeout (uses `http.DefaultClient`, which has no timeout)
- retry / backoff on transient failures

### 3.4 CLI flags: version, stdin/file input, config file

Not implemented:

- `--version` (no version string injected via `ldflags`)
- reading the idea from stdin or a file (only the `--idea` flag)
- a config file (configuration is env-var only)

### 3.5 Concurrency / rate limiting on the fan-out

`BreakAll` launches one goroutine per persona with no limit. For a large persona set there is no cap on in-flight requests and no rate limiting.

### 3.6 Additional personas

Only three personas are embedded (`pessimist`, `optimist`, `architect`). The tool description implies more ("etc."), but no additional built-in personas exist.

### 3.7 Repository hygiene

- No `.gitignore`: `.env`, `bin/`, and generated result JSONs (e.g. `result.json`, `candle_idea.json`, `result-idea-breaker.json`) are currently untracked.

### 3.8 CLI / end-to-end tests and output validation

- No automated test covers `cmd` (the CLI wiring); the smoke test is manual (`./bin/aibreak --mock ...`).
- No validation of the output JSON against a schema; the output contract is enforced only by convention.

### 3.9 Output format options

The only output is a JSON file written to `--output` (plus a one-line "Output written to ..." message on stdout). Not implemented:

- human-readable stdout/markdown/text rendering
- alternate serialization formats
- printing the result to stdout instead of a file

### 3.10 Local idea registry

A persistent, local store of ideas and their breakdown results is not implemented. Today each run writes a one-off JSON file to `--output`, with no way to recall or manage past ideas. A local registry would add:

- a store (e.g. a SQLite database or a JSON/directory store under a user data dir) holding each idea and its personas/synthesis result
- CLI subcommands to create, list, get, and delete ideas (e.g. `aibreak idea list` / `aibreak idea rm`)
- recall of a previous idea's full breakdown without re-running the personas
