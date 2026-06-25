package main

import (
	"strings"
	"testing"
)

// V42/V43: add-test links a named test to an invariant; cover marks an invariant
// with ≥1 test covered (names shown) and one with none uncovered (`!`).
func TestAddTestAndCover(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := addInvariant(db, pid, "first"); err != nil { // V1
		t.Fatalf("addInvariant: %v", err)
	}
	if _, err := addInvariant(db, pid, "second"); err != nil { // V2
		t.Fatalf("addInvariant: %v", err)
	}
	if err := addTest(db, pid, 1, "TestFoo"); err != nil {
		t.Fatalf("addTest: %v", err)
	}

	lines, err := coverReport(db, pid)
	if err != nil {
		t.Fatalf("coverReport: %v", err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "V1 ✓ TestFoo") {
		t.Errorf("V1 should be covered by TestFoo:\n%s", joined)
	}
	if !strings.Contains(joined, "! V2 aucun test") {
		t.Errorf("V2 should be flagged uncovered:\n%s", joined)
	}
	if !strings.Contains(joined, "gardés: 1/2 invariants") {
		t.Errorf("summary wrong:\n%s", joined)
	}
}

// V42: add-test on an invariant ordinal absent from the project is rejected (the
// declared-link analogue of the cite FK, V5).
func TestAddTestRejectsAbsentInvariant(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := addTest(db, pid, 99, "TestNope"); err == nil {
		t.Error("add-test on a missing invariant should error")
	}
}

// V42: re-adding the same (invariant, test) is a no-op — UNIQUE(invariant_id,
// name) + ON CONFLICT DO NOTHING, so no silent duplicate row.
func TestAddTestIdempotent(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := addInvariant(db, pid, "inv"); err != nil { // V1
		t.Fatalf("addInvariant: %v", err)
	}
	for i := range 3 {
		if err := addTest(db, pid, 1, "TestDup"); err != nil {
			t.Fatalf("addTest #%d: %v", i, err)
		}
	}
	if n := count(t, db, "test"); n != 1 {
		t.Errorf("test rows = %d, want 1 (idempotent)", n)
	}
}

// V44: test rows are not rendered into SPEC.md — adding one leaves the generated
// spec byte-identical, so the export/check drift contract (V6) is untouched.
func TestTestsAbsentFromSpec(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := addInvariant(db, pid, "inv"); err != nil { // V1
		t.Fatalf("addInvariant: %v", err)
	}
	before, err := renderSpec(db, pid)
	if err != nil {
		t.Fatalf("renderSpec: %v", err)
	}
	if err := addTest(db, pid, 1, "TestFoo"); err != nil {
		t.Fatalf("addTest: %v", err)
	}
	after, err := renderSpec(db, pid)
	if err != nil {
		t.Fatalf("renderSpec: %v", err)
	}
	if before != after {
		t.Error("a test row changed SPEC.md (V44: tests must not be rendered)")
	}
}
