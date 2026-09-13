# aibreak — Project Specification

> This document is the single source of truth for the project. Every change
> starts here: update the spec, then update the tests, then the code.
> Acceptance criteria are written as Given/When/Then and map 1:1 to tests.

## 1. Purpose

aibreak is an AI-powered **idea evaluation engine** exposed through three
front-ends:

- a **CLI** (`aibreak`)
- an **HTTP API** (`aibreakd`)
- a **desktop app** (`aibreak-desktop`, Wails-based, see §11) — local,
  single-user, same non-goals as v1 (no auth, no real-time collaboration)

It is for **anyone evaluating an idea** — founders, product managers,
students, researchers, hobbyists, or teams weighing a feature — not only
startup founders.

The engine critiques/evaluates an idea from multiple **persona** perspectives
(each backed by a distinct system prompt), asks the LLM for a small-range
**score (0–5)** plus rationale per persona, and aggregates those into a single
**feasibility score (0–100)**. It also maintains a local **registry** of ideas
and related development features.

aibreak is **decision support, not an oracle**: persona scores, the aggregate
total, spread, and synthesized verdicts are fast, cheap, structured priors to
inform judgment. They do not replace human insight into market dynamics,
emotion, or domain nuance — the human reviewer (see Feedback, §2) is the
decision-maker, and every numeric result should be read alongside its
rationale, coverage, and disagreement signal.

### Non-goals (for v1)

- No multi-user authentication or authorization (local, single-user tool).
- No real-time collaboration.
- No training/self-modifying behavior.

### Future direction (not specified yet)

- **Multi-user authentication**: users, idea ownership, scoped access.
- **Real-time collaboration**: shared idea boards, presence, concurrent edits.
- **Persona version history**: immutable persona versions (v1 records only the
  current `Version` on each evaluation).
- **External feedback via shareable links/tokens**: letting an external person
  view an idea and submit feedback over the web (deferred to the multi-user
  milestone; v1 captures feedback locally).

These are deferred to a later milestone. The architecture is shaped so they can
be added without breaking the engine: the engine is stateless, the store is
behind an interface, and the API is versioned (`/v1/`). `Idea` will eventually
gain an `OwnerID` field; it is intentionally omitted from v1.

## 2. Domain model

### Idea
| Field     | Type     | Notes                                        |
|-----------|----------|----------------------------------------------|
| `ID`      | string   | Unique, generated (ULID)                     |
| `Title`   | string   | Required, non-empty                          |
| `Body`    | string   | Optional long-form description               |
| `Tags`    | []string | Optional                                     |
| `Created` | time     | Set on creation                              |
| `Updated` | time     | Set on update                                |

### Persona
| Field          | Type    | Notes                                            |
|----------------|---------|--------------------------------------------------|
| `ID`           | string  | Generated ULID for custom personas; built-ins keep stable slugs (`skeptic`, `optimist`, `engineer`) |
| `Name`         | string  | Display name                                     |
| `SystemPrompt` | string  | Instruction set defining the perspective (should include 0–5 rubric anchors; see §3) |
| `Weight`       | float   | Default 1.0 (a zero weight is normalized to 1.0); used in the aggregate score |
| `Version`      | int     | Auto-incremented (1, 2, 3, …) on every edit; captured by evaluations |
| `Created`      | time    | Set on creation                                  |

Built-in personas ship with the tool at `Version` `1` and are **not editable**
(edit and delete are rejected for built-in ids). Custom personas are **persisted
in the store** with a **generated ULID** and are editable in place via
`UpdatePersona` (see §4): every edit **auto-increments the `Version`** (1 → 2 →
3 → …), so each version is a distinct, reproducible snapshot. Every evaluation
captures the persona `Version` used, so results stay reproducible if a persona
later changes. `Weight` must be `>= 0`.

Partial persona updates use a `PersonaPatch` type with pointer/optional fields
(`*string` for name/system-prompt, `*float64` for weight), mirroring `IdeaPatch`:
a nil field means "unchanged", a non-nil field means "set to this value". The
`Version` is never caller-supplied — the service bumps it automatically.

**Built-in personas (v1):** seeded with `Weight` `1.0` and `Version` `1`.
Each `SystemPrompt` includes its 0–5 rubric anchors.

| ID | Name | SystemPrompt |
|----|------|--------------|
| `skeptic` | Skeptic | "You are a skeptical critic evaluating a product idea. Identify its flaws, risks, and failure modes; be specific and direct. Rate the idea's robustness on a 0–5 scale: 0 = fatally flawed, 1 = major flaws, 2 = significant flaws, 3 = mixed, 4 = mostly sound, 5 = nearly bulletproof." |
| `optimist` | Optimist | "You are an optimist evaluating a product idea. Identify its upside, strengths, and opportunities; be specific and direct. Rate the idea's potential on a 0–5 scale: 0 = no upside, 1 = marginal, 2 = modest, 3 = solid, 4 = strong, 5 = exceptional." |
| `engineer` | Engineer | "You are a pragmatic engineer evaluating a product idea. Assess its technical feasibility and implementation effort; be specific and direct. Rate the idea's feasibility on a 0–5 scale: 0 = infeasible, 1 = very hard, 2 = hard, 3 = moderate, 4 = easy, 5 = trivial with a clear path." |

### Evaluation
| Field           | Type     | Notes                                   |
|-----------------|----------|-----------------------------------------|
| `RunID`         | string   | Groups all evaluations of one run       |
| `IdeaID`        | string   | Idea being evaluated                    |
| `PersonaID`     | string   | Persona that produced this result       |
| `PersonaVersion`| int      | Persona version captured at eval time   |
| `Weight`        | float    | Persona weight captured at eval time    |
| `Status`        | string   | `success` or `failed`                   |
| `Score`         | int      | 0–5 (inclusive); present only on success |
| `Rationale`     | string   | Written pros/risks; empty on failure    |
| `Error`         | string   | Failure message; empty on success       |
| `Created`       | time     |                                         |

