package sqlite

import "aibreak/internal/domain"

// migrations holds ordered, versioned schema migrations. Append only: the
// runner assigns version = index+1 and skips recorded versions, so existing
// entries must never move.
var migrations = []string{
	// v1: initial schema.
	`
CREATE TABLE ideas (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    tags       TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE personas (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    system_prompt TEXT NOT NULL,
    weight        REAL NOT NULL DEFAULT 1.0,
    version       TEXT NOT NULL,
    created_at    TEXT NOT NULL
);

CREATE TABLE runs (
    run_id    TEXT PRIMARY KEY,
    idea_id   TEXT NOT NULL REFERENCES ideas(id) ON DELETE CASCADE,
    total     REAL NOT NULL,
    requested INTEGER NOT NULL,
    responded INTEGER NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE evaluations (
    run_id          TEXT NOT NULL REFERENCES runs(run_id) ON DELETE CASCADE,
    idea_id         TEXT NOT NULL REFERENCES ideas(id) ON DELETE CASCADE,
    persona_id      TEXT NOT NULL,
    persona_version TEXT NOT NULL,
    weight          REAL NOT NULL,
    status          TEXT NOT NULL,
    score           INTEGER,
    rationale       TEXT NOT NULL DEFAULT '',
    error           TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    PRIMARY KEY (run_id, persona_id)
);

CREATE TABLE feedbacks (
    id         TEXT PRIMARY KEY,
    idea_id    TEXT NOT NULL REFERENCES ideas(id) ON DELETE CASCADE,
    author     TEXT NOT NULL,
    score      INTEGER NOT NULL,
    rationale  TEXT NOT NULL DEFAULT '',
    aspect     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE resources (
    id         TEXT PRIMARY KEY,
    idea_id    TEXT NOT NULL REFERENCES ideas(id) ON DELETE CASCADE,
    url        TEXT NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    kind       TEXT NOT NULL DEFAULT 'other',
    note       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
`,
	// v2: synthesized summary/verdict on runs.
	`
ALTER TABLE runs ADD COLUMN summary TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN verdict TEXT NOT NULL DEFAULT '';
`,
	// v3: disagreement spread on runs.
	`
ALTER TABLE runs ADD COLUMN spread REAL NOT NULL DEFAULT 0;
`,
	// v4: persona versions become auto-incrementing integers (1, 2, 3, ...).
	// Rebuild the version columns from TEXT (semver) to INTEGER, casting any
	// existing values.
	`
CREATE TABLE personas_v4 (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    system_prompt TEXT NOT NULL,
    weight        REAL NOT NULL DEFAULT 1.0,
    version       INTEGER NOT NULL,
    created_at    TEXT NOT NULL
);
INSERT INTO personas_v4 (id, name, system_prompt, weight, version, created_at)
    SELECT id, name, system_prompt, weight, CAST(version AS INTEGER), created_at FROM personas;
DROP TABLE personas;
ALTER TABLE personas_v4 RENAME TO personas;

CREATE TABLE evaluations_v4 (
    run_id          TEXT NOT NULL REFERENCES runs(run_id) ON DELETE CASCADE,
    idea_id         TEXT NOT NULL REFERENCES ideas(id) ON DELETE CASCADE,
    persona_id      TEXT NOT NULL,
    persona_version INTEGER NOT NULL,
    weight          REAL NOT NULL,
    status          TEXT NOT NULL,
    score           INTEGER,
    rationale       TEXT NOT NULL DEFAULT '',
    error           TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    PRIMARY KEY (run_id, persona_id)
);
INSERT INTO evaluations_v4 (run_id, idea_id, persona_id, persona_version, weight, status, score, rationale, error, created_at)
    SELECT run_id, idea_id, persona_id, CAST(persona_version AS INTEGER), weight, status, score, rationale, error, created_at FROM evaluations;
DROP TABLE evaluations;
ALTER TABLE evaluations_v4 RENAME TO evaluations;
`,
	// v5: provider API key metadata (the secret lives in the OS keyring).
	`
CREATE TABLE provider_keys (
    id         TEXT PRIMARY KEY,
    provider   TEXT NOT NULL,
    label      TEXT NOT NULL DEFAULT '',
    hint       TEXT NOT NULL DEFAULT '',
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);
`,
	// v6: key-value application settings (active provider, per-provider models,
	// prices, etc.).
	`
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`,
}

// builtinPersonas are seeded on first run and are not deletable.
var builtinPersonas = []domain.Persona{
	{
		ID:      "skeptic",
		Name:    "Skeptic",
		Weight:  1.0,
		Version: 1,
		SystemPrompt: "You are a skeptical critic evaluating a product idea. Identify its " +
			"flaws, risks, and failure modes; be specific and direct. Rate the idea's " +
			"robustness on a 0-5 scale: 0 = fatally flawed, 1 = major flaws, " +
			"2 = significant flaws, 3 = mixed, 4 = mostly sound, 5 = nearly bulletproof.",
	},
	{
		ID:      "optimist",
		Name:    "Optimist",
		Weight:  1.0,
		Version: 1,
		SystemPrompt: "You are an optimist evaluating a product idea. Identify its upside, " +
			"strengths, and opportunities; be specific and direct. Rate the idea's " +
			"potential on a 0-5 scale: 0 = no upside, 1 = marginal, 2 = modest, " +
			"3 = solid, 4 = strong, 5 = exceptional.",
	},
	{
		ID:      "engineer",
		Name:    "Engineer",
		Weight:  1.0,
		Version: 1,
		SystemPrompt: "You are a pragmatic engineer evaluating a product idea. Assess its " +
			"technical feasibility and implementation effort; be specific and direct. " +
			"Rate the idea's feasibility on a 0-5 scale: 0 = infeasible, 1 = very hard, " +
			"2 = hard, 3 = moderate, 4 = easy, 5 = trivial with a clear path.",
	},
}

var builtinIDs = func() map[string]bool {
	m := make(map[string]bool, len(builtinPersonas))
	for _, p := range builtinPersonas {
		m[p.ID] = true
	}
	return m
}()
