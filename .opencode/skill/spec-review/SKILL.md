---
name: spec-review
description: Use when asked to review the specs, or to "act as a product manager", "critical reviewer", or check whether PROJECT_SPECIFICATION.md / TECHNICAL_SPECIFICATION.md are sound and well-defined. Produces a prioritized review of blocking, important, and minor issues.
---

# Spec Review (PM / critical reviewer)

Act as a product manager and critical reviewer of the aibreak specs:

- `PROJECT_SPECIFICATION.md` — behavior, domain model, acceptance criteria.
- `TECHNICAL_SPECIFICATION.md` — architecture, libraries, layout.

## How to review

1. Read the full document(s) in one pass.
2. Judge **soundness** (does it describe a coherent, buildable product?) and
   **well-definedness** (are semantics, edge cases, and consistency nailed down?).
3. Check consistency across all surfaces: domain model ↔ behaviors ↔ store ↔
   CLI ↔ API, and cross-references between sections/files.

## Focus areas (checklist)

- **Scoring semantics**: partial failure handling, division-by-zero/zero-weight
  guards, rounding/clamping, coverage/confidence.
- **Data model**: dead fields that imply missing operations, nullability on
  failure, ID/versioning choices.
- **LLM contract**: strict output schema, integer/range enforcement, rubric
  anchoring, temperature/timeout/retry defaults.
- **API surface**: path consistency, error-code taxonomy, request/response
  shapes, cascade/delete behavior.
- **CLI**: flag naming consistency with domain fields, missing get/edit/delete
  commands, exit codes.
- **Edge cases**: empty inputs, unknown ids, zero-length collections.

## Output format

Produce a prioritized review:

- **Blocking** — must resolve before code (would force redesign).
- **Important** — should resolve (test gaps, ambiguities).
- **Minor** — polish, non-blocking.

For each issue: cite the section, explain why it matters, and propose a fix.
Lead with the single most important risk. End with a clear verdict (ready vs.
what to fix first). Do not edit the specs during review unless asked.
