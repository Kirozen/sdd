package main

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
)

// userVersion anchors schema migrations (V9), kept explicit as the V9 anchor but
// guarded: it must equal the last derived step's version (V60). v2 adds the
// global project scope (project table + project_id) and per-project display
// ordinals (ord). v3 adds the unknown table (parked grill questions). v4 adds
// the test table (invariant ↔ proving test, V42). v5 adds the gate table
// (durable per-feature review verdict, V46). v6 indexes every uncovered
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

// ddlStep is one migration step: an embedded db/schema file's DDL paired with
// the schema version it produces (V59).
type ddlStep struct {
	version int
	sql     string
}

// schemaFileRe is the filename contract schemaSteps derives versions from: a
// 3-digit zero-padded prefix, then "_<name>.sql" (V60).
var schemaFileRe = regexp.MustCompile(`^(\d{3})_.*\.sql$`)

// stepVersions validates the filename contract (V60) over the schema file names
// — given in fs.ReadDir's sorted order — and returns each file's target version
// (prefix+1, so 001->v2). It panics on any violation, a build-time wiring error
// like mustDDL: every name matches schemaFileRe and the prefixes are contiguous
// from 001 (no gap, no missing base). That contiguity is what makes "last step
// == file count + 1" hold, so a forgotten/renamed file is caught, never
// silently mis-stamped. Pure (no embed access) so the contract is unit-testable.
func stepVersions(names []string) []int {
	if len(names) == 0 {
		panic("embedded db/schema: no DDL files (V60)")
	}
	versions := make([]int, len(names))
	for i, name := range names {
		m := schemaFileRe.FindStringSubmatch(name)
		if m == nil {
			panic(fmt.Sprintf("embedded db/schema/%s: name breaks the NNN_*.sql contract (V60)", name))
		}
		n, _ := strconv.Atoi(m[1]) // 3 digits, cannot fail
		if n != i+1 {
			panic(fmt.Sprintf("embedded db/schema: prefix gap at %s — expected %03d_*.sql (contiguous from 001, V60)", name, i+1))
		}
		versions[i] = n + 1
	}
	return versions
}

// schemaSteps derives the ordered migration chain from the embedded db/schema
// files rather than a hand-kept map (V59): fs.ReadDir returns them sorted by
// name, and file 00N_*.sql produces schema version N+1 (001_base = v2). The
// filename contract is enforced by stepVersions (V60).
func schemaSteps() []ddlStep {
	entries, err := fs.ReadDir(schemaFS, "db/schema")
	if err != nil {
		panic(fmt.Sprintf("read embedded db/schema: %v", err))
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	versions := stepVersions(names)
	steps := make([]ddlStep, len(names))
	for i, name := range names {
		steps[i] = ddlStep{version: versions[i], sql: mustDDL(name)}
	}
	return steps
}

// steps is the migration chain, derived once at startup (V59). applySchema and
// migrate both iterate it, so fresh == migrated by construction (V45).
var steps = schemaSteps()

// applySchema lays every step's DDL in order on a fresh db, then stamps the
// current version (V9 anchor). It iterates the same `steps` slice migrate uses,
// so a fresh db carries exactly what a fully-migrated one does (V45).
func applySchema(db *sql.DB) error {
	for _, s := range steps {
		if _, err := db.Exec(s.sql); err != nil {
			return fmt.Errorf("apply schema (v%d): %w", s.version, err)
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
// other uv applies every step past uv, stamping THAT step's literal version —
// never the moving userVersion constant, which would jump the version forward
// and skip later steps. So a v2 db chains 2→3→…→6.
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
	for _, s := range steps {
		if s.version <= uv {
			continue
		}
		if _, err := db.Exec(s.sql); err != nil {
			return fmt.Errorf("migrate v%d: %w", s.version, err)
		}
		if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version=%d;", s.version)); err != nil {
			return fmt.Errorf("migrate v%d (stamp): %w", s.version, err)
		}
	}
	return nil
}
