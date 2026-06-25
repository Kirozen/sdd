-- v2 base schema (F8: single source for runtime migrator + sqlc, V51).
-- A global project layer (each row scoped to a project), a durable layer that
-- persists across features, an ephemeral feature layer wiped via feature cascade
-- (V4), and typed join tables carrying real foreign keys (V5). project_id (V20)
-- scopes durable rows + features; ord (V26) is the per-(project,kind) display/cite
-- ordinal. Pure DDL — no PRAGMA, no data (sqlc parses this; V52 keeps migration
-- a runtime concern).

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
