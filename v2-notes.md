# Features to include when planning v2

## Synthesized summary

A final LLM pass (a "summarizer"/"synthesizer" persona/system prompt) that reads
the breakdown and writes a consolidated verdict + recommendation, optionally
added to the aggregate output.

### Architecture

The synthesizer is **not a peer persona** — it is a **second-stage pass** that
consumes the per-persona `breakdown` and writes a consolidated verdict:

1. Stage 1: run personas in parallel → `breakdown` (scores + rationales).
2. Stage 2 (if enabled): one extra LLM call with a "synthesizer" system prompt
   that reads the breakdown and returns a summary (e.g. `{ "summary": "..." }`).

This requires a dedicated synthesizer contract and a `Summary` field on
`FeasibilityScore` (see "Persist vs compute-on-the-fly" below).

### Invocation shape — lean: opt-in `--synthesize` flag (default off)

Considered:

- **`--synthesize` opt-in (default off) — recommended.** Keeps base `evaluate`
  cheap (evaluation already costs N LLM calls); the extra AI pass only runs
  when asked.
- Always-on with `--no-synthesize` to disable. Better default UX, but every
  `evaluate` costs +1 call.
- Separate on-demand command (e.g. `aibreak summarize <id>`). No extra cost at
  eval time and works on history, but a two-step workflow and the summary is
  not part of the evaluate result.

### Persist vs compute-on-the-fly — lean: persist on the run

- **Persist** (recommended): if the summary is produced at eval time, store it
  on the run (a `summary` column on `runs`, a `summary` field in the
  aggregate) so `history` shows past summaries. Otherwise it is lost.
- Compute-on-the-fly: re-running an LLM call over old breakdowns is
  non-deterministic — avoid.

### Flag naming — decided: `--summary`

- CLI: `evaluate --summary`. API: `POST /evaluate` body `{"summary": true}`.
- Verdict labels are also decided (below) — no open items in this section.

### Synthesizer system prompt (defined, via external review)

**Role.** You are the SYNTHESIZER, the final aggregation stage for "aibreak."
You are not a peer evaluator. You consume a user's idea and a
FeasibilityScore — a deterministic report containing a weighted total score
(0–100) and a breakdown of evaluations from various AI "personas" (e.g.
Skeptic, Engineer, Optimist) — and synthesize them into a single, cohesive,
decision-useful verdict as strict JSON.

**Synthesis rules.**

- Synthesize, do not parrot. Do not merely list what each persona said.
  Combine their themes into a unified analysis of the idea's viability.
- Ground truth. Base the summary solely on the provided FeasibilityScore data.
  Reference the deterministic total score. Do not generate your own numeric
  scores, invent new personas, or hallucinate risks/pros not explicitly raised
  in the breakdown.
- Acknowledge coverage. Compare `requested` and `responded`. If
  `responded < requested`, explicitly state that the evaluation is incomplete
  due to failed personas (naming the missing personas if evident) and temper
  the recommendation. Never silently ignore a partial run.
- Adaptive context. The summary must read as bespoke advice tailored to the
  specific mechanisms of THIS idea, using its terminology, not a generic
  template.

**Structure and length.** Strictly 3–5 sentences, in this shape:

1. The bottom-line verdict incorporating the total score.
2. The idea's core strengths, synthesized from the breakdown.
3. The most critical risks or weaknesses identified.
4. (5.) A final recommendation or next step (and, if applicable, the warning
   about missing persona coverage).

**Verdict labels (locked).** Exactly one of:

- `proceed` — strong consensus of viability; minor risks easily mitigated.
- `promising` — high potential, but requires significant iteration to resolve
  identified blockers.
- `risky` — serious flaws, structural risks, or heavily divided opinions
  (flaws only — never incomplete data).
- `pass` — fundamental flaws, safety issues, or unanimous rejection.
- `inconclusive` — reserved for `responded < requested`; the synthesizer MUST
  return this (not `risky`) when any requested persona failed.

### Synthesizer output contract

Strict JSON object. Deviation = failed synthesis step.

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "verdict": {
      "type": "string",
      "enum": ["proceed", "promising", "risky", "pass", "inconclusive"],
      "description": "A categorical label representing the aggregate evaluation."
    },
    "summary": {
      "type": "string",
      "minLength": 20,
      "description": "A 3-5 sentence synthesized summary of the persona evaluations."
    }
  },
  "required": ["verdict", "summary"],
  "additionalProperties": false
}
```

**Invalidation rules.** Reject when: malformed JSON; keys other than `verdict`
and `summary` (e.g. a new numeric-score field); `verdict` not exactly one of
the five allowed strings (case-sensitive); `summary` missing, empty, or
whitespace-only.

### Worked examples

**Example A — full success run.** Idea "SmartCoaster" (thermal-sensor drink
coaster with phone push notification). FeasibilityScore: total 68, requested 3,
responded 3. Optimist 4, Engineer 3, Skeptic 2.

```json
{
  "verdict": "promising",
  "summary": "Earning a total score of 68/100, the SmartCoaster concept offers a fun, highly feasible gadget targeted effectively at distracted office workers. However, it faces notable friction regarding battery constraints in a slim form factor and the potential annoyance of push notifications for trivial events. To improve viability, consider pivoting from a notification model to an active heating element to solve the user's problem directly."
}
```

**Example B — partial run.** Idea "Algae-Based Urban Air Filters" (algae
bioreactors on bus stops). FeasibilityScore: total 42, requested 4, responded
2. Biologist 4, Engineer failed (timeout), City Planner failed (token limit),
Skeptic 2.

```json
{
  "verdict": "risky",
  "summary": "With a total score of 42/100, this urban algae filter concept presents a biologically sound method for carbon capture, but cannot be fully validated because 2 of the 4 requested evaluations (the Engineer and City Planner) failed to complete. The completed feedback highlights severe logistical risks regarding the ongoing maintenance and biomass harvesting at thousands of distributed bus stops. Proceed with caution until the engineering feasibility and urban infrastructure impacts can be successfully evaluated."
}
```

### Verdict labels — decided: 5 labels, separate `inconclusive`

- `proceed` / `promising` / `risky` / `pass` as defined above.
- `inconclusive` is reserved for `responded < requested` (partial coverage).
  The synthesizer MUST return `inconclusive` (not `risky`) when any requested
  persona failed, and MUST acknowledge the missing personas. `risky` therefore
  means structural flaws / divided opinions only — never incomplete data.

### Partial runs — must acknowledge failures

When `Responded < Requested`, the summary must acknowledge that some personas
failed rather than silently ignoring them.

## Deferred (not v2)

### Resources as evaluation context

Resources (`url` / `title` / `note`) are stored and listed only — they are NOT
fed into persona evaluation. Using them as context is deferred beyond v2. It
splits into two scopes with very different cost:

- (a) **Metadata-only enrichment** — feed the stored `title` + `note` into the
  persona prompt. No fetching required.
- (b) **URL content fetching** — browse and extract page content (HTTP fetch,
  HTML→text extraction, token budgeting/capping, SSRF/security, freshness /
  caching, rate limits).

Both are out of scope for v2. If revisited, do (a) before (b); never ship a
half-baked version where URLs silently contribute nothing while notes do.