package main

import (
	"strings"
	"testing"
)

// V38: list task --status / --feature narrow the result; a valid filter with no
// match yields no lines (not an error).
func TestListTasksFiltered(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f1, _ := addFeature(db, pid, "one")
	f2, _ := addFeature(db, pid, "two")
	a, _ := addTask(db, pid, f1, "a", nil) // T1 in f1
	addTask(db, pid, f2, "b", nil)         // T2 in f2
	setStatus(t, db, a, "x")

	// by status
	done, err := listTasksFiltered(db, pid, "x", 0)
	if err != nil {
		t.Fatalf("filter status: %v", err)
	}
	if len(done) != 1 || !strings.Contains(done[0], "T1|x|a") {
		t.Fatalf("status filter wrong: %v", done)
	}

	// by feature
	inF2, err := listTasksFiltered(db, pid, "", 2)
	if err != nil {
		t.Fatalf("filter feature: %v", err)
	}
	if len(inF2) != 1 || !strings.Contains(inF2[0], "T2|.|b") {
		t.Fatalf("feature filter wrong: %v", inF2)
	}

	// valid filter, zero matches → empty, no error
	none, err := listTasksFiltered(db, pid, "~", 0)
	if err != nil {
		t.Fatalf("empty filter errored: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("want no rows, got %v", none)
	}
}

// V38: filtering on a feature that does not exist errors.
func TestListTasksFilteredMissingFeature(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := listTasksFiltered(db, pid, "", 99); err == nil {
		t.Error("filtering on a missing feature should error")
	}
}

// V41: --status/--feature are rejected unless the kind is task; an invalid
// status value is rejected. Driven through the cobra command (the boundary that
// owns V41), asserting exit≠0 via the returned error.
func TestListFilterFlagGuards(t *testing.T) {
	run := func(args ...string) error {
		c := newListCmd()
		c.SetArgs(args)
		c.SilenceUsage = true
		c.SilenceErrors = true
		return c.Execute()
	}
	// --status without a kind → reject (before any db access)
	if err := run("--status", "x"); err == nil {
		t.Error("--status with no kind should error")
	}
	// --feature on a non-task kind → reject
	if err := run("invariant", "--feature", "1"); err == nil {
		t.Error("--feature on a non-task kind should error")
	}
	// invalid --status value → reject
	if err := run("task", "--status", "?"); err == nil {
		t.Error("invalid --status value should error")
	}
}

// V39: guide maps each feature's stage to the next skill; an empty project
// points at the grill.
func TestGuideReport(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)

	empty, err := guideReport(db, pid)
	if err != nil {
		t.Fatalf("guideReport: %v", err)
	}
	if len(empty) != 1 || !strings.Contains(empty[0], "sdd-grill") {
		t.Fatalf("empty project guide wrong: %v", empty)
	}

	// grilled feature (goal, no task) → sdd-spec
	g, _ := addFeature(db, pid, "g")
	addGoal(db, g, "goal")
	// building feature (mixed task statuses) → sdd-build
	b, _ := addFeature(db, pid, "b")
	bx, _ := addTask(db, pid, b, "done", nil)
	addTask(db, pid, b, "todo", nil)
	setStatus(t, db, bx, "x")

	lines, err := guideReport(db, pid)
	if err != nil {
		t.Fatalf("guideReport: %v", err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "F1 g  [grilled] → sdd-spec") {
		t.Errorf("grilled mapping wrong:\n%s", joined)
	}
	if !strings.Contains(joined, "F2 b  [building] → sdd-build") {
		t.Errorf("building mapping wrong:\n%s", joined)
	}
}
