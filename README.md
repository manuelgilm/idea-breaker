# Idea Breaker

`aibreak` breaks an idea down using AI personas (pessimist, optimist, architect) and synthesizes a scored review.

Give it an idea, and it fans the idea out to several personas in parallel, each with its own perspective. It then collects their structured takeaways and has a synthesizer produce an overall verdict with a feasibility score. The result is written to a JSON file.

## Prerequisites

- Go 1.26 or newer
- An API key for an OpenAI-compatible LLM provider (OpenAI by default)

## Install

Build the binary from source:

```bash
go build -o bin/aibreak ./cmd/aibreak
```

Or install it into your Go `bin` directory and run `aibreak` from anywhere:

```bash
go install ./cmd/aibreak
aibreak --help
```

## Configure

The tool reads its configuration from environment variables.

| Variable               | Required | Default            | Purpose                                      |
| ---------------------- | -------- | ------------------ | -------------------------------------------- |
| `OPENAI_API_KEY`       | yes      | —                  | Provider API key. Missing → error (no flag). |
| `AIBREAK_PERSONAS_DIR` | no       | `configs/personas` | Directory containing the persona YAML files. |

There is no `--api-key` flag; credentials come only from the environment.

## Run

```bash
export OPENAI_API_KEY=<your key>
./bin/aibreak --idea "a startup that delivers groceries by drone" --output /tmp/result.json
```

| Flag       | Required | Description                     |
| ---------- | -------- | ------------------------------- |
| `--idea`   | yes      | The idea to break down          |
| `--output` | yes      | Path to the output JSON file    |

Omitting `--idea` or `--output`, or running without `OPENAI_API_KEY`, returns an error.

## Output

The result is indented JSON written to `--output`:

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
    },
    {
      "name": "optimist",
      "summary": "Large convenience upside in dense cities.",
      "key_points": ["fast delivery", "food fresh from source"],
      "risks": ["low margins"],
      "dependencies": ["regulatory approval"],
      "score": 70
    },
    {
      "name": "architect",
      "summary": "Two-stage build: route planner, then fleet.",
      "key_points": ["GPS routing service", "drone docking stations"],
      "risks": ["battery range"],
      "dependencies": ["drone SDK", "maps API"],
      "score": 55
    }
  ],
  "synthesis": {
    "feedback": "Promising niche, but regulators and economics remain the gating risks.",
    "score": 52
  },
  "status": "complete"
}
```

Each persona reports `summary`, `key_points`, `risks`, `dependencies`, and a `score` (0–100). If a persona call fails, its entries are included with an `error` field instead.

## Adjusting the personas

Personas are plain YAML files, one per persona, in `configs/personas/` (or your `AIBREAK_PERSONAS_DIR`). Each has a `name` and a `prompt`:

```yaml
name: pessimist
prompt: |
  You are a ruthless but constructive pessimist evaluating the feasibility of an idea.
  ...
  Respond with ONLY this JSON object, no code fences:
  {"summary": "...", "key_points": ["..."], "risks": ["..."], "dependencies": ["..."], "score": <0-100>}
```

Add a persona by dropping a new file into that directory — no code changes needed. Each persona prompt must instruct the model to reply with a single JSON object of the form `{summary, key_points, risks, dependencies, score}`.

The file `synthesizer.yaml` in that directory is reserved for the synthesizer prompt, which aggregates all persona results into the scored review — it is not a persona.

## How it works

1. The idea is sent to each persona in parallel, along with its system prompt.
2. Every persona returns a structured `{summary, key_points, risks, dependencies, score}` JSON object.
3. Those takeaways are combined and fed to the synthesizer, which returns `{feedback, score}`.
4. The whole result is written to the `--output` path.

## Note on testing with a placeholder key

Using the placeholder `OPENAI_API_KEY=sk-test` verifies the plumbing but hits the real provider and returns `401`. Per-persona `401`s appear in the `error` field and do not stop the run; a `401` during synthesis is a hard failure (exit 1), because the scored review is the deliverable. A real key returns real responses.
