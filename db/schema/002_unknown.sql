-- v3: the unknown table (parked grill questions). Feature-scoped (cascade per
-- V4), with a per-project ordinal U<n> (V26) and an open→resolved lifecycle
-- (V35). Applied after 001 in filename order (FK -> feature).
CREATE TABLE unknown (
	id         INTEGER PRIMARY KEY,
	feature_id INTEGER NOT NULL REFERENCES feature(id) ON DELETE CASCADE,
	ord        INTEGER,
	text       TEXT NOT NULL,
	status     TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','resolved'))
);
