package sdd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestLifecycle runs a full populate→export→check→wipe flow against a single
// project in the db, exercising V4 (wipe keeps durable), V5 (FK), V6 (check), V7
// (deterministic export) through the same core functions the CLI uses.
func TestLifecycle(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	spec := filepath.Join(t.TempDir(), "SPEC.md")

	invOrd, err := addInvariant(db, pid, "auth check before handler")
	if err != nil {
		t.Fatalf("addInvariant: %v", err)
	}
	if _, err := addInterface(db, pid, "cmd", "init", "create db"); err != nil {
		t.Fatalf("addInterface: %v", err)
	}
	fpk, _ := addFeature(db, pid, "auth")
	if err := addGoal(db, fpk, "login JWT"); err != nil {
		t.Fatalf("addGoal: %v", err)
	}
	if err := addConstraint(db, fpk, "expire 15min"); err != nil {
		t.Fatalf("addConstraint: %v", err)
	}
	if _, err := addTask(db, pid, fpk, "impl mw", []string{fmt.Sprintf("V%d", invOrd), "I.init"}); err != nil {
		t.Fatalf("addTask: %v", err)
	}

	// export then check is clean (V6)
	if err := exportSpec(db, pid, spec); err != nil {
		t.Fatalf("export: %v", err)
	}
	if err := checkSpec(db, pid, spec); err != nil {
		t.Errorf("check after export: %v", err)
	}

	// V7: a second export is byte-identical
	before, _ := os.ReadFile(spec)
	if err := exportSpec(db, pid, spec); err != nil {
		t.Fatalf("re-export: %v", err)
	}
	after, _ := os.ReadFile(spec)
	if string(before) != string(after) {
		t.Error("re-export not byte-identical (V7)")
	}

	// V5: orphan cite rejected, nothing partial
	if _, err := addTask(db, pid, fpk, "bad", []string{"V999"}); err == nil {
		t.Error("orphan cite accepted (V5)")
	}

	// V4: wipe the feature (ord 1 in a fresh project), durable rows survive
	if err := wipeFeature(db, pid, 1); err != nil {
		t.Fatalf("wipe: %v", err)
	}
	if n := count(t, db, "task"); n != 0 {
		t.Errorf("task after wipe = %d, want 0", n)
	}
	if n := count(t, db, "invariant"); n != 1 {
		t.Errorf("invariant after wipe = %d, want 1 (durable)", n)
	}
	if n := count(t, db, "interface"); n != 1 {
		t.Errorf("interface after wipe = %d, want 1 (durable)", n)
	}

	// export + check still consistent after wipe (V6)
	if err := exportSpec(db, pid, spec); err != nil {
		t.Fatalf("export after wipe: %v", err)
	}
	if err := checkSpec(db, pid, spec); err != nil {
		t.Errorf("check after wipe: %v", err)
	}
}
