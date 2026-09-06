# Idea Breaker

`aibreak` breaks an idea down using AI personas (pessimist, optimist, architect) and synthesizes a scored review.

Give it an idea, and it fans the idea out to several personas in parallel, each with its own perspective. It then collects their structured takeaways and has a synthesizer produce an overall verdict with a feasibility score. The result is written to a JSON file.

## Download a prebuilt binary

Prebuilt binaries are published as GitHub Releases — no Go needed. Pick the one matching your OS and architecture, extract it, and run `aibreak`. The personas are embedded in the binary, so it's fully self-contained.

| OS / architecture | Archive |
| ----------------- | ------- |
| Linux (amd64)     | `aibreak_<version>_linux_amd64.tar.gz` |
| Linux (arm64)     | `aibreak_<version>_linux_arm64.tar.gz` |
| macOS (amd64)     | `aibreak_<version>_darwin_amd64.tar.gz` |
| macOS (arm64)     | `aibreak_<version>_darwin_arm64.tar.gz` |
| Windows (amd64)   | `aibreak_<version>_windows_amd64.zip` |
| Windows (arm64)   | `aibreak_<version>_windows_arm64.zip` |

Whoever is releasing creates a tag (e.g. `v1.0.0`); the GitHub Actions workflow builds all targets and publishes them.

## Build from source (developers)

Requires Go 1.26 or newer:

```bash
go build -o bin/aibreak ./cmd/aibreak
```

Or install it into your Go `bin` directory:

```bash
go install ./cmd/aibreak
aibreak --help
```

## Configure

The tool reads its configuration from environment variables.

| Variable               | Required | Default | Purpose                                      |
| ---------------------- | -------- | ------- | -------------------------------------------- |
| `OPENAI_API_KEY`       | yes      | —       | Provider API key. Missing → error (no flag). |
| `AIBREAK_PERSONAS_DIR` | no       | —       | Override the embedded personas with your own YAML dir. |

There is no `--api-key` flag; credentials come only from the environment. By default the personas are embedded in the binary; set `AIBREAK_PERSONAS_DIR` to load your own YAML files from disk instead.

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

Personas are YAML files, one per persona. The built-in ones (`pessimist`, `optimist`, `architect`) plus the synthesizer prompt are **embedded into the binary**. To use customized prompts, point `AIBREAK_PERSONAS_DIR` at a directory you provide — the binary reads one YAML file per persona from it (no need to rebuild):

```yaml
name: pessimist
prompt: |
  You are a ruthless but constructive pessimist evaluating the feasibility of an idea.
  ...
  Respond with ONLY this JSON object, no code fences:
  {"summary": "...", "key_points": ["..."], "risks": ["..."], "dependencies": ["..."], "score": <0-100>}
```

Each persona prompt must instruct the model to reply with a single JSON object of the form `{summary, key_points, risks, dependencies, score}`. The file `synthesizer.yaml` in that directory is reserved for the synthesizer prompt, which aggregates all persona results into the scored review — it is not a persona.

## How it works

1. The idea is sent to each persona in parallel, along with its system prompt.
2. Every persona returns a structured `{summary, key_points, risks, dependencies, score}` JSON object.
3. Those takeaways are combined and fed to the synthesizer, which returns `{feedback, score}`.
4. The whole result is written to the `--output` path.

## Trying it without a key

Use `--mock` to run the full pipeline offline — no `OPENAI_API_KEY` needed. Personas and the synthesizer return deterministic mock responses, and the run always exits 0 with valid JSON:

```bash
./bin/aibreak --mock --idea "a startup that delivers groceries by drone" --output /tmp/result.json
```

A real run requires `OPENAI_API_KEY`. The placeholder `sk-test` reaches the real provider and returns `401`: per-persona `401`s appear in the `error` field and do not stop the run, but a `401` during synthesis is a hard failure (exit 1) because the scored review is the deliverable.
