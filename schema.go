package main

import (
	"database/sql"
	"embed"
	"fmt"
)

// userVersion anchors schema migrations (V9). Bumped per migration script.
// v2 adds the global project scope (project table + project_id) and per-project
// display ordinals (ord). v3 adds the unknown table (parked grill questions).
// v4 adds the test table (invariant ↔ proving test, V42). v5 adds the gate
// table (durable per-feature review verdict, V46). v6 indexes every uncovered
// foreign-key child column (V58).
const userVersion = 6

// schemaFS embeds the DDL that is the SINGLE source for both the runtime migrator
// (applySchema/migrate, below) and sqlc codegen (V51): sqlc reads these same
// db/schema/*.sql files at `sqlc generate` time while the binary embeds them
// here, so the codegen schema and the runtime schema cannot diverge. sqlc never
// executes them — only this migrator does (V52). Files apply in filename order
// (001 base before the 002+ additive steps, which FK back into it).
//
//go:embed db/schema/*.sql
var schemaFS embed.FS

// mustDDL returns an embedded schema file's contents, panicking if it is missing
// — a build-time wiring error, caught the first time any db is opened.
func mustDDL(name string) string {
	b, err := schemaFS.ReadFile("db/schema/" + name)
	if err != nil {
		panic(fmt.Sprintf("embedded schema %s: %v", name, err))
	}
	return string(b)
}

// The base v2 schema and each additive step now live in db/schema/*.sql rather
// than inline string constants, but applySchema/migrate are unchanged: the same
// strings feed the fresh path and the migration path, so a fresh db is
// byte-identical to a migrated one by construction (V45). 001_base.sql is the
// full v2 schema (project scope V20, durable + ephemeral layers, typed FK joins
// V5); the 00N files are the v(N+1) additive steps.
var (
	schemaDDL  = mustDDL("001_base.sql")
	unknownDDL = mustDDL("002_unknown.sql")
	testDDL    = mustDDL("003_test.sql")
	gateDDL    = mustDDL("004_gate.sql")
	indexDDL   = mustDDL("005_indexes.sql")
)

// migrations maps each schema version > 2 to its additive DDL step. The same
// constants feed both applySchema (fresh) and migrate (upgrade), so a fresh db
// is byte-identical to a migrated one by construction (V45). To add a version,
// bump userVersion and add one entry — nothing else.
var migrations = map[int]string{
	3: unknownDDL,
	4: testDDL,
	5: gateDDL,
	6: indexDDL,
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