Evaluation results are **persisted** (see §6): every `Evaluate` call saves one
`Evaluation` per persona, all sharing the same `RunID`. A persona that fails
(after its one retry) is recorded as an `Evaluation` with `Status=failed` and a
non-empty `Error`; failed personas are excluded from the aggregate score.
`RunID` is a **ULID** (lexicographically sortable, time-ordered). Each
`Evaluation` snapshots the persona `Weight` (alongside `PersonaVersion`), and
each run's aggregate is persisted in the `runs` table, so historical results
stay reproducible even after a persona is changed or deleted.

### FeasibilityScore (aggregate)
| Field        | Type             | Notes                                  |
|--------------|------------------|----------------------------------------|
| `RunID`      | string           | Groups the evaluations of this run     |
| `IdeaID`     | string           |                                        |
| `Total`      | float            | 0–100, weighted average of *successful* persona scores |
| `Breakdown`  | []Evaluation     | Per-persona results (incl. failures)   |
| `Requested`  | int              | Number of personas attempted           |
| `Responded`  | int              | Number of personas that succeeded      |
| `Spread`     | float            | Range (max − min) of *successful* persona scores, 0–5 |
| `Summary`    | string           | Synthesized verdict text; empty unless synthesis requested and succeeded |
| `Verdict`    | string           | One of the labels below; empty unless synthesis succeeded |
| `Created`    | time             |                                        |

`Spread` measures persona disagreement on the raw 0–5 scale: all-agreeing runs
have spread 0, a single successful persona has spread 0, and failed personas
are excluded (same population as the aggregate). Interpretation labels
(single source of truth for all surfaces): `consensus` (0–1), `mixed` (2–3),
`divided` (4–5).

The optional second-stage **synthesizer** (see §4, §5) writes `Summary` and
`Verdict`. It is text-only: it never changes the deterministic `Total`.
`Verdict` labels:

- `proceed` — strong consensus of viability; minor risks easily mitigated.
- `promising` — high potential, requires significant iteration.
- `risky` — serious flaws, structural risks, or heavily divided opinions.
- `pass` — fundamental flaws, safety issues, or unanimous rejection.
- `inconclusive` — returned automatically when `responded < requested`
  (partial coverage); the summary must acknowledge the missing personas.

### Feedback (human review)
| Field       | Type    | Notes                                          |
|-------------|---------|------------------------------------------------|
| `ID`        | string  | Generated                                       |
| `IdeaID`    | string  | Idea being reviewed                             |
| `Author`    | string  | Name/email of the reviewer                      |
| `Score`     | int     | 0–5 (inclusive)                                 |
| `Rationale` | string  | Written feedback                                |
| `Aspect`    | string  | Optional tag, e.g. `market`, `technical`        |
| `Created`   | time    |                                                 |

Human feedback mirrors the persona score shape but is **separate** from
persona `Evaluation`s: it is authored by a person, is kept distinct in the
store and in all output, and **does not** affect the feasibility score.

### Resource (research item)
| Field     | Type    | Notes                                          |
|-----------|---------|------------------------------------------------|
| `ID`      | string  | Generated                                       |
| `IdeaID`  | string  | Owning idea                                     |
| `URL`     | string  | Required, validated                             |
| `Title`   | string  | Optional, user-supplied                         |
| `Kind`    | string  | One of `article`, `repo`, `paper`, `video`, `other` |
| `Note`    | string  | Optional — why it is relevant                   |
| `Created` | time    |                                                 |

A `Resource` attaches external research (an article, a repository, a paper,
etc.) to an idea. Metadata is **not** auto-fetched in v1; `Title`/`Note` are
user-supplied. `Kind` is a fixed set with `other` as the fallback.

## 3. Scoring model

- Each persona returns a **Score in [0, 5]** (integers only; non-integer values
  are rejected). The small range keeps the LLM well-calibrated.
