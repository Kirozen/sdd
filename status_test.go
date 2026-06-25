package main

import (
	"fmt"
	"strings"
	"testing"
)

// I.status + V19: status reports per-feature task counts and flags every task
// that cites a deprecated interface.
func TestStatusCountsAndDeprecatedWarn(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := addInterface(db, pid, "cmd", "olditer", "sig"); err != nil {
		t.Fatalf("addInterface: %v", err)
	}
	fid, err := addFeature(db, pid, "f")
	if err != nil {
		t.Fatalf("addFeature: %v", err)
	}
	tid, err := addTask(db, pid, fid, "uses old", []string{"I.olditer"})
	if err != nil {
		t.Fatalf("addTask: %v", err)
	}

	// counts: the one task sits at the default status "."
	lines, err := statusReport(db, pid)
	if err != nil {
		t.Fatalf("statusReport: %v", err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, fmt.Sprintf("F%d f  x:0 ~:0 .:1", fid)) {
		t.Errorf("counts wrong:\n%s", joined)
	}
	if strings.Contains(joined, "deprecated") {
		t.Errorf("warned on an active interface:\n%s", joined)
	}

	// deprecate the cited interface → the warning appears (V19)
	if err := deprecateInterface(db, pid, "olditer"); err != nil {
		t.Fatalf("deprecateInterface: %v", err)
	}
	lines, err = statusReport(db, pid)
	if err != nil {
		t.Fatalf("statusReport after deprecate: %v", err)
	}
	joined = strings.Join(lines, "\n")
	if !strings.Contains(joined, fmt.Sprintf("! T%d cites deprecated I.olditer", tid)) {
		t.Errorf("missing deprecated-cite warning (V19):\n%s", joined)
	}
}

// V32: featureStage infers the pipeline stage from feature data, first-match.
func TestFeatureStageInference(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)

	// seeded: feature with no goal/constraint/task.
	seeded, _ := addFeature(db, pid, "seeded")
	// grilled: a goal, no task.
	grilled, _ := addFeature(db, pid, "grilled")
	addGoal(db, grilled, "g")
	// specced: tasks all "."
	specced, _ := addFeature(db, pid, "specced")
	addTask(db, pid, specced, "t1", nil)
	addTask(db, pid, specced, "t2", nil)
	// building: a mix of x and "." (no ~), exercising V32's "neither" branch.
	building, _ := addFeature(db, pid, "building")
	bx, _ := addTask(db, pid, building, "done", nil)
	addTask(db, pid, building, "todo", nil)
	setStatus(t, db, bx, "x")
	// built: every task x.
	built, _ := addFeature(db, pid, "built")
	dx, _ := addTask(db, pid, built, "fin", nil)
	setStatus(t, db, dx, "x")

	cases := []struct {
		pk   int64
		want string
	}{
		{seeded, "seeded"},
		{grilled, "grilled"},
		{specced, "specced"},
		{building, "building"},
		{built, "built"},
	}
	for _, c := range cases {
		got, err := featureStage(db, c.pk)
		if err != nil {
			t.Fatalf("featureStage(%s): %v", c.want, err)
		}
		if got != c.want {
			t.Errorf("stage = %q, want %q", got, c.want)
		}
	}
}

// V34: the status line carries the stage appended after the {x,~,.} counts, so
// the counts substring (and the V19 warning lines) stay byte-stable.
func TestStatusLineCarriesStage(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f, _ := addFeature(db, pid, "f")
	addTask(db, pid, f, "t", nil) // one "." task → specced

	lines, err := statusReport(db, pid)
	if err != nil {
		t.Fatalf("statusReport: %v", err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, fmt.Sprintf("F%d f  x:0 ~:0 .:1 [specced]", f)) {
		t.Errorf("stage not appended after counts:\n%s", joined)
	}
}
