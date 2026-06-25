package sdd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupBinary(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addInvariant(db, pid, "x")
	addInvariant(db, pid, "y")

	dest := filepath.Join(t.TempDir(), "bak.db")
	if err := backupBinary(db, dest); err != nil {
		t.Fatalf("backupBinary: %v", err)
	}

	bak, err := open(dest)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer bak.Close()
	if n := count(t, bak, "invariant"); n != 2 {
		t.Errorf("backup invariant count = %d, want 2", n)
	}
}

// VACUUM INTO refuses to overwrite, so a backup never clobbers.
func TestBackupRefusesExisting(t *testing.T) {
	db := openTestDB(t)
	dest := filepath.Join(t.TempDir(), "bak.db")
	if err := backupBinary(db, dest); err != nil {
		t.Fatalf("first backup: %v", err)
	}
	if err := backupBinary(db, dest); err == nil {
		t.Error("second backup overwrote existing file")
	}
}

// the SQL dump round-trips: re-importing it into a fresh db reproduces rows.
func TestDumpSQLRoundTrip(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	invID, _ := addInvariant(db, pid, "has 'quotes' inside")
	addInterface(db, pid, "cmd", "init", "create db")
	fid, _ := addFeature(db, pid, "f")
	addTask(db, pid, fid, "t", []string{})
	_ = invID

	dump := filepath.Join(t.TempDir(), "dump.sql")
	if err := dumpSQL(db, dump); err != nil {
		t.Fatalf("dumpSQL: %v", err)
	}

	fresh, err := open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatalf("open fresh: %v", err)
	}
	defer fresh.Close()
	sqlBytes, err := os.ReadFile(dump)
	if err != nil {
		t.Fatalf("read dump: %v", err)
	}
	if _, err := fresh.Exec(string(sqlBytes)); err != nil {
		t.Fatalf("replay dump: %v", err)
	}
	if n := count(t, fresh, "invariant"); n != 1 {
		t.Errorf("invariant after replay = %d, want 1", n)
	}
	if n := count(t, fresh, "task"); n != 1 {
		t.Errorf("task after replay = %d, want 1", n)
	}
}