- **Rubric anchoring (convention)**: every persona's `SystemPrompt` should define
  what its 0–5 scale means (e.g. "5 = immediately viable, 0 = fundamentally
  flawed"). Built-in personas ship with anchors. This is a documented convention,
  not a hard validation — the engine does not reject a persona whose prompt lacks
  anchors (free-text detection is unreliable), but it logs a warning. The fixed
  evaluation instruction (§5) always appends the JSON-output requirement
  regardless of anchors.
- The engine computes the aggregate over **successful** evaluations only:

  ```
  total = 100 * ( Σ(score_i * weight_i) / (5 * Σ(weight_i)) )
  ```

  where the sum runs over personas with `Status=success`, clamped to `[0, 100]`,
  rounded to one decimal.
- **Coverage**: the `FeasibilityScore` reports `Requested` and `Responded` counts.
  A run with `Responded < Requested` still returns a score, but callers should
  treat reduced coverage as lower confidence.
- **Disagreement**: the `FeasibilityScore` reports `Spread`, defined as
  `max(score_i) − min(score_i)` over personas with `Status=success` (single
  success → spread 0). `Spread` is exact (scores are integers) on the 0–5
  scale. Interpretation (used by every surface): `consensus` for spread 0–1,
  `mixed` for 2–3, `divided` for 4–5.
- **Failure handling**: if *all* personas fail, evaluation returns an error and no
  `FeasibilityScore` is produced.
- **Zero-weight guard**: if `Σ(weight_i) = 0` over the successful personas,
  evaluation returns an error (avoids division by zero).
- **Synthesis is separate and non-numeric**: an optional second-stage pass
  (see §4, §5) produces a text `Summary` and a categorical `Verdict`; it never
  alters the deterministic `Total`.

### Acceptance criteria

- **Given** scores `{skeptic: 2, optimist: 4, engineer: 3}` with default weights,
  **When** the engine aggregates, **Then** `total = 100 * (9 / 15) = 60.0`.
- **Given** a persona weight of `0`, **When** the engine aggregates, **Then** that
  persona contributes nothing to the numerator or denominator.
- **Given** a score outside `[0,5]` or a non-integer score, **When** parsing the
  LLM response, **Then** it is rejected as an invalid result.
- **Given** a failed persona among others that succeed, **When** the engine
  aggregates, **Then** the failed persona is excluded and `Requested` counts it
  while `Responded` does not.
- **Given** all personas fail, **When** evaluation runs, **Then** it returns an
  error and no `FeasibilityScore` is produced.
- **Given** all successful personas have weight `0`, **When** the engine
  aggregates, **Then** it returns an error (division by zero guard).
- **Given** successful scores `{skeptic: 1, optimist: 5, engineer: 3}`,
  **When** the engine aggregates, **Then** `spread = 5 − 1 = 4` (`divided`).
- **Given** a single successful persona (others failed), **When** the engine
  aggregates, **Then** `spread = 0` (`consensus`).

## 4. Application behaviors

The system has two layers:

- **Engine** (`internal/engine`): a **pure** package with no I/O dependencies. It
  depends only on the `llm.Provider` interface and performs evaluation +
  scoring. Fully unit-testable with a mocked provider.
- **Service** (application layer): orchestrates the engine and the `Store` for
  the stateful behaviors below (CRUD, feedback, resources).

The behaviors below describe the whole application surface; each maps 1:1 to a
test.

### Register an idea

- **Given** a valid `Title` and optional `Body`/`Tags`, **When** `CreateIdea` is
  called, **Then** the idea is stored with a generated `ID` and timestamps, and
  is retrievable by `GetIdea`.
- **Given** an empty `Title`, **When** `CreateIdea` is called, **Then** it
  returns a validation error.
- **Given** an existing idea whose trimmed `Title` matches the new title
  case-insensitively, **When** `CreateIdea` is called, **Then** the idea is
  still created and a warning is returned.
- **Given** no idea with a matching title, **When** `CreateIdea` is called,
  **Then** no warning is returned.

### List ideas

- **Given** registered ideas, **When** `ListIdeas` is called, **Then** all ideas
  are returned ordered by `Created` descending.
- **Given** a `tag` filter, **When** `ListIdeas` is called, **Then** only ideas
  carrying that tag are returned.

### Update an idea

- **Given** an existing idea, **When** `UpdateIdea(id, ...)` is called with one
  or more provided fields, **Then** the provided fields are merged into the idea
  (unprovided fields are unchanged) and `Updated` is refreshed.
- **Given** an idea with a body and an update providing only a title, **When**
  `UpdateIdea` is called, **Then** the title changes and the body is preserved.
- **Given** a provided empty `Title`, **When** `UpdateIdea` is called, **Then** it
  returns a validation error.
- **Given** an unknown id, **When** `UpdateIdea` is called, **Then** it returns
  a not-found error.

### Delete an idea

- **Given** an idea exists, **When** `DeleteIdea(id)` is called, **Then** the
  idea is removed and a subsequent `GetIdea` returns not-found.
- **Given** an unknown id, **When** `DeleteIdea(id)` is called, **Then** it
  returns a not-found error.

### Update a persona

- **Given** an existing custom persona, **When** `UpdatePersona(id, ...)` is
  called with one or more provided fields, **Then** the provided fields are
  merged into the persona (unprovided fields are unchanged) and its `Version`
  auto-increments by one.
- **Given** a persona created with `Version` 1, **When** `UpdatePersona` is
  called once, **Then** the resulting `Version` is 2.
- **Given** a `Name` or `SystemPrompt` provided as empty, **When**
  `UpdatePersona` is called, **Then** it returns a validation error.
- **Given** a `Weight` provided as negative, **When** `UpdatePersona` is
  called, **Then** it returns a validation error.
- **Given** a `Weight` provided as `0`, **When** `UpdatePersona` is called,
  **Then** the persona is stored with `Weight` `1.0` (the default).
- **Given** no provided fields (an empty patch), **When** `UpdatePersona` is
  called, **Then** it returns a validation error.
- **Given** a built-in persona id, **When** `UpdatePersona` is called, **Then**
  it returns a conflict error (built-ins are not editable).
- **Given** an unknown id, **When** `UpdatePersona` is called, **Then** it
  returns a not-found error.

### Create a persona

- **Given** a valid `Name` and `SystemPrompt` (and optional `Weight`), **When**
  `CreatePersona` is called, **Then** the persona is stored with a **generated
  ULID**, `Version` 1, and timestamps, and is retrievable by `GetPersona`.
- **Given** an empty `Name` or `SystemPrompt`, **When** `CreatePersona` is
  called, **Then** it returns a validation error.
- **Given** a negative `Weight`, **When** `CreatePersona` is called, **Then**
  it returns a validation error.
- **Given** a `Weight` of `0`, **When** `CreatePersona` is called, **Then** the
  persona is stored with `Weight` `1.0` (the default).

### Evaluate an idea

- **Given** an idea and a set of personas, **When** `Evaluate(ctx, idea,
  personas)` is called, **Then** the engine calls the LLM provider once per
  persona with that persona's `SystemPrompt`, parses each result into an
  `Evaluation` (score + rationale), and returns a `FeasibilityScore`.
- **Given** a persona's call fails with a retryable error or returns unparseable
  output, **When** `Evaluate` runs, **Then** the engine retries that persona once,
  then records an `Evaluation` with `Status=failed` and a non-empty `Error` for
  that persona and continues with the others.
- **Given** a persona's call fails with a non-retryable error (e.g. `ErrAuth`),
  **When** `Evaluate` runs, **Then** the engine records `Status=failed`
  immediately without retrying.
- **Given** the provider returns a JSON object `{"score": 3, "rationale": "..."}`
  **When** parsing, **Then** the engine produces an `Evaluation` with
  `Status=success` and score 3.
- **Given** a persona with `Version` `2`, **When** `Evaluate` runs, **Then**
  the resulting `Evaluation` records `PersonaVersion` `2`.
- **Given** the provider returns non-JSON, a non-integer, or an out-of-range
  score, **When** parsing, **Then** the result is treated as unparseable (see
  retry rule above).
- **Given** a successful `Evaluate`, **When** it completes, **Then** the run
  (aggregate) and its `Evaluation`s are persisted atomically in a single
  transaction under a shared `RunID` (auto-persist).
- **Given** a persistence failure mid-run, **When** `Evaluate` completes, **Then**
  no partial run is left behind (the transaction rolls back).
- **Given** an empty set of personas, **When** `Evaluate` is invoked, **Then** it
  returns a validation error.
- **Given** a requested persona id that does not exist, **When** `Evaluate` is
  invoked, **Then** it returns a validation error.
- **Given** a set of persona ids containing duplicates, **When** `Evaluate` is
  invoked, **Then** each distinct persona is evaluated once (duplicates are
  ignored).

When the persona set is omitted, `Evaluate` uses **all** personas in the store
(built-in and custom).

### Synthesize an evaluation

- **Given** a completed run and a request to synthesize (`summary` opt-in),
  **When** `Evaluate` runs, **Then** the engine makes one extra LLM call with
  the synthesizer prompt (§5), feeding the idea and the `FeasibilityScore`, and
  populates `Summary` and `Verdict` (best-effort, see below).
- **Given** synthesis is not requested, **When** `Evaluate` runs, **Then**
  `Summary` and `Verdict` are empty.
- **Given** the synthesizer returns `{"verdict": "promising", "summary": "..."}`,
  **When** parsing, **Then** the engine records both fields.
- **Given** `responded < requested`, **When** synthesizing, **Then** `Verdict`
  is `inconclusive` and the summary acknowledges the failed personas.
- **Given** the synthesizer returns non-JSON, an unknown `verdict`, or an empty
  summary, **When** parsing, **Then** the result is treated as unparseable
  (retried once on transient errors, then `Summary`/`Verdict` are left empty).
- **Given** the synthesizer call fails after its retry, **When** `Evaluate`
  completes, **Then** the run is still returned and persisted with empty
  `Summary`/`Verdict` (best-effort; synthesis never fails the run).

### List evaluation history

- **Given** an idea with past evaluations, **When** `ListRuns(ideaID)` is called,
  **Then** all runs are returned ordered by `RunID` ascending, each as a
  `FeasibilityScore` whose `Breakdown` contains its evaluations ordered by
  `Created` ascending.
- **Given** an idea with no evaluations, **When** `ListRuns(ideaID)` is called,
  **Then** an empty list is returned.

### Add human feedback

- **Given** an idea, an `Author`, a `Score` in `[0,5]`, and a `Rationale`, **When**
  `AddFeedback` is called, **Then** the feedback is stored with a generated `ID`
  and `Created` timestamp.
- **Given** an empty `Author` or a score outside `[0,5]`, **When** `AddFeedback`
  is called, **Then** it returns a validation error.

### List human feedback

- **Given** an idea with feedback, **When** `ListFeedback(ideaID)` is called,
  **Then** all feedback is returned ordered by `Created` ascending.
- **Given** an idea with no feedback, **When** `ListFeedback(ideaID)` is called,
  **Then** an empty list is returned.

### Delete human feedback

- **Given** feedback exists, **When** `DeleteFeedback(id)` is called, **Then** it
  is removed.
- **Given** an unknown feedback id, **When** `DeleteFeedback(id)` is called,
  **Then** it returns a not-found error.

### Attach a resource

- **Given** an idea, a valid `URL`, and an optional `Title`/`Kind`/`Note`, **When**
  `AddResource` is called, **Then** the resource is stored with a generated `ID`
  and `Created` timestamp.
- **Given** an invalid `URL`, **When** `AddResource` is called, **Then** it
  returns a validation error.
- **Given** a `Kind` outside `{article, repo, paper, video, other}`, **When**
  `AddResource` is called, **Then** it returns a validation error.

### List resources

- **Given** an idea with resources, **When** `ListResources(ideaID)` is called,
  **Then** all resources are returned ordered by `Created` ascending.
- **Given** an idea with no resources, **When** `ListResources(ideaID)` is
  called, **Then** an empty list is returned.

### Remove a resource

- **Given** a resource exists, **When** `DeleteResource(id)` is called, **Then**
  it is removed.
- **Given** an unknown resource id, **When** `DeleteResource(id)` is called,
  **Then** it returns a not-found error.

## 5. LLM provider contract

```go
package llm

type Message struct {
    Role    string // "system" | "user"
    Content string
}

type Request struct {
    Model       string
    Messages    []Message
    Temperature float64
    MaxTokens   int
    JSONMode    bool // request structured JSON output
}

type Response struct {
    Content string
    // optional usage metadata
}

type Provider interface {
    Complete(ctx context.Context, req Request) (Response, error)
}
```

- `OpenAIProvider` is the first implementation (OpenAI-compatible chat
  completions API, `json_object` response format when `JSONMode` is set).
- Providers are discovered/configured via config (§9); the interface is designed
  so `anthropic`, `ollama`, etc. can be added without engine changes.
- Errors are surfaced as typed errors the engine can distinguish:
  `ErrRateLimited` (429), `ErrAuth` (401/403), `ErrProvider` (network/5xx), and
  timeout via `ctx`. `ErrRateLimited`, `ErrProvider`, and timeouts are
  **retryable**; `ErrAuth` and invalid-request errors are **not retried**.

### Evaluation prompt & output contract

For each persona, the engine sends:

- a **system message** = the persona's `SystemPrompt` (which by convention
  includes its 0–5 rubric anchors), plus a fixed instruction: *"Respond with a
  single JSON object with exactly two keys: `score` (an integer from 0 to 5)
  and `rationale` (a short string). No other text."*
