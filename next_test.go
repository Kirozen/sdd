package main

import (
	"database/sql"
	"strings"
	"testing"
)

// setStatus flips a task's status by global pk — a test-local shortcut around
// setTaskStatus (which keys on the per-project ordinal).
func setStatus(t *testing.T, db *sql.DB, taskPK int64, status string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE task SET status=? WHERE id=?`, status, taskPK); err != nil {
		t.Fatalf("setStatus: %v", err)
	}
}

// V30: next picks the first feature (by ord) owning a non-`x` task.
func TestNextActionableFirstFeature(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f1, _ := addFeature(db, pid, "one")
	f2, _ := addFeature(db, pid, "two")
	addTask(db, pid, f1, "f1 task", nil)
	addTask(db, pid, f2, "f2 task", nil)

	r, err := nextActionable(db, pid)
	if err != nil {
		t.Fatalf("nextActionable: %v", err)
	}
	if r == nil || r.featName != "one" {
		t.Fatalf("want feature one, got %+v", r)
	}
}

// V30: within a feature, `~` (wip) wins over a lower-ord `.` (todo).
func TestNextActionableWipBeforeTodo(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f, _ := addFeature(db, pid, "f")
	todo, _ := addTask(db, pid, f, "todo first", nil) // ord 1, status .
	wip, _ := addTask(db, pid, f, "wip second", nil)  // ord 2
	setStatus(t, db, wip, "~")

	r, err := nextActionable(db, pid)
	if err != nil {
		t.Fatalf("nextActionable: %v", err)
	}
	if r == nil || r.taskLine != "T2|~|wip second|-" {
		t.Fatalf("want wip task T2, got %+v", r)
	}
	_ = todo
}

// V30: a fully-done feature (all `x`) is skipped; next moves to the next feature.
func TestNextActionableSkipsDoneFeature(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f1, _ := addFeature(db, pid, "done")
	f2, _ := addFeature(db, pid, "live")
	d, _ := addTask(db, pid, f1, "finished", nil)
	setStatus(t, db, d, "x")
	addTask(db, pid, f2, "todo", nil)

	r, err := nextActionable(db, pid)
	if err != nil {
		t.Fatalf("nextActionable: %v", err)
	}
	if r == nil || r.featName != "live" {
		t.Fatalf("want feature live, got %+v", r)
	}
}

// V31 selection half: when every task is `x`, next has nothing actionable → nil.
func TestNextActionableAllDone(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f, _ := addFeature(db, pid, "f")
	tk, _ := addTask(db, pid, f, "only", nil)
	setStatus(t, db, tk, "x")

	r, err := nextActionable(db, pid)
	if err != nil {
		t.Fatalf("nextActionable: %v", err)
	}
	if r != nil {
		t.Fatalf("want nil (all done), got %+v", r)
	}
}

// V33: next emits every goal of the chosen feature (0..N, ORDER BY id), and 0
// goals is not an error.
func TestNextActionableGoals(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)

	// 0 goals: no crash, empty slice.
	f0, _ := addFeature(db, pid, "nogoals")
	addTask(db, pid, f0, "t", nil)
	r, err := nextActionable(db, pid)
	if err != nil {
		t.Fatalf("nextActionable: %v", err)
	}
	if r == nil || len(r.goals) != 0 {
		t.Fatalf("want 0 goals, got %+v", r)
	}

	// N goals on an earlier feature win selection and all are returned in order.
	db2 := openTestDB(t)
	pid2 := mustProject(t, db2)
	f, _ := addFeature(db2, pid2, "f")
	addGoal(db2, f, "goal A")
	addGoal(db2, f, "goal B")
	addTask(db2, pid2, f, "t", nil)
	r2, err := nextActionable(db2, pid2)
	if err != nil {
		t.Fatalf("nextActionable: %v", err)
	}
	if r2 == nil || len(r2.goals) != 2 || r2.goals[0] != "goal A" || r2.goals[1] != "goal B" {
		t.Fatalf("want [goal A, goal B], got %+v", r2)
	}
}

// V18: cites resolve to their full caveman line via the shared render source.
func TestNextActionableResolvesCites(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addInvariant(db, pid, "some invariant")
	f, _ := addFeature(db, pid, "f")
	addTask(db, pid, f, "cites V1", []string{"V1"})

	r, err := nextActionable(db, pid)
	if err != nil {
		t.Fatalf("nextActionable: %v", err)
	}
	if r == nil || len(r.citeLines) != 1 || r.citeLines[0] != "V1: some invariant" {
		t.Fatalf("want resolved cite line, got %+v", r)
	}
}

// V31: all tasks done and every feature built → "project up to date".
func TestNextOutputAllDone(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f, _ := addFeature(db, pid, "f")
	tk, _ := addTask(db, pid, f, "only", nil)
	setStatus(t, db, tk, "x")

	out, err := nextOutput(db, pid)
	if err != nil {
		t.Fatalf("nextOutput: %v", err)
	}
	if len(out) != 1 || !strings.Contains(out[0], "up to date") {
		t.Fatalf("want up-to-date, got %v", out)
	}
}

// V31 (the BLOCK-1 case): a built feature plus a grilled-but-unspecced one must
// NOT report "up to date" — next points at the feature awaiting spec.
func TestNextOutputAwaitsSpec(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	built, _ := addFeature(db, pid, "built")
	bx, _ := addTask(db, pid, built, "done", nil)
	setStatus(t, db, bx, "x")
	grilled, _ := addFeature(db, pid, "grilled")
	addGoal(db, grilled, "g")

	out, err := nextOutput(db, pid)
	if err != nil {
		t.Fatalf("nextOutput: %v", err)
	}
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, "up to date") {
		t.Fatalf("falsely reported up-to-date with a grilled feature:\n%s", joined)
	}
	if !strings.Contains(joined, "F2 grilled awaits spec") {
		t.Fatalf("want awaits-spec hint, got:\n%s", joined)
	}
}

// V31: a project with no feature points at the grill.
func TestNextOutputNoFeature(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	out, err := nextOutput(db, pid)
	if err != nil {
		t.Fatalf("nextOutput: %v", err)
	}
	if len(out) != 1 || !strings.Contains(out[0], "sdd-grill") {
		t.Fatalf("want grill hint, got %v", out)
	}
}

// V18: the actionable block renders the feature, its goal, the task's spec line
// and each resolved cite.
func TestRenderNextShape(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addInvariant(db, pid, "inv one")
	f, _ := addFeature(db, pid, "feat")
	addGoal(db, f, "the goal")
	addTask(db, pid, f, "do it", []string{"V1"})

	out, err := nextOutput(db, pid)
	if err != nil {
		t.Fatalf("nextOutput: %v", err)
	}
	joined := strings.Join(out, "\n")
	for _, want := range []string{"F1 feat", "§G the goal", "T1|.|do it|V1", "V1: inv one"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}
