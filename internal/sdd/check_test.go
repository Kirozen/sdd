package sdd

import (
	"os"
	"path/filepath"
	"testing"
)

// V6: check passes right after an export.
func TestCheckClean(t *testing.T) {
	db := openTestDB(t)
	pid := seedSpec(t, db)
	path := filepath.Join(t.TempDir(), "SPEC.md")
	if err := exportSpec(db, pid, path); err != nil {
		t.Fatalf("export: %v", err)
	}
	if err := checkSpec(db, pid, path); err != nil {
		t.Errorf("check failed on fresh export: %v", err)
	}
}

// V6: check catches a hand-edit of the generated file.
func TestCheckDetectsHandEdit(t *testing.T) {
	db := openTestDB(t)
	pid := seedSpec(t, db)
	path := filepath.Join(t.TempDir(), "SPEC.md")
	if err := exportSpec(db, pid, path); err != nil {
		t.Fatalf("export: %v", err)
	}
	if err := os.WriteFile(path, []byte("hand-edited\n"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	if err := checkSpec(db, pid, path); err == nil {
		t.Error("check passed on hand-edited SPEC.md (drift undetected)")
	}
}

func TestCheckMissingFile(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := checkSpec(db, pid, filepath.Join(t.TempDir(), "absent.md")); err == nil {
		t.Error("check passed with no SPEC.md")
	}
}
