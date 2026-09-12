# aibreak — Technical Specification

> The "how" of the project. The product spec (`PROJECT_SPECIFICATION.md`) is the
> "what/why" and the single source of truth for behavior. This document pins down
> architecture, library choices, and implementation conventions. When they
> conflict, the product spec wins; update this doc to match.

## 1. Overview & traceability

| Concern                  | Defined in                                   |
|--------------------------|----------------------------------------------|
| Domain model, behaviors  | `PROJECT_SPECIFICATION.md` §2–§4             |
| LLM provider contract    | `PROJECT_SPECIFICATION.md` §5                |
| Persistence schema       | `PROJECT_SPECIFICATION.md` §6                |
| CLI / API surface        | `PROJECT_SPECIFICATION.md` §7–§8             |
| Configuration keys       | `PROJECT_SPECIFICATION.md` §9                |
| Architecture & libraries | this document                                |

## 2. Architecture

Layered, dependency rule = **depend inward only**:

```
cmd/aibreak  (CLI adapter) ──┐
cmd/aibreakd (API adapter) ──┤
                              ▼
                       internal/service   (orchestration)
                              │
            ┌─────────────────┴───────────────────┐
            ▼                                     ▼
   internal/engine  (pure)               internal/registry (Store)
            │                                     │
            ▼                                     ▼
   internal/llm  (Provider iface)          sqlite impl + migrations
            │
            ▼
   openai impl (net/http)
```

- **`internal/engine`** — pure, no I/O. Imports `internal/domain` and
  `internal/llm` (interface) plus stdlib. Two-stage flow: (1) persona
  evaluation + scoring (parallel); (2) an optional sequential synthesize pass
  that consumes the `FeasibilityScore` and writes `Summary`/`Verdict`.
- **`internal/service`** — application layer. Orchestrates `engine` +
  `registry.Store`; owns validation, run grouping, auto-persist, cascade logic.
- **`internal/llm`** — defines `Provider`; `openai` subpackage implements it.
- **`internal/registry`** — defines `Store`; `sqlite` subpackage implements it
  plus migrations and built-in persona seeding.
- **`internal/cli` / `internal/api`** — thin adapters over `service`.
- **`internal/config`** — loads configuration (see §7).

Wire-up happens in `cmd/*/main.go` (composition root). No package reaches
"upward" across a layer boundary.

## 3. Module & layout

- Module path: `aibreak` (rename to a hosted path when published).
- Go version: **1.22+** (stdlib `http.ServeMux` method/path patterns, `log/slog`).

```
aibreak/
├── PROJECT_SPECIFICATION.md
├── TECHNICAL_SPECIFICATION.md
├── AGENTS.md
├── go.mod
├── Makefile
├── cmd/
│   ├── aibreak/main.go        # CLI entrypoint
│   └── aibreakd/main.go       # HTTP server entrypoint
└── internal/
    ├── domain/                # shared types (Idea, Persona, Evaluation, ...)
    ├── engine/                # pure evaluation + scoring (imports domain + llm)
    ├── service/               # orchestration (engine + registry)
    ├── llm/                   # Provider iface + openai/ impl
    ├── registry/              # Store iface + sqlite/ impl + migrations/
    ├── cli/                   # cobra commands
    ├── api/                   # net/http handlers + router
    └── config/                # config loading
```

## 4. Library choices

| Concern    | Choice                        | Rationale                                             |
|------------|-------------------------------|-------------------------------------------------------|
| CLI        | `github.com/spf13/cobra`      | De facto standard; subcommands + flags match spec §7. |
| HTTP       | stdlib `net/http` (1.22 `ServeMux`) | No framework needed for a small API; method+path patterns cover §8. |
| SQLite     | `modernc.org/sqlite`          | Pure Go (no CGO), easy cross-compile, `database/sql` compatible. |
| Migrations | embedded SQL, applied on open | Simple, versioned-in-repo; no external tool.           |
| ULID       | `github.com/oklog/ulid/v2`    | Time-ordered IDs for `Idea.ID`, `RunID`, etc.          |
| Logging    | stdlib `log/slog`             | Structured, no dep; spec §10 requirement.              |
| Tests      | stdlib `testing` + `github.com/stretchr/testify` | Table-driven + assert/require.   |
| Config     | stdlib + TOML (`github.com/BurntSushi/toml`) | Small surface; TOML for the file form. |
| LLM client | plain `net/http`              | Avoid heavyweight SDK; the provider contract is small. |

No ORM, no web framework, no DI framework.

## 5. Interfaces

These mirror the product spec and live in their defining packages:

```go
// internal/llm/llm.go
type Message struct { Role, Content string }

type Request struct {
    Model       string
    Messages    []Message
    Temperature float64
    MaxTokens   int
    JSONMode    bool
}

type Response struct { Content string }

type Provider interface {
    Complete(ctx context.Context, req Request) (Response, error)
}
```

