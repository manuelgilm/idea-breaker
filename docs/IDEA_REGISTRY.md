# Idea Registry — Design

## 1. Overview

The Idea Registry is a persistent, local store for ideas. An idea is created, listed, inspected, updated, and deleted via the CLI, and every idea evolves over time through immutable, numbered versions. Persistence is SQLite using a pure-Go driver, so the whole thing stays CGO-free and self-contained.

## 2. Design decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Storage | SQLite via `modernc.org/sqlite` | Pure Go, CGO-free (GoReleaser `CGO_ENABLED=0` unchanged), single `.db` file. |
| Package layout | New top-level `registry/` package | Persistence is a separate concern from `engine` (LLM orchestration); keeps `engine` free of storage. |
| Connection | `registry` owns `Open`/`Migrate` + a narrow `Store` interface | Centralized connection + migrations; split into its own package only if a second consumer appears. |
| Versioning | `title` = identity, `description` = versioned content | Renaming is not a content change; changing the substance creates a new snapshot. |
| CLI verbs | `break` = AI action, `create`/`update`/`rm` = registry | "Break" runs the AI breakdown; registering/managing an idea is a plain operation. |

## 3. Domain model

```
Idea    { ID, Title, CreatedAt, UpdatedAt }
Version { ID, IdeaID, Number, Description, CreatedAt }
```

- `Idea` is the identity: a title plus when it was created/updated.
- `Version` is a snapshot of the idea's content (its description) at a point in time.

## 4. Versioning rules

- Renaming the **title** updates the `Idea` in place and does **not** create a version.
- Changing the **description** creates a new `Version` with an incrementing `number` (`MAX(number)+1` for that idea).
- A `Version` is immutable once created; the version history is the record of how the idea evolved.

## 5. Storage

### Connection & migrations

- `registry.Open(path string) (*sql.DB, error)` opens the pool once (sets `PRAGMA foreign_keys = ON`).
- `registry.Migrate(ctx, db) error` applies an ordered, embedded list of SQL migrations idempotently, tracking them in a `schema_migrations` table.
- Consumers depend on a narrow `Store` interface, not raw `*sql.DB`; tests use an in-memory `:memory:` database.

### Schema

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  version    INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ideas (
  id         TEXT PRIMARY KEY,
  title      TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS versions (
  id          TEXT PRIMARY KEY,
  idea_id     TEXT NOT NULL REFERENCES ideas(id) ON DELETE CASCADE,
  number      INTEGER NOT NULL,
  description TEXT NOT NULL,
  created_at  TEXT NOT NULL
);
```

- Foreign keys are enforced (`PRAGMA foreign_keys = ON`); deleting an idea cascades to its versions.

### Location

- Default path: `os.UserConfigDir()/aibreak/aibreak.db`; override via env `AIBREAK_DB_PATH`.

## 6. CLI

Root `aibreak` becomes a parent command. Subcommands:

| Command | Description |
| --- | --- |
| `aibreak idea create --title T --description D` | Create an idea + version 1; prints the new idea ID. |
| `aibreak idea list` | List all ideas. |
| `aibreak idea show <id>` | Show an idea and its latest version. |
| `aibreak idea update <id> --title T` | Rename in place (no new version). |
| `aibreak idea update <id> --description D` | Create a new version. |
| `aibreak idea versions <id>` | List an idea's version history. |
| `aibreak idea rm <id>` | Delete an idea and its versions (cascade). |
| `aibreak idea break <id> [--version N] [--output Y] [--mock]` | Run the AI breakdown on a version (default: latest) and write the result to `--output`. |

Semantics:

- `break` is the only AI action; it does not register anything. Registering is `create`'s job.
- `update` requires at least one of `--title`/`--description`, else error.
- Global flags: `--db` (override DB path), `--json` (machine-readable output for `list`/`show`/`versions`).
- The DB is opened lazily — only `idea*` subcommands touch it.

## 7. CRUD spec

| Operation | Behavior |
| --- | --- |
| create | Insert `ideas` row + `versions` row (`number = 1`) in one transaction. |
| get / show | Return the idea and its latest version. |
| list | Return all ideas (optionally with latest version). |
| update title | Update `ideas.title` + `updated_at` in place. |
| update description | Insert a new `versions` row with `number = MAX(number)+1`, and bump `ideas.updated_at`. |
| delete | Delete the idea; versions cascade. |

## 8. Store interface

```go
type Store interface {
    CreateIdea(title, description string) (Idea, error)
    GetIdea(id string) (Idea, error)
    ListIdeas() ([]Idea, error)
    UpdateTitle(id, title string) (Idea, error)
    AddVersion(id, description string) (Version, error)
    ListVersions(ideaID string) ([]Version, error)
    LatestVersion(ideaID string) (Version, error)
    DeleteIdea(id string) error
}
```

## 9. Identifiers & timestamps

- IDs: UUID v4 generated from `crypto/rand` (no external dependency), canonical 36-char lowercase, stored as `TEXT`.
- Timestamps: RFC 3339 UTC — `time.Now().UTC().Format(time.RFC3339)`.

## 10. Testing

- `registry`: CRUD, version-number bump, cascade delete, `:memory:` store.

## 11. Dependencies

- Add `modernc.org/sqlite` (plus its transitive indirect deps) via `go get` + `go mod tidy`.
- No CI/release changes (pure Go, CGO-free).

## 12. Out of scope (separate docs)

- Evaluations and evaluators (running the AI breakdown and persisting its results).
- Personas/evaluators stored in the DB (prompt registry).
