package main

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// tableSQL returns the stored CREATE statement for a table, or "" if absent.
func tableSQL(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&ddl); err != nil {
		return ""
	}
	return ddl
}

func userVersionOf(t *testing.T, db *sql.DB) int {
	t.Helper()
	var uv int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		t.Fatalf("user_version: %v", err)
	}
	return uv
}

// openV2 builds a db frozen at the v2 schema (everything but unknown), the shape
// an existing global db has before this migration.
func openV2(t *testing.T) *sql.DB {
	t.Helper()
	db, err := open(filepath.Join(t.TempDir(), "spec.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(steps[0].sql); err != nil {
		t.Fatalf("base DDL: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version=2;"); err != nil {
		t.Fatalf("stamp v2: %v", err)
	}
	return db
}

// V36: a fresh db carries the unknown table and is stamped at the current version.
func TestFreshSchemaHasUnknownV3(t *testing.T) {
	db := openTestDB(t)
	if tableSQL(t, db, "unknown") == "" {
		t.Error("fresh db missing unknown table")
	}
	if got := userVersionOf(t, db); got != userVersion {
		t.Errorf("user_version = %d, want %d", got, userVersion)
	}
}

// V36: the migrated unknown table is byte-identical to the fresh one — proof the
// single-source DDL holds (no fresh-vs-migrated divergence).
func TestFreshEqualsMigratedSchema(t *testing.T) {
	fresh := openTestDB(t)
	freshSQL := tableSQL(t, fresh, "unknown")

	v2 := openV2(t)
	if err := migrate(v2, 2); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	migratedSQL := tableSQL(t, v2, "unknown")

	if freshSQL != migratedSQL {
		t.Errorf("schema divergence:\nfresh:    %q\nmigrated: %q", freshSQL, migratedSQL)
	}
	if got := userVersionOf(t, v2); got != userVersion {
		t.Errorf("post-migrate user_version = %d, want %d", got, userVersion)
	}
}

// V36: the migration is additive — every pre-existing row survives untouched.
func TestMigratePreservesRows(t *testing.T) {
	db := openV2(t)
	pid := mustProject(t, db)
	fid, err := addFeature(db, pid, "kept")
	if err != nil {
		t.Fatalf("addFeature: %v", err)
	}
	if _, err := addInvariant(db, pid, "kept invariant"); err != nil {
		t.Fatalf("addInvariant: %v", err)
	}

	if err := migrate(db, 2); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var name string
	if err := db.QueryRow(`SELECT name FROM feature WHERE id=?`, fid).Scan(&name); err != nil || name != "kept" {
		t.Errorf("feature lost after migrate: name=%q err=%v", name, err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM invariant`).Scan(&n); err != nil || n != 1 {
		t.Errorf("invariant lost after migrate: n=%d err=%v", n, err)
	}
}

// V45: migrate is a no-op at the current version and refuses unsupported ones
// (uv==1 pre-scope, uv>userVersion from a newer binary) rather than
// half-migrating. uv∈[2,userVersion) are valid chain starts (proven separately
// against a real v2 db, which a full db cannot stand in for).
func TestMigrateVersionGuards(t *testing.T) {
	db := openTestDB(t) // already at userVersion
	if err := migrate(db, userVersion); err != nil {
		t.Errorf("migrate at current version should be a no-op, got %v", err)
	}
	for _, bad := range []int{1, userVersion + 1, 99} {
		if err := migrate(db, bad); err == nil {
			t.Errorf("migrate(uv=%d) should error, got nil", bad)
		}
	}
}

// V45: the migrated test table is byte-identical to the fresh one — the v4
// single-source DDL holds, same as V36 proves for unknown.
func TestFreshEqualsMigratedSchemaV4(t *testing.T) {
	fresh := openTestDB(t)
	freshSQL := tableSQL(t, fresh, "test")

	v2 := openV2(t)
	if err := migrate(v2, 2); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	migratedSQL := tableSQL(t, v2, "test")

	if freshSQL == "" || freshSQL != migratedSQL {
		t.Errorf("schema divergence:\nfresh:    %q\nmigrated: %q", freshSQL, migratedSQL)
	}
}

// V45: a v2 db chains 2→3→4 in one migrate call — BOTH additive tables land and
// every pre-existing row survives. This is the loop's reason to exist: a single
// step (the old switch) would have stranded such a db at v3.
func TestMigrateChainV2toV4(t *testing.T) {
	db := openV2(t)
	pid := mustProject(t, db)
	fid, err := addFeature(db, pid, "kept")
	if err != nil {
		t.Fatalf("addFeature: %v", err)
	}

	if err := migrate(db, 2); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if tableSQL(t, db, "unknown") == "" {
		t.Error("unknown table missing after chained migrate (v3 step skipped)")
	}
	if tableSQL(t, db, "test") == "" {
		t.Error("test table missing after chained migrate (v4 step skipped)")
	}
	if got := userVersionOf(t, db); got != userVersion {
		t.Errorf("post-chain user_version = %d, want %d", got, userVersion)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM feature WHERE id=?`, fid).Scan(&name); err != nil || name != "kept" {
		t.Errorf("feature lost after chained migrate: name=%q err=%v", name, err)
	}
}

// V59/V60: the explicit userVersion anchor must equal the last derived step's
// version (prefix+1) — NOT merely the file count, so a numbering gap is caught
// here, not masked. A forgotten bump after adding a 00N file, or an orphan file
// without a bump, fails this. Guards the runtime mis-stamp V60 describes: a
// userVersion below the last applied step would re-exec a non-IF-NOT-EXISTS
// CREATE TABLE on the next open.
func TestUserVersionMatchesEmbeddedSteps(t *testing.T) {
	last := steps[len(steps)-1].version
	if userVersion != last {
		t.Errorf("userVersion = %d, but the last embedded step targets v%d — bump userVersion or check db/schema (V59/V60)", userVersion, last)
	}
}

// V59: 00N_*.sql maps to schema version N+1 (001_base = v2) over a contiguous,
// well-formed set.
func TestStepVersionsAcceptsContiguous(t *testing.T) {
	got := stepVersions([]string{"001_base.sql", "002_x.sql", "003_y.sql"})
	want := []int{2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("stepVersions len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("stepVersions[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

// V60: stepVersions enforces the filename contract it derives from, panicking on
// any violation — a malformed name, a numbering gap, a missing 001 base, or an
// empty set — so a bad file can never silently corrupt the derived chain.
func TestStepVersionsRejectsBadFilenames(t *testing.T) {
	cases := map[string][]string{
		"empty set":          {},
		"missing 001 base":   {"002_unknown.sql"},
		"gap in numbering":   {"001_base.sql", "002_x.sql", "004_y.sql"},
		"non-numeric prefix": {"001_base.sql", "00a_x.sql"},
		"too-short prefix":   {"001_base.sql", "02_x.sql"},
		"no prefix":          {"001_base.sql", "notes.sql"},
	}
	for name, names := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("stepVersions(%v) did not panic", names)
				}
			}()
			stepVersions(names)
		})
	}
}
