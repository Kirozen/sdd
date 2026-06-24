package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()
	if err := runInit(dir); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// db exists with schema applied
	db, err := open(filepath.Join(dir, "spec.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	var uv int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if uv != userVersion {
		t.Errorf("user_version = %d, want %d", uv, userVersion)
	}
	var name string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='invariant'`,
	).Scan(&name); err != nil {
		t.Errorf("schema not applied: %v", err)
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

// N2: init must not clobber an existing spec.db.
func TestInitRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	if err := runInit(dir); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if err := runInit(dir); err == nil {
		t.Fatal("second init succeeded; should refuse existing spec.db")
	}
}

func TestInitGitignoreAppend(t *testing.T) {
	dir := t.TempDir()
	gi := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gi, []byte("node_modules/\nspec.db\n"), 0o644); err != nil {
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
	if strings.Count(got, "spec.db\n") != 1 {
		t.Errorf("spec.db duplicated:\n%s", got)
	}
	if !strings.Contains(got, "SPEC.md") {
		t.Error("new entry SPEC.md not appended")
	}
}