```go
// internal/registry/store.go  (domain types from internal/domain)
type Store interface {
    CreateIdea(ctx context.Context, idea Idea) (Idea, error)
    GetIdea(ctx context.Context, id string) (Idea, error)
    ListIdeas(ctx context.Context, tag string) ([]Idea, error)
    FindIdeasByTitle(ctx context.Context, title string) ([]Idea, error)
    UpdateIdea(ctx context.Context, idea Idea) (Idea, error)
    DeleteIdea(ctx context.Context, id string) error

    CreatePersona(ctx context.Context, p Persona) (Persona, error)
    GetPersona(ctx context.Context, id string) (Persona, error)
    ListPersonas(ctx context.Context) ([]Persona, error)
    UpdatePersona(ctx context.Context, p Persona) (Persona, error)
    DeletePersona(ctx context.Context, id string) error

    SaveRun(ctx context.Context, score FeasibilityScore, evals []Evaluation) error
    ListRuns(ctx context.Context, ideaID string) ([]FeasibilityScore, error)

    AddFeedback(ctx context.Context, f Feedback) (Feedback, error)
    ListFeedback(ctx context.Context, ideaID string) ([]Feedback, error)
    DeleteFeedback(ctx context.Context, id string) error

    AddResource(ctx context.Context, r Resource) (Resource, error)
    ListResources(ctx context.Context, ideaID string) ([]Resource, error)
    DeleteResource(ctx context.Context, id string) error
}
```

Domain types (`Idea`, `Persona`, `Evaluation`, `FeasibilityScore`, `Feedback`,
`Resource`) plus patch types (`IdeaPatch`, `PersonaPatch`) live in
**`internal/domain`**, imported by `engine`, `service`, and `registry`. This
keeps the acyclic graph: `engine` imports `domain` + `llm`, `registry` imports
`domain`, `service` imports all three. `FeasibilityScore` carries
`Summary`/`Verdict` (synthesizer output) and `Spread` (disagreement, computed
in the engine aggregate alongside `Total`); the synthesizer prompt constant and
the `Agreement(spread)` label helper live in `internal/engine`.

## 6. Error model

Typed errors in `service`, mapped to the API envelope codes (spec §8):

| Condition             | Go error                     | API code / HTTP      |
|-----------------------|------------------------------|----------------------|
| Invalid input         | `ErrValidation`              | `validation_error` 400 |
| Not found             | `ErrNotFound`                | `not_found` 404      |
| Duplicate id/slug     | `ErrConflict`                | `conflict` 409       |
| Upstream rate limit   | `ErrRateLimited` (from llm)  | `rate_limited` 429   |
| Upstream LLM failure  | `ErrProvider` (from llm)     | `llm_error` 502      |
| All personas failed   | `ErrNoResults`               | `llm_error` 502      |
| Anything else         | (unwrap/500)                 | `internal` 500       |

Use `errors.Is`/`errors.As`; wrap with context. The CLI maps errors to a
non-zero exit code and a stderr message.

Warnings are not errors: `service.CreateIdea` returns
`(domain.Idea, string, error)` where the string is a human-readable warning
(empty when none), e.g. a duplicate-title notice. The CLI prints it to stderr;
the API surfaces it as an `X-Warning` response header.

## 7. Configuration

Precedence (spec §9): **flags > env vars > config file > defaults**.

- Env vars: `AIBREAK_*` (and `OPENAI_API_KEY`) per spec §9.
- Config file: TOML at `$AIBREAK_CONFIG`, else `~/.config/aibreak/config.toml`.
- `internal/config.Load() (Config, error)` produces a `Config` struct consumed
  by `cmd/*`.

Resolved product-spec decisions recorded here:
- `Idea.ID` (and all generated IDs) are **ULIDs**.
- Persona `--version` default is **`1.0.0`**.

## 8. Concurrency & context

- All `Store`/`Provider`/service methods take `context.Context`.
- Persona evaluations within a run execute **concurrently** with bounded
  parallelism (semaphore, max 4 in flight).
- `AIBREAK_LLM_TIMEOUT` is a **per-call** deadline: each `Provider.Complete`
  gets its own timeout via `context.WithTimeout`. There is no separate
  whole-run timeout in v1.
- Results are assembled deterministically: the `FeasibilityScore.Breakdown` is
  ordered by the requested persona order regardless of completion order, so
  CLI/API output and tests are stable.
- When synthesis is requested, it runs as a **single sequential call after the
  persona stage completes** (it consumes the aggregate, so it cannot run in
  parallel with it). The same per-call timeout applies.

## 9. Testing strategy

| Layer     | Kind        | Tooling                                   |
|-----------|-------------|-------------------------------------------|
| engine    | unit        | mocked `Provider`; table-driven; covers §3 scoring edge cases + synthesizer strict-parse, verdict enum, `inconclusive`-on-partial coverage, and spread values (incl. all-agree, single-success, failed-excluded) |
| llm       | contract    | `httptest` fake server; 200/429/malformed |
| registry  | integration | `:memory:` and temp-file SQLite; migrations idempotent (incl. v1→v2→v3 upgrade path); cascade checks; `UpdatePersona` incl. built-in guard; `SaveRun`/`ListRuns` round-trip with `Spread` |
| service   | unit        | fake `Store` + fake `Provider`            |
| api       | integration | `httptest` against real handlers; error-code mapping |
| cli       | golden/exit | run command func with buffers; assert output + exit code |

Each acceptance criterion in the product spec maps 1:1 to a test.

## 10. Build & tooling

`Makefile` targets:

- `make build` — compile `cmd/aibreak` and `cmd/aibreakd`.
- `make test` — `go test ./...`.
- `make lint` — `golangci-lint run`.
- `make run-cli` / `make run-api` — dev run helpers.
- `make fmt` — `gofmt` + `go mod tidy`.

CI (optional, later): `go test ./...` + `golangci-lint` on PR.
