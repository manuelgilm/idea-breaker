# aibreak

AI-powered idea evaluation engine — a CLI (`aibreak`), an HTTP API
(`aibreakd`), and a desktop app (`aibreak-desktop`) that critique an idea from
multiple AI personas, score it on a 0–5 scale, and aggregate those into a single
feasibility score (0–100).

Specs (source of truth):

- [`PROJECT_SPECIFICATION.md`](PROJECT_SPECIFICATION.md) — behavior & domain model.
- [`TECHNICAL_SPECIFICATION.md`](TECHNICAL_SPECIFICATION.md) — architecture & libraries.

## Requirements

- Go 1.22+ (only to build from source; not needed for prebuilt binaries)
- An OpenAI API key (`OPENAI_API_KEY`) to run `evaluate`
- `golangci-lint` (optional, for `make lint`)

## Install (prebuilt binaries)

Download the archive for your platform from
[GitHub Releases](https://github.com/manuelgilm/idea-breaker/releases).
Each archive (`aibreak-<version>-<os>-<arch>.tar.gz`, `.zip` on Windows)
contains both binaries: `aibreak` and `aibreakd`.

```sh
# Linux / macOS example (replace <version> and pick your platform)
tar xzf aibreak-<version>-linux-amd64.tar.gz
sudo install -m 755 aibreak aibreakd /usr/local/bin/

# verify
aibreak --help
```

On Windows, unzip the archive and add the folder to your `PATH`.

Optionally verify integrity against `checksums.txt` from the same release:

```sh
sha256sum -c checksums.txt --ignore-missing
```

## Build (from source)

```sh
make build            # builds cmd/aibreak and cmd/aibreakd
make test             # go test ./...
make lint             # golangci-lint run (requires golangci-lint)
```

Or run directly without building:

```sh
go run ./cmd/aibreak  <args...>
go run ./cmd/aibreakd
```

## Desktop app

`aibreak-desktop` is a local, single-user Wails desktop app for managing ideas,
evaluating them with AI personas, and managing your LLM API keys (stored in the
OS keyring).

### Install (prebuilt)

Download the `aibreak-desktop` asset for your platform from
[GitHub Releases](https://github.com/manuelgilm/idea-breaker/releases):

- **Linux** — `aibreak-desktop-<version>-linux-amd64.tar.gz`; extract and run the
  `aibreak-desktop` binary.
- **macOS** — `aibreak-desktop-<version>-darwin-universal.zip` (Intel + Apple
  Silicon); unzip and run `aibreak-desktop.app`.
- **Windows** — `aibreak-desktop-<version>-windows-amd64.zip`; unzip and run
  `aibreak-desktop.exe`.

### Build from source

Requires Go 1.22+, Node.js, and the [Wails](https://wails.io) CLI. On Linux also
install the webview dev libraries:

```sh
# Ubuntu 24.04+
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
go install github.com/wailsapp/wails/v2/cmd/wails@latest

make desktop-dev     # live-reload development
make desktop-build   # native binary for the host OS
```

The desktop app stores API keys in the **OS keyring**. On Linux a running Secret
Service (gnome-keyring/KWallet) is required; if absent, it falls back to the
`OPENAI_API_KEY` env var / config file.

## Configuration

Precedence: **flags > env vars > config file > defaults**.

| Setting       | Env var                   | Default              |
|---------------|---------------------------|----------------------|
| LLM provider  | `AIBREAK_LLM_PROVIDER`    | `openai`             |
| API key       | `OPENAI_API_KEY`          | —                    |
| Model         | `AIBREAK_LLM_MODEL`       | `gpt-4o-mini`        |
| Temperature   | `AIBREAK_LLM_TEMPERATURE` | `0`                  |
| Max tokens    | `AIBREAK_LLM_MAX_TOKENS`  | `512`                |
| Timeout       | `AIBREAK_LLM_TIMEOUT`     | `60s`                |
| Retries       | `AIBREAK_LLM_RETRIES`     | `1`                  |
| DB path       | `AIBREAK_DB`              | `<user config dir>/aibreak/aibreak.db` |
| API address   | `AIBREAK_ADDR`            | `127.0.0.1:8080`     |

Optional TOML config file at `$AIBREAK_CONFIG` or
`~/.config/aibreak/config.toml`:

```toml
model = "gpt-4o"
db_path = "/path/to/aibreak.db"
```

## CLI usage

```sh
export OPENAI_API_KEY=sk-...

# register an idea
aibreak registry add --title "A new app" --body "..." --tag demo

# list / inspect / edit / delete ideas
aibreak registry list [--tag demo] [--json]
aibreak registry get <id>
aibreak registry edit <id> --title "New title"
aibreak registry rm <id>

# evaluate with all (or selected) personas; add --summary for a synthesized verdict
aibreak evaluate <id> [--personas skeptic,optimist] [--summary] [--json]
aibreak history <id>        # past evaluation runs (shows verdict when synthesized)

# personas (custom only; built-ins are fixed)
aibreak persona list
aibreak persona add --name Investor --prompt "..." [--weight 1.5]   # prints the generated ID
aibreak persona edit <id> [--name ...] [--prompt ...] [--weight ...]  # version auto-increments
aibreak persona rm <id>

# human feedback
aibreak feedback add <id> --author "Alice" --score 4 --rationale "..." [--aspect market]
aibreak feedback list <id>
aibreak feedback rm <id>

# research resources
aibreak resource add <id> --url https://... [--title ...] [--kind article|repo|paper|video|other] [--note ...]
aibreak resource list <id>
aibreak resource rm <id>
```

Built-in personas: `skeptic`, `optimist`, `engineer`.

## HTTP API

```sh
aibreakd   # listens on 127.0.0.1:8080 by default
```

| Method | Path                              | Description            |
|--------|-----------------------------------|------------------------|
| POST   | `/v1/ideas`                       | Create an idea         |
| GET    | `/v1/ideas` (`?tag=`)             | List ideas             |
| GET    | `/v1/ideas/{id}`                  | Get an idea            |
| PATCH  | `/v1/ideas/{id}`                  | Update an idea         |
| DELETE | `/v1/ideas/{id}`                  | Delete an idea         |
| POST   | `/v1/ideas/{id}/evaluate`         | Evaluate an idea (body `{"summary": true}` to synthesize a verdict) |
| GET    | `/v1/ideas/{id}/evaluations`      | Evaluation history     |
| POST   | `/v1/ideas/{id}/feedback`         | Add feedback           |
| GET    | `/v1/ideas/{id}/feedback`         | List feedback          |
| DELETE | `/v1/ideas/{id}/feedback/{fid}`   | Delete feedback        |
| POST   | `/v1/ideas/{id}/resources`        | Add a resource         |
| GET    | `/v1/ideas/{id}/resources`        | List resources         |
| DELETE | `/v1/ideas/{id}/resources/{rid}`  | Remove a resource      |
| GET    | `/v1/personas`                    | List personas          |
| POST   | `/v1/personas`                    | Create a persona       |
| PATCH  | `/v1/personas/{id}`               | Update a custom persona (version auto-increments) |
| DELETE | `/v1/personas/{id}`               | Delete a persona       |

Evaluate responses include `spread` (persona disagreement: `consensus` 0–1,
`mixed` 2–3, `divided` 4–5), plus `summary` and `verdict` when synthesis was
requested; evaluation history returns them per run.

Errors use a consistent envelope:

```json
{ "error": { "code": "not_found", "message": "..." } }
```

Example:

```sh
curl -s http://127.0.0.1:8080/v1/personas
curl -s -X POST http://127.0.0.1:8080/v1/ideas -d '{"title":"My idea"}'
curl -s -X POST http://127.0.0.1:8080/v1/ideas/<id>/evaluate -d '{}'
curl -s -X POST http://127.0.0.1:8080/v1/ideas/<id>/evaluate -d '{"summary": true}'
```

## Development

The repo is spec-driven. See [`AGENTS.md`](AGENTS.md) for the workflow and
[`.opencode/skill/`](.opencode/skill/) for reusable agent skills.
