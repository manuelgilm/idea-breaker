# AGENTS.md

Repository instructions for AI agents working on **aibreak**.

## Project

aibreak is an AI-powered idea evaluation engine exposed as a CLI (`aibreak`)
and HTTP API (`aibreakd`), written in Go.

## Spec-driven development (mandatory)

This repo is **spec-driven**. The specs are the single source of truth:

- `PROJECT_SPECIFICATION.md` — behavior, domain model, acceptance criteria (what/why).
- `TECHNICAL_SPECIFICATION.md` — architecture, libraries, layout (how).

For any change, follow this order:

1. Update the relevant spec first (behavior → product spec; structure/deps → technical spec).
2. Write or update the tests that encode the spec's acceptance criteria.
3. Implement until green.
4. Run `make test` and `make lint`.

Never implement behavior that isn't in the spec. If the spec is ambiguous,
resolve it by editing the spec, not by guessing in code.

## Conventions

- Go 1.22+, module path `aibreak`.
- Layout: `cmd/aibreak`, `cmd/aibreakd`, `internal/{domain,engine,service,llm,registry,cli,api,config}`.
- Dependency rule: depend inward only. `internal/engine` is pure (no I/O).
- All external I/O behind interfaces (`llm.Provider`, `registry.Store`).
- Structured logging via `log/slog`. Honors `context.Context` everywhere.
- Tests: table-driven, stdlib `testing` + `testify`. Each spec acceptance
  criterion maps 1:1 to a test.

## Verification

Always run after changes:

```sh
make test
make lint
```

## Skills

Reusable workflows live in `.opencode/skill/`:

- `spec-driven-dev` — the change workflow above.
- `spec-review` — act as a PM/critical reviewer of the specs.
