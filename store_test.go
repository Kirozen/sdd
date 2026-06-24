package main

import (
	"path/filepath"
	"testing"
)

// V22: the store dir is $XDG_CONFIG_HOME/sdd when XDG is an absolute path, else
// ~/.config/sdd; a relative XDG_CONFIG_HOME is ignored (XDG spec).
func TestGlobalDBDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/cfg")
	if got, want := globalDBDir(), "/custom/cfg/sdd"; got != want {
		t.Errorf("with XDG set: dir = %q, want %q", got, want)
	}

	t.Setenv("XDG_CONFIG_HOME", "relative/path") // not absolute → ignored
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got, want := globalDBDir(), filepath.Join(home, ".config", "sdd"); got != want {
		t.Errorf("with relative XDG: dir = %q, want %q (default)", got, want)
	}
}

// V22: openGlobalDB creates the dir + db + schema on demand, and is idempotent.
func TestOpenGlobalDBCreatesAndIsIdempotent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	db, err := openGlobalDB()
	if err != nil {
		t.Fatalf("openGlobalDB: %v", err)
	}
	// schema applied: the project table exists and version is stamped
	var name string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='project'`).Scan(&name); err != nil {
		t.Errorf("project table missing after create: %v", err)
	}
	var uv int
	db.QueryRow(`PRAGMA user_version`).Scan(&uv)
	if uv != userVersion {
		t.Errorf("user_version = %d, want %d", uv, userVersion)
	}
	if _, err := db.Exec(`INSERT INTO project(path) VALUES('/repo')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	db.Close()

	// reopening must not wipe or re-apply schema — the row survives
	db2, err := openGlobalDB()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()
	if n := count(t, db2, "project"); n != 1 {
		t.Errorf("project rows after reopen = %d, want 1 (not idempotent)", n)
	}
}
