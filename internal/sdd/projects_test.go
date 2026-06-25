package sdd

import (
	"strings"
	"testing"
)

// T95 / V92: projects lists every project in the store with correct counts,
// across project boundaries (the one non-scoped read).
func TestProjectsCountsMultiProject(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)

	// A: 2 invariants, 1 feature, 2 tasks (1 done -> 1 open).
	addInvariant(db, a, "a1")
	addInvariant(db, a, "a2")
	fa, _ := addFeature(db, a, "fa")
	addTask(db, a, fa, "t-open", nil)
	tdone, _ := addTask(db, a, fa, "t-done", nil)
	setTaskStatus(db, a, tdone, "x")
	// B: empty (LEFT-JOIN style 0/0/0).

	lines, err := projectLines(db, 0)
	if err != nil {
		t.Fatalf("projectLines: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("want 2 project lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "invariants:2") || !strings.Contains(lines[0], "features:1") || !strings.Contains(lines[0], "open-tasks:1") {
		t.Errorf("A counts wrong: %q", lines[0])
	}
	if !strings.Contains(lines[1], "invariants:0") || !strings.Contains(lines[1], "open-tasks:0") {
		t.Errorf("empty project B not 0/0/0: %q", lines[1])
	}
	_ = b
}

// T95 / V92: the current project (and only it) is marked with '*'; others are not.
func TestProjectsCurrentMarker(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)

	lines, err := projectLines(db, b)
	if err != nil {
		t.Fatalf("projectLines: %v", err)
	}
	var marked int
	for _, l := range lines {
		if strings.HasPrefix(l, "* ") {
			marked++
		}
	}
	if marked != 1 {
		t.Errorf("want exactly 1 marked project, got %d", marked)
	}
	// with no current project (0), nothing is marked.
	lines0, _ := projectLines(db, 0)
	for _, l := range lines0 {
		if strings.HasPrefix(l, "* ") {
			t.Errorf("unresolved current still marked a project: %q", l)
		}
	}
	_ = a
}
