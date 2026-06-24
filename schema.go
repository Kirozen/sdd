package main

import (
	"database/sql"
	"fmt"
)

// userVersion anchors schema migrations (V9). Bumped per migration script.
const userVersion = 1

// schemaDDL is the full v1 schema: a durable layer that persists across
// features, an ephemeral feature layer wiped via feature cascade (V4), and
// typed join tables carrying real foreign keys (V5).
const schemaDDL = `
-- durable layer (no feature scope; persists across features)
CREATE TABLE invariant (
	id   INTEGER PRIMARY KEY,
	text TEXT NOT NULL
);
CREATE TABLE interface (
	id     INTEGER PRIMARY KEY,
	kind   TEXT NOT NULL,
	name   TEXT NOT NULL UNIQUE,
	sig    TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','deprecated'))
);
CREATE TABLE bug (
	id    INTEGER PRIMARY KEY,
	date  TEXT NOT NULL,
	cause TEXT NOT NULL
);
CREATE TABLE research (
	id      INTEGER PRIMARY KEY,
	topic   TEXT NOT NULL,
	finding TEXT NOT NULL,
	src     TEXT NOT NULL
);

-- feature layer (ephemeral; wiped per feature via feature cascade)
CREATE TABLE feature (
	id   INTEGER PRIMARY KEY,
	name TEXT NOT NULL
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

// applySchema creates all tables and stamps user_version (V9 migration anchor).
func applySchema(db *sql.DB) error {
	if _, err := db.Exec(schemaDDL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version=%d;", userVersion)); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}
	return nil
}