- a **user message** = the idea's `Title` and `Body`.

The engine parses the provider content as strict JSON against this schema:

```json
{
  "score": 3,
  "rationale": "string"
}
```

- `score` must be an **integer** in `[0, 5]`; any other type/value is invalid.
- Missing keys, extra content, or non-JSON content is invalid.
- `Temperature` is set to `0` for evaluations (deterministic output) unless
  overridden by config.

### Synthesizer contract

When synthesis is requested, the engine makes one additional LLM call **after**
the persona stage completes:

- a **system message** = the synthesizer prompt: *"You are the SYNTHESIZER, the
  final aggregation stage. You are not a peer evaluator. Synthesize this idea's
  FeasibilityScore — deterministic total plus per-persona breakdown — into a
  single, cohesive, decision-useful verdict. Do not merely list what each
  persona said; combine themes into a unified analysis. Ground truth: base the
  verdict solely on the provided data; reference the deterministic total score;
  do not generate your own numeric scores, invent personas, or hallucinate.
  Summary must be strictly 3–5 sentences: (1) bottom-line verdict with the
  total; (2) core strengths; (3) most critical risks; (4/5) recommendation and,
  if any requested persona failed, an explicit acknowledgment of the incomplete
  coverage."*
- a **user message** = the idea's `Title` and `Body` plus the `FeasibilityScore`
  (total, requested, responded, breakdown).

