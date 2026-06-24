package main

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// openTestDB returns a fresh schema-applied db on a temp file (not :memory:,
// so the connection pool shares one db).
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := open(filepath.Join(t.TempDir(), "spec.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := applySchema(db); err != nil {
		t.Fatalf("applySchema: %v", err)
	}
	return db
}

func mustFeature(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO feature(name) VALUES(?)`, name)
	if err != nil {
		t.Fatalf("insert feature: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func mustTask(t *testing.T, db *sql.DB, fid int64, text string) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO task(feature_id, text) VALUES(?, ?)`, fid, text)
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM "` + table + `"`).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestSchemaApplies(t *testing.T) {
	db := openTestDB(t)

	want := []string{
		"invariant", "interface", "bug", "research",
		"feature", "goal", "constraint", "task",
		"task_cites_inv", "task_cites_iface", "bug_fix",
	}
	for _, name := range want {
		var got string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name,
		).Scan(&got)
		if err != nil {
			t.Errorf("table %q missing: %v", name, err)
		}
	}

	var uv int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if uv != userVersion {
		t.Errorf("user_version = %d, want %d", uv, userVersion)
	}
}

// V5: a task citing a non-existent invariant is rejected by the FK.
func TestFK_OrphanCiteRejected(t *testing.T) {
	db := openTestDB(t)
	fid := mustFeature(t, db, "f")
	tid := mustTask(t, db, fid, "t")

	if _, err := db.Exec(
		`INSERT INTO task_cites_inv(task_id, inv_id) VALUES(?, 999)`, tid,
	); err == nil {
		t.Fatal("orphan cite accepted; FK not enforced (V5 violated)")
	}
}

// V4: wiping a feature deletes only its rows; durable rows survive.
func TestWipeCascade(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec(`INSERT INTO invariant(id, text) VALUES(1, 'durable')`); err != nil {
		t.Fatalf("insert invariant: %v", err)
	}
	fid := mustFeature(t, db, "f")
	if _, err := db.Exec(`INSERT INTO goal(feature_id, text) VALUES(?, 'g')`, fid); err != nil {
		t.Fatalf("insert goal: %v", err)
	}
	mustTask(t, db, fid, "t")

	if _, err := db.Exec(`DELETE FROM feature WHERE id=?`, fid); err != nil {
		t.Fatalf("delete feature: %v", err)
	}

	if n := count(t, db, "goal"); n != 0 {
		t.Errorf("goal rows after wipe = %d, want 0", n)
	}
	if n := count(t, db, "task"); n != 0 {
		t.Errorf("task rows after wipe = %d, want 0", n)
	}
	if n := count(t, db, "invariant"); n != 1 {
		t.Errorf("invariant rows after wipe = %d, want 1 (durable must survive)", n)
	}
}

// V11: interface.status is constrained; default is active.
func TestInterfaceStatusCheck(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec(
		`INSERT INTO interface(kind, name, sig, status) VALUES('cmd','x','x','bogus')`,
	); err == nil {
		t.Fatal("bogus interface status accepted (V11 violated)")
	}
	if _, err := db.Exec(
		`INSERT INTO interface(kind, name, sig) VALUES('cmd','y','y')`,
	); err != nil {
		t.Fatalf("insert interface: %v", err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM interface WHERE name='y'`).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "active" {
		t.Errorf("default status = %q, want active", status)
	}
}

// V10: task.status is constrained; default is todo.
func TestTaskStatusCheck(t *testing.T) {
	db := openTestDB(t)
	fid := mustFeature(t, db, "f")
	if _, err := db.Exec(
		`INSERT INTO task(feature_id, text, status) VALUES(?, 't', 'q')`, fid,
	); err == nil {
		t.Fatal("bogus task status accepted (V10 violated)")
	}
	tid := mustTask(t, db, fid, "t")
	var status string
	if err := db.QueryRow(`SELECT status FROM task WHERE id=?`, tid).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "." {
		t.Errorf("default status = %q, want .", status)
	}
}
