package sdd

import (
	"strconv"
	"strings"
	"testing"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// T104 / V95: an uncited invariant retracts cleanly.
func TestRetractInvariantUncited(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	v1, _ := addInvariant(db, pid, "lonely invariant")

	msg, err := retractInvariant(db, pid, v1)
	if err != nil {
		t.Fatalf("retractInvariant: %v", err)
	}
	if !strings.Contains(msg, "retracted") {
		t.Errorf("msg = %q", msg)
	}
	var n int
	db.QueryRow(`SELECT count(*) FROM invariant WHERE ord=?`, v1).Scan(&n)
	if n != 0 {
		t.Errorf("invariant survived retract")
	}
}

// T104 / V95 / V5: a cited invariant is refused — citers listed, DELETE NOT run,
// and the raw FK error is never surfaced.
func TestRetractInvariantCitedRefused(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	v1, _ := addInvariant(db, pid, "cited invariant")
	fid, _ := addFeature(db, pid, "f")
	tord, _ := addTask(db, pid, fid, "cites it", []string{"V1"})

	_, err := retractInvariant(db, pid, v1)
	if err == nil {
		t.Fatal("retract of a cited invariant succeeded (V5)")
	}
	if strings.Contains(strings.ToUpper(err.Error()), "FOREIGN KEY") {
		t.Errorf("raw FK error leaked: %v", err)
	}
	if !strings.Contains(err.Error(), "T"+itoa(tord)) {
		t.Errorf("citer not listed in error: %v", err)
	}
	var n int
	db.QueryRow(`SELECT count(*) FROM invariant WHERE ord=?`, v1).Scan(&n)
	if n != 1 {
		t.Errorf("DELETE ran despite refusal")
	}
}

// T104 / V95 / V42: a tested invariant is announced (msg notes the tests) and the
// proving tests cascade away.
func TestRetractInvariantAnnouncesTests(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	v1, _ := addInvariant(db, pid, "tested invariant")
	if err := addTest(db, pid, v1, "TestProvesIt"); err != nil {
		t.Fatalf("addTest: %v", err)
	}

	msg, err := retractInvariant(db, pid, v1)
	if err != nil {
		t.Fatalf("retractInvariant: %v", err)
	}
	if !strings.Contains(msg, "test") {
		t.Errorf("msg did not announce tests: %q", msg)
	}
	var tests int
	db.QueryRow(`SELECT count(*) FROM test`).Scan(&tests)
	if tests != 0 {
		t.Errorf("proving tests did not cascade: %d remain", tests)
	}
}

// T104 / V97: retracting the highest ordinal reuses it on the next insert (the
// ord is not monotone); a middle ordinal leaves a permanent gap.
func TestRetractOrdinalReuseAndGap(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	a, _ := addInvariant(db, pid, "a") // ord 1
	b, _ := addInvariant(db, pid, "b") // ord 2 (max)

	// retract the max -> next add reuses its ordinal
	if _, err := retractInvariant(db, pid, b); err != nil {
		t.Fatalf("retract max: %v", err)
	}
	reused, _ := addInvariant(db, pid, "c")
	if reused != b {
		t.Errorf("max ordinal not reused: got %d, want %d", reused, b)
	}

	// now ords are {1(a), 2(c)}; retract the middle-most lowest and confirm a gap
	if _, err := retractInvariant(db, pid, a); err != nil {
		t.Fatalf("retract a: %v", err)
	}
	var hasA int
	db.QueryRow(`SELECT count(*) FROM invariant WHERE ord=? AND project_id=?`, a, pid).Scan(&hasA)
	if hasA != 0 {
		t.Errorf("ord %d should be gone", a)
	}
}

// T104 / V20: retract is project-scoped — an invariant ord from B is invisible to A.
func TestRetractInvariantScoped(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)
	vb, _ := addInvariant(db, b, "in B")

	if _, err := retractInvariant(db, a, vb); err == nil {
		t.Error("retract reached across projects (V20)")
	}
	var n int
	db.QueryRow(`SELECT count(*) FROM invariant WHERE ord=? AND project_id=?`, vb, b).Scan(&n)
	if n != 1 {
		t.Errorf("B's invariant deleted from A's scope")
	}
}

// T104 / V95: retract-interface refuses a cited interface (citers listed), and
// retracts an uncited one.
func TestRetractInterface(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := addInterface(db, pid, "cmd", "lonely", "sig"); err != nil {
		t.Fatalf("addInterface lonely: %v", err)
	}
	if _, err := addInterface(db, pid, "cmd", "used", "sig"); err != nil {
		t.Fatalf("addInterface used: %v", err)
	}
	fid, _ := addFeature(db, pid, "f")
	tord, _ := addTask(db, pid, fid, "cites iface", []string{"I.used"})

	// cited -> refused, citer listed, no delete
	_, err := retractInterface(db, pid, "used")
	if err == nil {
		t.Fatal("retract of a cited interface succeeded (V5)")
	}
	if !strings.Contains(err.Error(), "T"+itoa(tord)) {
		t.Errorf("citer not listed: %v", err)
	}
	var used int
	db.QueryRow(`SELECT count(*) FROM interface WHERE name='used'`).Scan(&used)
	if used != 1 {
		t.Errorf("cited interface deleted despite refusal")
	}

	// uncited -> retracted
	if _, err := retractInterface(db, pid, "lonely"); err != nil {
		t.Fatalf("retract lonely: %v", err)
	}
	var lonely int
	db.QueryRow(`SELECT count(*) FROM interface WHERE name='lonely'`).Scan(&lonely)
	if lonely != 0 {
		t.Errorf("uncited interface survived retract")
	}
}