The engine parses the provider content as strict JSON:

```json
{
  "verdict": "promising",
  "summary": "string"
}
```

- `verdict` must be exactly one of `proceed`, `promising`, `risky`, `pass`,
  `inconclusive` (case-sensitive). When `responded < requested`, the engine
  requires `inconclusive`.
- `summary` must be a non-empty, non-whitespace string.
- Missing keys, extra keys, or non-JSON content is invalid.
- The synthesizer produces **text only — no numeric score**.

### Acceptance criteria

- **Given** a fake HTTP server returning `200` with valid completions JSON,
  **When** `OpenAIProvider.Complete` is called, **Then** it returns the content.
- **Given** a fake HTTP server returning `429`, **When** `Complete` is called,
  **Then** it returns a rate-limit error (not a success).
- **Given** provider content `{"score": 3, "rationale": "ok"}`, **When** the
  engine parses it, **Then** it yields score 3 and rationale "ok".
- **Given** provider content `{"score": 3.5, ...}`, **When** the engine parses
  it, **Then** it is rejected as invalid.
- **Given** synthesizer content `{"verdict": "promising", "summary": "..."}`,
  **When** the engine parses it, **Then** it yields `Verdict=promising` and the
  summary text.
- **Given** synthesizer content with an unknown `verdict` or an empty
  `summary`, **When** the engine parses it, **Then** it is rejected as invalid.

## 6. Persistence (registry)

- SQLite storage, default file `aibreak.db` in the config directory (or a
  `--db` override). In tests, use `:memory:` or a temp file.
- The registry owns the schema + migrations. Tables:

  - `ideas(id, title, body, tags, created_at, updated_at)`
  - `personas(id, name, system_prompt, weight, version INTEGER, created_at)`
  - `runs(run_id, idea_id, total, requested, responded, summary, verdict, spread, created_at)`
  - `evaluations(run_id, idea_id, persona_id, persona_version INTEGER, weight, status, score, rationale, error, created_at)`
  - `feedbacks(id, idea_id, author, score, rationale, aspect, created_at)`
  - `resources(id, idea_id, url, title, kind, note, created_at)`
  - `provider_keys(id, provider, label, hint, is_default, created_at)` — metadata only; the secret lives in the OS keyring

- **Encoding**: `ideas.tags` is a JSON array of strings in a single TEXT column.
  `ListIdeas(tag)` matches ideas whose `tags` array contains the exact tag value.
- **Timestamps** are stored as RFC3339 UTC text (e.g. `2026-09-11T10:00:00Z`),
  which sorts lexicographically in chronological order. Timestamps have
  second precision, so list ordering breaks same-second ties by the ULID `id`
  (lexicographically time-ordered).
- **Nullability**: in `evaluations`, `score` is `NULL` when `Status=failed`;
  `rationale` is the empty string on failure and `error` is the empty string on
  success.

- Built-in personas are seeded on first run; custom personas are created,
  listed, updated, and deleted at runtime. Editing a custom persona
  auto-increments its `Version`; built-in personas are neither editable nor
  deletable.
- **Cascade policy**: deleting an idea cascade-deletes its **runs**, evaluations,
  feedback, and resources (`ON DELETE CASCADE`). Deleting a persona does **not**
  delete past evaluations — they remain as historical records because each
  captures a `PersonaVersion` snapshot.
