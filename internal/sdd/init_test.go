package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := gitRepo(t)
	if err := runInit(dir); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// the repo is registered as a project in the global store
	db, err := openGlobalDB()
	if err != nil {
		t.Fatalf("openGlobalDB: %v", err)
	}
	defer db.Close()
	if n := count(t, db, "project"); n != 1 {
		t.Errorf("project rows after init = %d, want 1", n)
	}

	// the worktree-root SPEC.md is written
	if _, err := os.Stat(filepath.Join(dir, "SPEC.md")); err != nil {
		t.Errorf("SPEC.md not written: %v", err)
	}

	// .gitignore has every entry
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	for _, e := range gitignoreEntries {
		if !strings.Contains(string(data), e) {
			t.Errorf(".gitignore missing %q", e)
		}
	}
}

// init is idempotent: re-running on a registered repo is a no-op find, not a
// second project.
func TestInitIdempotent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := gitRepo(t)
	if err := runInit(dir); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if err := runInit(dir); err != nil {
		t.Fatalf("second init: %v", err)
	}
	db, err := openGlobalDB()
	if err != nil {
		t.Fatalf("openGlobalDB: %v", err)
	}
	defer db.Close()
	if n := count(t, db, "project"); n != 1 {
		t.Errorf("project rows after re-init = %d, want 1 (idempotent)", n)
	}
}

func TestInitGitignoreAppend(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := gitRepo(t)
	gi := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gi, []byte("node_modules/\nSPEC.md\n"), 0o644); err != nil {
		t.Fatalf("seed .gitignore: %v", err)
	}

	if err := runInit(dir); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(gi)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "node_modules/") {
		t.Error("existing .gitignore content lost")
	}
	if strings.Count(got, "SPEC.md\n") != 1 {
		t.Errorf("SPEC.md duplicated:\n%s", got)
	}
}
