---
name: spec-driven-dev
description: Use when implementing a feature or making any change in the aibreak repo. Applies the mandatory spec-driven workflow — update PROJECT_SPECIFICATION.md / TECHNICAL_SPECIFICATION.md first, then tests, then code — and verifies with make test and make lint.
---

# Spec-Driven Development (aibreak)

The aibreak repo is spec-driven. The specs are the single source of truth:

- `PROJECT_SPECIFICATION.md` — behavior, domain model, acceptance criteria (what/why).
- `TECHNICAL_SPECIFICATION.md` — architecture, libraries, layout (how).

## Workflow (mandatory order)

1. **Update the spec first.**
   - Behavior change → edit `PROJECT_SPECIFICATION.md` (add/change Given/When/Then
     acceptance criteria).
   - Structure/dependency/library change → edit `TECHNICAL_SPECIFICATION.md`.

2. **Write or update tests** that encode the new/changed acceptance criteria.
   - Each acceptance criterion maps 1:1 to a test.
   - Tests are table-driven, stdlib `testing` + `testify`.

3. **Implement until green.**

4. **Verify** with `make test` and `make lint`.

## Rules

- Never implement behavior that is not in the spec.
- If the spec is ambiguous, resolve it by editing the spec, not by guessing in code.
- Keep Go conventions: module `aibreak`, depend inward only, `internal/engine`
  pure (no I/O), all external I/O behind interfaces (`llm.Provider`, `registry.Store`),
  `log/slog`, `context.Context` everywhere.

## Acceptance criteria style

Specs use Given/When/Then. Example mapping to a test:

```
Given scores {skeptic: 2, optimist: 4, engineer: 3} with default weights,
When the engine aggregates, Then total = 60.0
```

→ a table-driven test case asserting `total == 60.0`.

When a behavior spans multiple surfaces (domain + store + CLI + API), update all
four spec sections consistently and write tests for each.