- The registry is behind an interface so the engine stays store-agnostic:

  ```go
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

  `UpdateIdea` writes a full object; the **service** layer merges a partial
  update into the stored idea before calling it. Partial updates use an
  `IdeaPatch` type with pointer/optional fields (`*string`, `*[]string`): a nil
  field means "unchanged", a non-nil field means "set to this value" (including
  an empty value, which clears the field).

  `SaveRun` persists the scalar fields of `score` (run_id, idea_id, total,
  requested, responded, summary, verdict, spread, created) as the `runs` row
  and persists each entry of `evals` as an `evaluations` row, atomically.
  `score.Breakdown` is ignored on write; the breakdown is reconstructed from
  `evals` on read via `ListRuns`.

  Like `UpdateIdea`, `UpdatePersona` writes a full object; the **service**
  merges a `PersonaPatch` into the stored persona before calling it.

### Acceptance criteria

- **Given** an empty store, **When** an idea is created, **Then** it can be read
  back with identical fields (plus generated `ID`).
- **Given** an idea is updated with a subset of fields, **When** the store is
  queried, **Then** the updated fields change, the others are preserved, and
  `Updated` is refreshed.
- **Given** ideas titled "Note App" and "note app", **When**
  `FindIdeasByTitle("NOTE APP")` is called, **Then** both ideas are returned
  (case-insensitive, trimmed).
- **Given** an idea is deleted, **When** the store is queried, **Then** it is
  not present, and its runs, evaluations, feedback, and resources are also
  removed.
- **Given** a custom persona is created, **When** the store is queried, **Then**
  it is returned with its `Version` and `Created` timestamp.
- **Given** a persona with a negative `Weight`, **When** `CreatePersona` is
  called, **Then** it returns a validation error.
- **Given** a persona `ID` that already exists, **When** `CreatePersona` is
  called, **Then** it returns a conflict error.
- **Given** a persona whose `SystemPrompt` lacks 0–5 rubric anchors, **When**
  `CreatePersona` is called, **Then** it is accepted but a warning is logged.
- **Given** a built-in persona, **When** `DeletePersona` is called, **Then** it
  returns an error (built-ins are not deletable).
- **Given** a custom persona updated with a subset of fields, **When** the store
  is queried, **Then** the updated fields change and the others are preserved.
- **Given** a built-in persona, **When** `UpdatePersona` is called, **Then** it
  returns a conflict error (built-ins are not editable).
- **Given** a run with evaluations is saved, **When** `ListRuns` is called,
  **Then** it returns a `FeasibilityScore` per run, each with `Total`,
  `Requested`, `Responded`, `Summary`, `Verdict`, `Spread`, and a `Breakdown`
  whose evaluations carry `PersonaVersion`, `Weight`, `Status`, score, and
  rationale, ordered by `RunID` then `Created`.
- **Given** a run saved with a `Summary` and `Verdict`, **When** `ListRuns` is
  called, **Then** both are returned with the run.
- **Given** feedback is added, **When** `ListFeedback` is called, **Then** it is
  returned with its `Author`, score, rationale, and optional `Aspect`.
- **Given** feedback is deleted, **When** `ListFeedback` is called, **Then** it
  is not present.
- **Given** a resource is added, **When** `ListResources` is called, **Then** it
  is returned with its `URL`, `Kind`, `Title`, and optional `Note`.

## 7. CLI interface

Command: `aibreak`. Uses `cobra`. Exit code `0` on success, non-zero on
error.

| Command                      | Description                              | Flags                       |
|------------------------------|------------------------------------------|-----------------------------|
| `aibreak evaluate <id>` | Evaluate an idea with all/selected personas | `--personas a,b`, `--summary`, `--json` |
| `aibreak registry add`  | Create an idea                            | `--title`, `--body`, `--tag` |
| `aibreak registry list` | List ideas                                | `--tag`, `--json`           |
| `aibreak registry get <id>` | Show one idea                          | `--json`                    |
| `aibreak registry edit <id>` | Update an idea                        | `--title`, `--body`, `--tag` |
| `aibreak registry rm <id>` | Delete an idea                         |                             |
| `aibreak persona list`  | List available personas                   | `--json`                    |
| `aibreak persona add`   | Create a custom persona                   | `--name`, `--prompt`, `--weight` |
| `aibreak persona edit <id>` | Update a custom persona                 | `--name`, `--prompt`, `--weight` |
| `aibreak persona rm <id>` | Delete a custom persona                 |                             |
| `aibreak history <id>`    | List an idea's past evaluations         | `--json`                    |
| `aibreak feedback add <id>` | Add human feedback                     | `--author`, `--score`, `--rationale`, `--aspect` |
| `aibreak feedback list <id>` | List an idea's human feedback         | `--json`                    |
| `aibreak feedback rm <id>` | Delete human feedback                   |                             |
| `aibreak resource add <id>` | Attach a research resource             | `--url`, `--title`, `--kind`, `--note` |
| `aibreak resource list <id>` | List an idea's resources             | `--json`                    |
| `aibreak resource rm <id>` | Remove a resource                        |                             |

- `--json` switches output to machine-readable JSON.
- Text output is human-readable (scores and rationale).
- `registry edit` flags are optional; only provided flags change the idea.
- `persona add` prints the generated persona ID; `persona edit` flags are
  optional and only provided flags change the persona (the version
  auto-increments).
- A global `--db <path>` (or `--db=<path>`) flag overrides the SQLite database
  path (same precedence as the `AIBREAK_DB` env var, per §9).

### Acceptance criteria

- **Given** `registry add --title "X"`, **When** run, **Then** it prints the new
  idea's ID and exits 0.
- **Given** `registry add --title "X"` when an idea titled "X" already exists,
  **When** run, **Then** it prints the new ID to stdout, a warning to stderr,
  and exits 0.
- **Given** `registry add` with no title, **When** run, **Then** it prints an
  error and exits non-zero.
- **Given** `registry list --tag t`, **When** run, **Then** it prints only ideas
  tagged `t`, exiting 0.
- **Given** `registry get <id>`, **When** run, **Then** it prints the idea's
  title, body, and tags, exiting 0.
- **Given** `registry edit <id> --title "Y"`, **When** run, **Then** it updates
  the idea and exits 0.
- **Given** `aibreak --db <path> registry add --title "X"`, **When** run, **Then**
  the idea is stored in the database at `<path>` and exits 0.
- **Given** `evaluate <id>` with a mocked provider, **When** run, **Then** it
  prints per-persona scores and the aggregate total with spread, exiting 0.
- **Given** `evaluate <id> --summary` with a mocked provider, **When** run,
  **Then** it prints the aggregate total, the breakdown, and the synthesized
  `verdict` + `summary`, exiting 0.
- **Given** `history <id>` with saved evaluations, **When** run, **Then** it
  prints past evaluations grouped by run in chronological order (with `spread`
  and `verdict` when present), exiting 0.
- **Given** `persona edit <id> --prompt "..."`, **When** run, **Then** it
  updates the persona (bumping its version) and exits 0.
- **Given** `persona add --name "N" --prompt "P"`, **When** run, **Then** it
  prints the generated persona ID and exits 0.
- **Given** `persona edit skeptic ...`, **When** run, **Then** it prints an
  error and exits non-zero (built-ins are not editable).
- **Given** `feedback add <id> --author A --score 3 --rationale "..."`, **When**
  run, **Then** it stores the feedback and exits 0.
- **Given** `feedback add <id>` with a missing `--author`, **When** run, **Then**
  it prints an error and exits non-zero.
- **Given** `resource add <id> --url https://...`, **When** run, **Then** it
  stores the resource and exits 0.
- **Given** `resource add <id>` with an invalid `--url`, **When** run, **Then**
  it prints an error and exits non-zero.

## 8. HTTP API interface

Server: `aibreakd`. JSON over HTTP. Errors use a consistent envelope:

