package sdd

import (
	"strings"
	"testing"
)

// V20 + V26: two projects in one db are fully isolated — each numbers its rows
// from 1, reads return only the project's own rows, and a cite binds within the
// citing project, never across.
func TestCrossProjectIsolation(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)

	oa, err := addInvariant(db, a, "alpha invariant")
	if err != nil {
		t.Fatalf("addInvariant a: %v", err)
	}
	ob, err := addInvariant(db, b, "beta invariant")
	if err != nil {
		t.Fatalf("addInvariant b: %v", err)
	}
	if oa != 1 || ob != 1 {
		t.Fatalf("ordinals = %d,%d, want both 1 (per-project numbering)", oa, ob)
	}

	// a read in A sees only A's row
	la, err := listKind(db, a, "invariant")
	if err != nil {
		t.Fatalf("list a: %v", err)
	}
	if len(la) != 1 || !strings.Contains(la[0], "alpha") {
		t.Errorf("project A invariants = %v, want only alpha", la)
	}
	// V1 resolves to each project's own first invariant
	sb, err := showRef(db, b, "V1")
	if err != nil {
		t.Fatalf("show b V1: %v", err)
	}
	if !strings.Contains(sb, "beta") {
		t.Errorf("show V1 in B = %q, want beta", sb)
	}

	// a task in A citing V1 must bind A's invariant, never B's (no leak)
	fa := mustFeature(t, db, a, "f")
	tid, err := addTask(db, a, fa, "uses V1", []string{"V1"})
	if err != nil {
		t.Fatalf("addTask a: %v", err)
	}
	var boundText string
	if err := db.QueryRow(`SELECT i.text FROM task_cites_inv j JOIN invariant i ON i.id=j.inv_id WHERE j.task_id=?`, tid).Scan(&boundText); err != nil {
		t.Fatalf("read bound invariant: %v", err)
	}
	if boundText != "alpha invariant" {
		t.Errorf("cite bound to %q, want A's alpha (cross-project leak)", boundText)
	}
}

// V13 + V20: importing into one project leaves other projects untouched and does
// not contaminate the imported project with their rows.
func TestImportScopedToProject(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	if _, err := addInvariant(db, a, "pre-existing in A"); err != nil {
		t.Fatalf("seed A: %v", err)
	}

	b := mustProject(t, db)
	if err := seedDB(db, b, parseSpec(fixtureSpec), "f", false); err != nil {
		t.Fatalf("import into B: %v", err)
	}

	// B holds exactly the fixture's invariants (2), not A's
	lb, err := listKind(db, b, "invariant")
	if err != nil {
		t.Fatalf("list B: %v", err)
	}
	if len(lb) != 2 {
		t.Errorf("project B invariants = %d, want 2 (fixture only)", len(lb))
	}
	for _, l := range lb {
		if strings.Contains(l, "pre-existing") {
			t.Errorf("A's row leaked into B: %q", l)
		}
	}
	// A still has just its one row, untouched by B's import
	la, err := listKind(db, a, "invariant")
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	if len(la) != 1 {
		t.Errorf("project A invariants = %d, want 1 (untouched by B import)", len(la))
	}
}
