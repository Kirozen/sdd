-- v7 (V113): aggregated per-command usage counters. One row per
-- (project_id, command); each invocation UPSERTs, incrementing ok_count or
-- fail_count and refreshing last_seen. project_id uses the SENTINEL 0 for
-- out-of-project invocations (NOT NULL, never NULL) because SQLite treats NULLs
-- as distinct in a UNIQUE index -- a nullable key would defeat the UPSERT and
-- grow the out-of-project bucket unbounded (V113). No FK to project(id): the
-- sentinel 0 has no project row, and usage telemetry is deliberately decoupled
-- from the project registry so it survives even when no project resolves.
-- Bounded by construction (about #commands x #projects). Pure DDL, no PRAGMA or
-- data (sqlc parses this; V52 keeps migration a runtime concern). The leading
-- UNIQUE(project_id, ...) column already indexes the project_id scope filter, so
-- no separate index is needed (same reasoning as 005_indexes.sql).
CREATE TABLE command_usage (
	project_id INTEGER NOT NULL DEFAULT 0,
	command    TEXT NOT NULL,
	ok_count   INTEGER NOT NULL DEFAULT 0,
	fail_count INTEGER NOT NULL DEFAULT 0,
	last_seen  TEXT NOT NULL,
	UNIQUE (project_id, command)
);
