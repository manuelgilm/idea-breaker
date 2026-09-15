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
   openai + gemini impls (net/http)
```

- **`internal/engine`** — pure, no I/O. Imports `internal/domain` and
  `internal/llm` (interface) plus stdlib. Two-stage flow: (1) persona
  evaluation + scoring (serial by default, see §8); (2) an optional sequential
  synthesize pass that consumes the `FeasibilityScore` and writes
  `Summary`/`Verdict`.
- **`internal/service`** — application layer. Orchestrates `engine` +
  `registry.Store`; owns validation, run grouping, auto-persist, cascade logic.
- **`internal/llm`** — defines `Provider` (and `KeyedProvider`); `openai` and
  `gemini` subpackages implement it.
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
│   ├── aibreakd/main.go       # HTTP server entrypoint
│   └── aibreak-desktop/       # self-contained Wails project: `wails build`
│       │                      # and `wails dev` run from THIS directory, since
│       │                      # Wails always builds the package in cwd
│       ├── main.go            # desktop app entrypoint
│       ├── wails.json         # Wails project config (frontend paths relative here)
│       └── frontend/          # desktop UI: vanilla TS + minimal CSS
│           ├── index.html
│           ├── src/
│           └── tsconfig.json
└── internal/
    ├── domain/                # shared types (Idea, Persona, Evaluation, ...)
    ├── engine/                # pure evaluation + scoring (imports domain + llm)
    ├── service/               # orchestration (engine + registry)
    ├── llm/                   # Provider iface + openai/ + gemini/ impls
    ├── registry/              # Store iface + sqlite/ impl + migrations/
    ├── cli/                   # cobra commands
    ├── api/                   # net/http handlers + router
    ├── desktop/               # Wails bindings over service (thin adapter)
    └── config/                # config loading
```

## 4. Library choices

| Concern    | Choice                        | Rationale                                             |
|------------|-------------------------------|-------------------------------------------------------|
| CLI        | `github.com/spf13/cobra`      | De facto standard; subcommands + flags match spec §7. |
| HTTP       | stdlib `net/http` (1.22 `ServeMux`) | No framework needed for a small API; method+path patterns cover §8. |
| SQLite     | `modernc.org/sqlite`          | Pure Go (no CGO), easy cross-compile, `database/sql` compatible. |
| Migrations | embedded SQL, applied on open | Simple, versioned-in-repo; no external tool.           |
| ULID       | `github.com/oklog/ulid/v2`    | Time-ordered IDs for `Idea.ID`, `RunID`, and custom persona IDs. |
| Logging    | stdlib `log/slog`             | Structured, no dep; spec §10 requirement.              |
| Tests      | stdlib `testing` + `github.com/stretchr/testify` | Table-driven + assert/require.   |
| Config     | stdlib + TOML (`github.com/BurntSushi/toml`) | Small surface; TOML for the file form. |
| Keyring    | `github.com/zalando/go-keyring` | OS credential store for the desktop OpenAI API key (Secret Service/Keychain/Credential Manager). |
| LLM client | plain `net/http`              | Avoid heavyweight SDK; the provider contract is small. |
| Desktop    | Wails v2 + vanilla TS         | Native-feeling local app; Go backend binds `service` in-process, no HTTP hop. Vanilla TS + minimal CSS keeps the bundle tiny; `tsc --noEmit` is the frontend check. |

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

