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
	if _, err := db.Exec(schemaDDL); err != nil {
		t.Fatalf("schemaDDL: %v", err)
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

// V36: migrate is a no-op at the current version and refuses unsupported ones
// rather than half-migrating.
func TestMigrateVersionGuards(t *testing.T) {
	db := openTestDB(t) // already at userVersion
	if err := migrate(db, userVersion); err != nil {
		t.Errorf("migrate at current version should be a no-op, got %v", err)
	}
	for _, bad := range []int{1, 4, 99} {
		if err := migrate(db, bad); err == nil {
			t.Errorf("migrate(uv=%d) should error, got nil", bad)
		}
	}
}