```json
{ "error": { "code": "not_found", "message": "..." } }
```

The API has **no authentication** (single-user local tool, §1). It binds to
`127.0.0.1:8080` by default; if `AIBREAK_ADDR` binds to a non-loopback address,
`aibreakd` logs a warning that the API is exposed to the network.

| Method | Path                | Description                    | Success |
|--------|---------------------|--------------------------------|---------|
| POST   | `/v1/ideas`         | Create an idea                 | 201     |
| GET    | `/v1/ideas`         | List ideas (`?tag=` filter)    | 200     |
| GET    | `/v1/ideas/{id}`    | Get an idea                    | 200     |
| PATCH  | `/v1/ideas/{id}`    | Update an idea                 | 200     |
| DELETE | `/v1/ideas/{id}`    | Delete an idea                 | 204     |
| POST   | `/v1/ideas/{id}/evaluate` | Evaluate an idea       | 200     |
| GET    | `/v1/ideas/{id}/evaluations` | List evaluation history | 200  |
| POST   | `/v1/ideas/{id}/feedback` | Add human feedback      | 201     |
| GET    | `/v1/ideas/{id}/feedback` | List human feedback     | 200     |
| DELETE | `/v1/ideas/{id}/feedback/{fid}` | Delete feedback  | 204     |
| POST   | `/v1/ideas/{id}/resources` | Add a resource        | 201     |
| GET    | `/v1/ideas/{id}/resources` | List resources        | 200     |
| DELETE | `/v1/ideas/{id}/resources/{rid}` | Remove a resource | 204 |
| GET    | `/v1/personas`      | List personas                  | 200     |
| POST   | `/v1/personas`      | Create a custom persona        | 201     |
| PATCH  | `/v1/personas/{id}` | Update a custom persona        | 200     |
| DELETE | `/v1/personas/{id}` | Delete a custom persona        | 204     |

### Error codes

| code              | HTTP | Meaning                                |
|-------------------|------|----------------------------------------|
| `validation_error`| 400  | Malformed or invalid request body      |
| `not_found`       | 404  | Unknown resource id                    |
| `conflict`        | 409  | Duplicate id (e.g. persona id)        |
| `rate_limited`    | 429  | Upstream LLM rate limit                |
| `llm_error`       | 502  | Upstream LLM/other provider failure    |
| `internal`        | 500  | Unexpected server error                |

### Request/response shapes

Request bodies mirror the domain model (§2) with the required fields noted.

- `POST /v1/ideas` — `{ "title": required, "body"?: string, "tags"?: [string] }`
  → `201` Idea. When the title matches an existing idea (case-insensitive,
  trimmed), the response also carries an `X-Warning` header; the body remains a
  bare Idea.
- `PATCH /v1/ideas/{id}` — partial `{ "title"?, "body"?, "tags"? }` → `200` Idea
  (unspecified fields are unchanged).
- `POST /v1/ideas/{id}/evaluate` — `{ "personas"?: [string], "summary"?: bool }`
  (empty/omitted `personas` = all personas, built-in + custom) → `200`
  FeasibilityScore (always with `spread`; with `summary`/`verdict` populated
  only when `"summary": true` is requested and synthesis succeeds).
- `POST /v1/ideas/{id}/feedback` — `{ "author": required, "score": required,
  "rationale"?: string, "aspect"?: string }` → `201` Feedback.
- `POST /v1/ideas/{id}/resources` — `{ "url": required, "title"?: string,
  "kind"?: string, "note"?: string }` → `201` Resource.
- `POST /v1/personas` — `{ "name": required, "system_prompt": required,
  "weight"?: float }` → `201` Persona (with a generated `id` and `version` 1).
- `PATCH /v1/personas/{id}` — partial `{ "name"?, "system_prompt"?,
  "weight"? }` → `200` Persona (unspecified fields are unchanged; the version
  auto-increments; a built-in id returns `409`).
- `GET /v1/ideas/{id}/evaluations` → `200` JSON array of `FeasibilityScore`
  objects (each with `run_id`, `total`, `requested`, `responded`, `spread`,
  `summary`, `verdict`, and a `breakdown` of evaluations), ordered by `run_id`
  ascending.
- Other list endpoints return JSON arrays.

### Acceptance criteria

- **Given** a `POST /v1/ideas` with `{"title":"X"}`, **When** handled, **Then**
  it returns `201` and a JSON idea with an `id`.
- **Given** a `POST /v1/ideas` whose title matches an existing idea, **When**
  handled, **Then** it returns `201` with an `X-Warning` header set.
- **Given** a `GET /v1/ideas/{unknown}`, **When** handled, **Then** it returns
  `404` with the error envelope.
- **Given** a `PATCH /v1/ideas/{id}` with `{"title":"Y"}`, **When** handled,
  **Then** it returns `200` with the updated idea.
- **Given** `POST /v1/ideas/{id}/evaluate` with a mocked provider, **When**
  handled, **Then** it returns `200` with a `FeasibilityScore` body.
- **Given** `POST /v1/ideas/{id}/evaluate` with `{"summary": true}` and a mocked
  provider, **When** handled, **Then** it returns `200` with `summary` and
  `verdict` populated.
- **Given** `POST /v1/ideas/{unknown}/evaluate`, **When** handled, **Then** it
  returns `404` with the error envelope.
- **Given** `POST /v1/ideas/{id}/evaluate` where all personas fail, **When**
  handled, **Then** it returns `502` with code `llm_error`.
- **Given** `GET /v1/ideas/{id}/evaluations` after evaluations exist, **When**
  handled, **Then** it returns `200` with a JSON array of `FeasibilityScore`
  objects (each with `run_id`, `total`, `requested`, `responded`, `spread`,
  `summary`, `verdict`, and `breakdown`), ordered by `run_id` ascending.
- **Given** a `PATCH /v1/personas/{id}` with `{"name":"New"}`, **When**
  handled, **Then** it returns `200` with the updated persona (whose `version`
  incremented).
