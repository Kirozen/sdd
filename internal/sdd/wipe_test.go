package sdd

import "testing"

// V4: wipe-feature deletes only that feature's rows; durable rows survive.
func TestWipeFeatureCmd(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := addInvariant(db, pid, "durable"); err != nil {
		t.Fatalf("addInvariant: %v", err)
	}
	fid, _ := addFeature(db, pid, "f")
	if err := addGoal(db, fid, "g"); err != nil {
		t.Fatalf("addGoal: %v", err)
	}
	if _, err := addTask(db, pid, fid, "t", nil); err != nil {
		t.Fatalf("addTask: %v", err)
	}

	if err := wipeFeature(db, pid, fid); err != nil {
		t.Fatalf("wipeFeature: %v", err)
	}

	if n := count(t, db, "feature"); n != 0 {
		t.Errorf("feature = %d, want 0", n)
	}
	if n := count(t, db, "goal"); n != 0 {
		t.Errorf("goal = %d, want 0", n)
	}
	if n := count(t, db, "task"); n != 0 {
		t.Errorf("task = %d, want 0", n)
	}
	if n := count(t, db, "invariant"); n != 1 {
		t.Errorf("invariant = %d, want 1 (durable must survive)", n)
	}
}

func TestWipeUnknownFeature(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := wipeFeature(db, pid, 999); err == nil {
		t.Fatal("wiping unknown feature succeeded")
	}
}

// other features are untouched by a scoped wipe.
func TestWipeLeavesOtherFeatures(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	keep, _ := addFeature(db, pid, "keep")
	addGoal(db, keep, "g")
	drop, _ := addFeature(db, pid, "drop")
	addGoal(db, drop, "g")

	if err := wipeFeature(db, pid, drop); err != nil {
		t.Fatalf("wipeFeature: %v", err)
	}
	if n := count(t, db, "feature"); n != 1 {
		t.Errorf("feature = %d, want 1", n)
	}
	if n := count(t, db, "goal"); n != 1 {
		t.Errorf("goal = %d, want 1 (other feature's goal must survive)", n)
	}
}
