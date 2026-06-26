package sdd

import "testing"

func TestSetTaskStatus(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fid, _ := addFeature(db, pid, "f")
	tid, _ := addTask(db, pid, fid, "t", nil)

	if err := setTaskStatus(db, pid, 1, tid, "~"); err != nil {
		t.Fatalf("setTaskStatus: %v", err)
	}
	var got string
	db.QueryRow(`SELECT status FROM task WHERE id=?`, tid).Scan(&got)
	if got != "~" {
		t.Errorf("status = %q, want ~", got)
	}
}

// V10: a status outside the enum is rejected by the CHECK constraint.
func TestSetTaskBadStatus(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fid, _ := addFeature(db, pid, "f")
	tid, _ := addTask(db, pid, fid, "t", nil)
	if err := setTaskStatus(db, pid, 1, tid, "q"); err == nil {
		t.Error("bogus status accepted (V10)")
	}
}

func TestSetTaskUnknown(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := setTaskStatus(db, pid, 1, 999, "x"); err == nil {
		t.Error("set-task on unknown id succeeded")
	}
}