// KeyedProvider is a Provider whose API key can be swapped at runtime.
type KeyedProvider interface {
    Provider
    SetAPIKey(key string)
}
```

`internal/llm` also provides `Switchable` — a `KeyedProvider`-router that routes
`Complete` to a named active provider (overriding the request model) and exposes
`SetActive`/`Names`/`Model`. It is used by the desktop app to switch providers
at runtime.

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

    AddProviderKey(ctx context.Context, k APIKey) (APIKey, error)
    GetProviderKey(ctx context.Context, id string) (APIKey, error)
    ListProviderKeys(ctx context.Context, provider string) ([]APIKey, error)
    DeleteProviderKey(ctx context.Context, id string) error
    SetDefaultProviderKey(ctx context.Context, id string) error
    GetDefaultProviderKey(ctx context.Context, provider string) (APIKey, error)

    GetSetting(ctx context.Context, key string) (string, error)
    SetSetting(ctx context.Context, key, value string) error
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

**Desktop bindings** (`internal/desktop`): thin adapter over `service` exposing
the product-spec §11 views — `ListIdeas` (with per-idea latest score),
`GetIdea`, `CreateIdea`, `UpdateIdea`, `DeleteIdea`, `Evaluate` (incl. persona
selection + summary toggle), `ListRuns`, `ListFeedback`, `AddFeedback`,
`DeleteFeedback`, `ListResources`, `AddResource`, `DeleteResource`,
`ListPersonas` (with a built-in flag), `CreatePersona`, `UpdatePersona`,
`DeletePersona`, and the provider settings (`GetProviders`, `SetActiveProvider`,
`SaveModel`, `ListAPIKeys`, `AddAPIKey`, `DeleteAPIKey`, `SetDefaultAPIKey`,
`ApplySettings`, `ApplyDefaultKey`). Same error semantics as the service; no new
domain behavior.

Wails does **not** inject `context.Context` into bound methods — it treats it as
a regular parameter, so the adapter holds a background `context.Context`
(initialized at construction) and passes it to the service internally; bound
methods never expose `context.Context`. The adapter holds an `llm.Switchable`
(behind a narrow `ProviderRouter` interface) so the provider view can switch the
active provider, swap its API key, and change its model in place, and a
`SecretStore` interface (default `go-keyring`; faked in tests) for key
persistence. Each key's secret lives in the keyring under account
`<provider>:<keyID>`; the adapter generates the key ID (ULID) and orchestrates
secret + metadata. The active provider and per-provider models are persisted in
the `settings` table and applied at startup via `ApplySettings`. `app.Build`
(CLI/API) returns a single `llm.KeyedProvider`; `app.BuildDesktop` returns the
`llm.Switchable` so the desktop can switch providers at runtime.

**Streaming evaluation** (progressive results). The adapter streams each
persona's result to the frontend as it completes rather than returning only the
final aggregate. `engine.Evaluator.EvaluateWithProgress` takes an
`onResult func(domain.Evaluation)` callback invoked (from each persona's
goroutine) the moment that persona's `Evaluation` is produced, before
aggregation; `service.EvaluateWithProgress` threads it through and
`service.Evaluate` delegates with `nil`, so CLI/API behavior is unchanged. The
desktop `Evaluate` binding resolves persona IDs to names (via `ListPersonas`)
and emits a Wails `evaluation:persona` event carrying a `PersonaResult`
`{name, status, score, rationale, error}` DTO for each completion; the aggregate
(total, spread, verdict, summary) is delivered only by the bound method's return
value, which the frontend renders last. Wails events require the application
`context.Context` (they carry the `events` service in the context value), so the
adapter captures it in a `Startup(ctx)` method wired to the Wails `OnStartup`
hook and wraps `runtime.EventsEmit` behind an `EventEmitter` interface (default
`wailsEventEmitter`; faked in tests) so the bound method is testable without a
Wails runtime.

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

- Env vars: `AIBREAK_*` (and `OPENAI_API_KEY`, `GEMINI_API_KEY`) per spec §9.
- Config file: TOML at `$AIBREAK_CONFIG`, else `~/.config/aibreak/config.toml`.
- `internal/config.Load() (Config, error)` produces a `Config` struct consumed
  by `cmd/*`. `LLMProvider` selects the provider (`openai` or `gemini`);
  `app.Build` maps it to the concrete `llm.KeyedProvider`, using `Model`
  (`AIBREAK_LLM_MODEL`) for OpenAI and `GeminiModel` (`AIBREAK_GEMINI_MODEL`)
  for Gemini.
- The desktop app additionally persists the **active provider** and
  **per-provider model** in the `settings` table (via
  `internal/desktop.ApplySettings`), which override the config/env defaults on
  launch.

Resolved product-spec decisions recorded here:
- `Idea.ID` (and all generated IDs) are **ULIDs**; custom personas get generated
  ULID IDs (built-ins keep their fixed slugs).
- Persona `Version` is an **auto-incrementing integer** (`1, 2, 3, …`), bumped
  by the service on every `UpdatePersona`; callers never supply it.
- The desktop app stores API key **secrets** in the OS keyring (service
  `aibreak`, account `<provider>:<keyID>`); key metadata lives in the
  `provider_keys` table. `internal/desktop` wraps `go-keyring` behind a
  `SecretStore` interface so tests can fake it. On Linux a running Secret
  Service (gnome-keyring/KWallet) is required; if absent, `AddAPIKey` returns an
  error and the app falls back to the `OPENAI_API_KEY` env var / config file.

## 8. Concurrency & context

- All `Store`/`Provider`/service methods take `context.Context`.
- Persona evaluations within a run execute **serially** by default
  (concurrency 1, tunable via `engine.WithConcurrency`) so results stream to the
  desktop one-by-one in request order; the semaphore caps in-flight evaluations
  at the configured limit.
- `AIBREAK_LLM_TIMEOUT` is a **per-call** deadline: each `Provider.Complete`
  gets its own timeout via `context.WithTimeout`. There is no separate
  whole-run timeout in v1.
- Results are assembled deterministically: the `FeasibilityScore.Breakdown` is
  ordered by the requested persona order regardless of completion order, so
  CLI/API output and tests are stable. Streaming `onResult` calls, by contrast,
  arrive in completion order (they are emitted from each persona's goroutine);
  consumers must not assume request order.
- When synthesis is requested, it runs as a **single sequential call after the
  persona stage completes** (it consumes the aggregate, so it cannot run in
  parallel with it). The same per-call timeout applies.
- The OpenAI provider's API key is **mutable at runtime** (`SetAPIKey`, used by
  the desktop settings screen); access to it is synchronized (`sync.RWMutex`)
  so a key update cannot race an in-flight `Complete`.

## 9. Testing strategy

| Layer     | Kind        | Tooling                                   |
|-----------|-------------|-------------------------------------------|
| engine    | unit        | mocked `Provider`; table-driven; covers §3 scoring edge cases + synthesizer strict-parse, verdict enum, `inconclusive`-on-partial coverage, and spread values (incl. all-agree, single-success, failed-excluded) |
| llm       | contract    | `httptest` fake server; 200/429/malformed |
| registry  | integration | `:memory:` and temp-file SQLite; migrations idempotent (incl. v1→…→v6 upgrade path); cascade checks; `UpdatePersona` incl. built-in guard; `SaveRun`/`ListRuns` round-trip with `Spread`; settings get/set |
| service   | unit        | fake `Store` + fake `Provider`            |
| api       | integration | `httptest` against real handlers; error-code mapping |
| cli       | golden/exit | run command func with buffers; assert output + exit code |
| desktop   | unit        | bound methods against in-memory store + fake provider + fake `SecretStore`, mirroring CLI/API test style; `tsc --noEmit` must pass on `frontend/` |

Each acceptance criterion in the product spec maps 1:1 to a test.

## 10. Build & tooling

`Makefile` targets:

- `make build` — compile `cmd/aibreak` and `cmd/aibreakd`.
- `make test` — `go test ./...`.
- `make lint` — `golangci-lint run`.
- `make run-cli` / `make run-api` — dev run helpers.
- `make fmt` — `gofmt` + `go mod tidy`.
- `make desktop-dev` — `wails dev` run inside `cmd/aibreak-desktop` (hot-reload frontend + Go backend).
- `make desktop-build` — `wails build` run inside `cmd/aibreak-desktop` (native binary for the host OS).

CI (optional, later): `go test ./...` + `golangci-lint` on PR.

**Desktop packaging.** Wails cannot cross-compile (it links the platform
webview + CGO), so releases build natively per OS: the release workflow gains
an OS matrix (ubuntu/macos/windows runners), each running `wails build`, and
GoReleaser ships `aibreak-desktop` alongside the CLI/API binaries. Prereqs:
Wails CLI + Node.js everywhere; webkit2gtk system deps on Linux CI runners
(macOS WKWebView and Windows WebView2 are built-in).

**Linux webkit version.** Wails defaults to `webkit2gtk-4.0`, which modern
distros (Ubuntu 24.04+) no longer ship — only `webkit2gtk-4.1` is available.
All Linux desktop builds in this repo therefore pass `-tags "webkit2_41"`
(see `Makefile` targets); CI runners must install `libwebkit2gtk-4.1-dev`.
Plain `go build`/`go vet`/`go test` are unaffected and need no flags.
