package sdd

import "testing"

// T101 / V96: rm-task hard-deletes a task and its cite rows cascade away.
func TestRmTaskCascadesCites(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	v1, _ := addInvariant(db, pid, "inv one")
	fid, _ := addFeature(db, pid, "f")
	tord, err := addTask(db, pid, fid, "cites V1", []string{"V1"})
	if err != nil {
		t.Fatalf("addTask: %v", err)
	}

	var citeRows int
	db.QueryRow(`SELECT count(*) FROM task_cites_inv`).Scan(&citeRows)
	if citeRows != 1 {
		t.Fatalf("setup: want 1 cite row, got %d", citeRows)
	}

	if err := rmTask(db, pid, tord); err != nil {
		t.Fatalf("rmTask: %v", err)
	}

	var taskRows, citeAfter int
	db.QueryRow(`SELECT count(*) FROM task WHERE ord=?`, tord).Scan(&taskRows)
	db.QueryRow(`SELECT count(*) FROM task_cites_inv`).Scan(&citeAfter)
	if taskRows != 0 {
		t.Errorf("task survived rm-task")
	}
	if citeAfter != 0 {
		t.Errorf("cite rows did not cascade: %d remain", citeAfter)
	}
	// the cited invariant itself is untouched
	var invRows int
	db.QueryRow(`SELECT count(*) FROM invariant WHERE ord=?`, v1).Scan(&invRows)
	if invRows != 1 {
		t.Errorf("rm-task deleted the cited invariant")
	}
}

// T101 / V17: rm-task on an unknown ordinal errors.
func TestRmTaskUnknown(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := rmTask(db, pid, 999); err == nil {
		t.Error("rm-task on unknown ord succeeded")
	}
}

// T101 / V97: removing one task does NOT renumber the survivors' ordinals.
func TestRmTaskKeepsSurvivorOrdinals(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fid, _ := addFeature(db, pid, "f")
	t1, _ := addTask(db, pid, fid, "a", nil)
	t2, _ := addTask(db, pid, fid, "b", nil)
	t3, _ := addTask(db, pid, fid, "c", nil)

	if err := rmTask(db, pid, t2); err != nil {
		t.Fatalf("rmTask: %v", err)
	}
	// t1 and t3 keep their ordinals; t2 is a permanent gap.
	for _, want := range []int64{t1, t3} {
		var n int
		db.QueryRow(`SELECT count(*) FROM task WHERE ord=?`, want).Scan(&n)
		if n != 1 {
			t.Errorf("survivor ord %d renumbered/lost", want)
		}
	}
	var gap int
	db.QueryRow(`SELECT count(*) FROM task WHERE ord=?`, t2).Scan(&gap)
	if gap != 0 {
		t.Errorf("ord %d should be a gap", t2)
	}
}

// T101 / V20: rm-task is project-scoped — a task ord from project B is invisible to A.
func TestRmTaskScoped(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)
	fb, _ := addFeature(db, b, "f")
	tb, _ := addTask(db, b, fb, "in B", nil)

	if err := rmTask(db, a, tb); err == nil {
		t.Error("rm-task reached across projects (V20)")
	}
	var n int
	db.QueryRow(`SELECT count(*) FROM task WHERE ord=?`, tb).Scan(&n)
	if n != 1 {
		t.Errorf("B's task was deleted from A's scope")
	}
}