- **Given** a `PATCH /v1/personas/{skeptic}` (built-in), **When** handled,
  **Then** it returns `409` with code `conflict`.
- **Given** `POST /v1/ideas/{id}/feedback` with a valid author/score, **When**
  handled, **Then** it returns `201` with the stored feedback body.
- **Given** `POST /v1/ideas/{id}/resources` with a valid URL, **When** handled,
  **Then** it returns `201` with the stored resource body.
- **Given** an invalid request body, **When** handled, **Then** it returns `400`
  with code `validation_error`.

## 9. Configuration

Sources, in priority order: flags > env vars > config file > defaults.

| Setting            | Env var                  | Default             |
|--------------------|--------------------------|---------------------|
| LLM provider       | `AIBREAK_LLM_PROVIDER` | `openai`         |
| OpenAI API key     | `OPENAI_API_KEY`         | —                   |
| Model              | `AIBREAK_LLM_MODEL` | `gpt-4o-mini`       |
| Temperature        | `AIBREAK_LLM_TEMPERATURE` | `0`              |
| Max tokens         | `AIBREAK_LLM_MAX_TOKENS` | `512`            |
| Timeout            | `AIBREAK_LLM_TIMEOUT`    | `60s`            |
| Retries            | `AIBREAK_LLM_RETRIES`    | `1`              |
| DB path            | `AIBREAK_DB`        | `aibreak.db`   |
| API listen address | `AIBREAK_ADDR`      | `127.0.0.1:8080`    |

The desktop app (`aibreak-desktop`) manages LLM provider API keys as
first-class objects (§11): key **metadata** (id, label, masked hint, default
flag) lives in the SQLite store, while each **secret** lives in the **OS
keyring** (service `aibreak`, account `<provider>:<keyID>`). One key per
provider is the **default**; the desktop applies it to the running provider at
startup and whenever the default changes, taking precedence over the
`OPENAI_API_KEY` env var / config file. If persisting the metadata fails after
the secret was written, the secret is removed from the keyring (best-effort
rollback).

## 10. Non-functional requirements

- The engine must be deterministic given the same provider output (no hidden
  randomness outside the provider).
- All external I/O (LLM, SQLite, HTTP) is behind interfaces for testability.
- Cancellation: all engine/provider operations honor `context.Context`.
- Logging: structured logging via `log/slog`.

## 11. Desktop app (aibreak-desktop)

`aibreak-desktop` is a local, single-user Wails desktop app. Its Go backend
binds `internal/service` methods directly (in-process, no HTTP); see the
technical spec for the binding contract. It introduces **no new domain
behavior** — every view below maps to existing service methods (§4). The only
new capability is the **provider settings screen**, which manages API keys as
first-class objects: metadata in the store, secrets in the OS keyring, with one
default key applied to the running provider (see §9).

### Screens

1. **Welcome** — app branding and a single **Start** button that transitions
   into the main app. No backend call; it is the seam for future
   authentication (still deferred per §1).

2. **Main app** — a left sidebar (navigation) and a content area:
   - **Registry** → **Ideas**, **Personas**.
   - **LLM Provider** → **OpenAI**.

### Views

- **Ideas** — a grid of cards plus a **Create / Register Idea** button. Each
  card shows the title and the first sentences of the body, a **feasibility
  dot** (top-right), and actions to **edit**, **delete**, and open **history**.
  Clicking a card opens the detail view.
- **Idea detail** — title, body, and tags (editable), creation/update time, a
  **Resources** section (add/remove `{url, title, kind, note}`), an **Evaluate**
  panel (persona checkboxes + a summary toggle + Evaluate), **History** (past
  runs), and **Feedback** (list + add).
- **Personas** — lists all personas with their `Name`, generated `ID`, and
  auto-incremented `Version`; built-ins are marked read-only, custom personas
  can be created, edited (each edit bumps the version), and deleted.
- **OpenAI provider** — shows the provider name/model and a list of registered
  API keys (label + masked suffix + default flag); the user can **add**, **set
  default**, and **delete** keys (the secret is stored in the keyring; metadata
  in the store).

### Feasibility dot

Each idea card is color-coded from the **latest run's `Total`** (0–100):

| Score | Color | Meaning |
| --- | --- | --- |
| no run | gray | not evaluated yet |
| `<= 50` | red | not feasible |
| `51 – 69` | yellow | middling |
| `>= 70` | green | feasible |

The latest run is the one with the greatest `RunID` (ULIDs are time-ordered;
`ListRuns` returns runs ordered ascending by `RunID`).

### Acceptance criteria

- **Given** registered ideas, **When** the ideas view loads, **Then** it shows
  every idea as a card with a feasibility dot derived from its latest run (gray
  when it has none).
- **Given** an idea and selected personas, **When** evaluation runs, **Then**
  the results show the total, spread, breakdown, and — when synthesis was
  requested — the verdict and summary, and the card's dot reflects the new
  total.
- **Given** an idea with past runs, **When** the history view loads, **Then**
  it shows each run's total, spread, and verdict (when present).
- **Given** valid author, score, and rationale, **When** feedback is submitted,
  **Then** it is stored and appears in the feedback view.
- **Given** a valid URL, **When** a resource is added, **Then** it is stored
  and appears in the resources view.
- **Given** a custom persona, **When** it is created, **Then** it appears in
  the personas view and is selectable during evaluation; built-ins are shown
  but not deletable.
- **Given** an API key, **When** it is added in the provider view, **Then** its
  metadata appears in the key list (with a masked suffix) and its secret is
  stored in the OS keyring; the first key becomes the default and is used by
  subsequent evaluations.
- **Given** a registered key that is not the default, **When** the user marks it
  default, **Then** the running provider switches to it and the previous default
  is cleared.
- **Given** the default key, **When** it is deleted, **Then** the most recently
  added remaining key is promoted to default (or none if no keys remain).

### Out of scope (still deferred per §1)

- Authentication (the welcome screen is only the seam).
- Model / base-URL / temperature settings in the UI (config-file/env only).
- Multi-provider configuration beyond OpenAI.
