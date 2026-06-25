package main

import (
	"database/sql"
	"fmt"
)

// userVersion anchors schema migrations (V9). Bumped per migration script.
// v2 adds the global project scope (project table + project_id) and per-project
// display ordinals (ord). v3 adds the unknown table (parked grill questions).
// v4 adds the test table (invariant ↔ proving test, V42).
const userVersion = 4

// schemaDDL is the full v2 schema: a global project layer (each row scoped to a
// project), a durable layer that persists across features, an ephemeral feature
// layer wiped via feature cascade (V4), and typed join tables carrying real
// foreign keys (V5). project_id (V20) scopes durable rows + features; ord (V26)
// is the per-(project,kind) display/cite ordinal.
const schemaDDL = `
-- global project scope: every durable row + feature belongs to one project (V20).
-- identity is dual-key: url (canonical, nullable) OR main worktree path (V21).
CREATE TABLE project (
	id   INTEGER PRIMARY KEY,
	url  TEXT UNIQUE,
	path TEXT NOT NULL UNIQUE
);

-- durable layer (persists across features; project-scoped)
CREATE TABLE invariant (
	id         INTEGER PRIMARY KEY,
	project_id INTEGER REFERENCES project(id) ON DELETE CASCADE,
	ord        INTEGER,
	text       TEXT NOT NULL
);
CREATE TABLE interface (
	id         INTEGER PRIMARY KEY,
	project_id INTEGER REFERENCES project(id) ON DELETE CASCADE,
	kind       TEXT NOT NULL,
	name       TEXT NOT NULL,
	sig        TEXT NOT NULL,
	status     TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','deprecated')),
	UNIQUE (project_id, name)
);
CREATE TABLE bug (
	id         INTEGER PRIMARY KEY,
	project_id INTEGER REFERENCES project(id) ON DELETE CASCADE,
	ord        INTEGER,
	date       TEXT NOT NULL,
	cause      TEXT NOT NULL
);
CREATE TABLE research (
	id         INTEGER PRIMARY KEY,
	project_id INTEGER REFERENCES project(id) ON DELETE CASCADE,
	ord        INTEGER,
	topic      TEXT NOT NULL,
	finding    TEXT NOT NULL,
	src        TEXT NOT NULL
);

-- feature layer (ephemeral; wiped per feature via feature cascade; project-scoped)
CREATE TABLE feature (
	id         INTEGER PRIMARY KEY,
	project_id INTEGER REFERENCES project(id) ON DELETE CASCADE,
	ord        INTEGER,
	name       TEXT NOT NULL
);
CREATE TABLE goal (
	id         INTEGER PRIMARY KEY,
	feature_id INTEGER NOT NULL REFERENCES feature(id) ON DELETE CASCADE,
	text       TEXT NOT NULL
);
CREATE TABLE "constraint" (
	id         INTEGER PRIMARY KEY,
	feature_id INTEGER NOT NULL REFERENCES feature(id) ON DELETE CASCADE,
	text       TEXT NOT NULL
);
CREATE TABLE task (
	id         INTEGER PRIMARY KEY,
	feature_id INTEGER NOT NULL REFERENCES feature(id) ON DELETE CASCADE,
	ord        INTEGER,
	text       TEXT NOT NULL,
	status     TEXT NOT NULL DEFAULT '.' CHECK (status IN ('.','~','x'))
);

-- typed join tables (real FK -> V5). cited durable rows are protected:
-- deleting an invariant/interface still referenced fails (NO ACTION).
CREATE TABLE task_cites_inv (
	task_id INTEGER NOT NULL REFERENCES task(id) ON DELETE CASCADE,
	inv_id  INTEGER NOT NULL REFERENCES invariant(id),
	PRIMARY KEY (task_id, inv_id)
);
CREATE TABLE task_cites_iface (
	task_id  INTEGER NOT NULL REFERENCES task(id) ON DELETE CASCADE,
	iface_id INTEGER NOT NULL REFERENCES interface(id),
	PRIMARY KEY (task_id, iface_id)
);
CREATE TABLE bug_fix (
	bug_id INTEGER NOT NULL REFERENCES bug(id) ON DELETE CASCADE,
	inv_id INTEGER NOT NULL REFERENCES invariant(id),
	PRIMARY KEY (bug_id, inv_id)
);
`

// unknownDDL is the v3 addition, kept as its own constant so the fresh path
// (applySchema) and the migration path (migrateV3) create byte-identical tables
// from a single source (V36). unknown is feature-scoped (cascade per V4), with a
// per-project ordinal U<n> (V26) and an open→resolved lifecycle (V35).
const unknownDDL = `
CREATE TABLE unknown (
	id         INTEGER PRIMARY KEY,
	feature_id INTEGER NOT NULL REFERENCES feature(id) ON DELETE CASCADE,
	ord        INTEGER,
	text       TEXT NOT NULL,
	status     TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','resolved'))
);
`

// testDDL is the v4 addition (V42): a durable link from an invariant to a test
// that proves it. UNIQUE(invariant_id, name) makes re-adding the same pair a
// no-op rather than a silent duplicate; one test name may guard several
// invariants (distinct rows). Kept as its own constant so the fresh path
// (applySchema) and the migration path build byte-identical tables (V45).
const testDDL = `
CREATE TABLE test (
	id           INTEGER PRIMARY KEY,
	invariant_id INTEGER NOT NULL REFERENCES invariant(id) ON DELETE CASCADE,
	name         TEXT NOT NULL,
	UNIQUE (invariant_id, name)
);
`

// migrations maps each schema version > 2 to its additive DDL step. The same
// constants feed both applySchema (fresh) and migrate (upgrade), so a fresh db
// is byte-identical to a migrated one by construction (V45). To add a version,
// bump userVersion and add one entry — nothing else.
var migrations = map[int]string{
	3: unknownDDL,
	4: testDDL,
}

// applySchema creates all tables and stamps user_version (V9 migration anchor).
// It lays the base schema then every additive step in version order, so a fresh
// db carries exactly what a fully-migrated one does (V45).
func applySchema(db *sql.DB) error {
	if _, err := db.Exec(schemaDDL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	for v := 3; v <= userVersion; v++ {
		if _, err := db.Exec(migrations[v]); err != nil {
			return fmt.Errorf("apply schema (v%d): %w", v, err)
		}
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version=%d;", userVersion)); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}
	return nil
}

// migrate brings a db at user_version uv up to the current one (V45). uv==0 is a
// fresh file → full schema. uv==1 (pre-project-scope, only the one-off SQL ever
// migrated it) and uv>userVersion (a db written by a newer binary) are
// unsupported and error rather than risk a blind half-migration (cf V23). Any
// other uv loops uv+1..userVersion, applying each step's DDL and stamping THAT
// literal version — never the moving userVersion constant, which would jump the
// version forward and skip later steps. So a v2 db chains 2→3→4.
func migrate(db *sql.DB, uv int) error {
	if uv == 0 {
		return applySchema(db)
	}
	if uv == userVersion {
		return nil
	}
	if uv == 1 || uv > userVersion {
		return fmt.Errorf("unsupported db user_version %d (want 0, or 2..%d)", uv, userVersion)
	}
	for v := uv + 1; v <= userVersion; v++ {
		ddl, ok := migrations[v]
		if !ok {
			return fmt.Errorf("no migration step to v%d", v)
		}
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("migrate v%d: %w", v, err)
		}
		if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version=%d;", v)); err != nil {
			return fmt.Errorf("migrate v%d (stamp): %w", v, err)
		}
	}
	return nil
}
