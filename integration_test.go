package main

import (
	"fmt"
	"os"
	"testing"
)

// TestLifecycle runs a full init→populate→export→check→wipe flow in an isolated
// directory, exercising V4 (wipe keeps durable), V5 (FK), V6 (check), V7
// (deterministic export) through the same paths the CLI uses.
func TestLifecycle(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := runInit("."); err != nil {
		t.Fatalf("init: %v", err)
	}
	db, err := openProjectDB()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	invID, err := addInvariant(db, "auth check before handler")
	if err != nil {
		t.Fatalf("addInvariant: %v", err)
	}
	if _, err := addInterface(db, "cmd", "init", "create db"); err != nil {
		t.Fatalf("addInterface: %v", err)
	}
	fid, _ := addFeature(db, "auth")
	if err := addGoal(db, fid, "login JWT"); err != nil {
		t.Fatalf("addGoal: %v", err)
	}
	if err := addConstraint(db, fid, "expire 15min"); err != nil {
		t.Fatalf("addConstraint: %v", err)
	}
	if _, err := addTask(db, fid, "impl mw", []string{fmt.Sprintf("V%d", invID), "I.init"}); err != nil {
		t.Fatalf("addTask: %v", err)
	}

	// export then check is clean (V6)
	if err := exportSpec(db, specPath); err != nil {
		t.Fatalf("export: %v", err)
	}
	if err := checkSpec(db, specPath); err != nil {
		t.Errorf("check after export: %v", err)
	}

	// V7: a second export is byte-identical
	before, _ := os.ReadFile(specPath)
	if err := exportSpec(db, specPath); err != nil {
		t.Fatalf("re-export: %v", err)
	}
	after, _ := os.ReadFile(specPath)
	if string(before) != string(after) {
		t.Error("re-export not byte-identical (V7)")
	}

	// V5: orphan cite rejected, nothing partial
	if _, err := addTask(db, fid, "bad", []string{"V999"}); err == nil {
		t.Error("orphan cite accepted (V5)")
	}

	// V4: wipe the feature, durable invariant + interface survive
	if err := wipeFeature(db, fid); err != nil {
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
	if err := exportSpec(db, specPath); err != nil {
		t.Fatalf("export after wipe: %v", err)
	}
	if err := checkSpec(db, specPath); err != nil {
		t.Errorf("check after wipe: %v", err)
	}
}
