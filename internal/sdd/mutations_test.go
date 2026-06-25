package sdd

import (
	"database/sql"
	"fmt"
	"testing"
)

// mustFailInTx runs fn inside a tx and asserts it errors, then rolls back. Post
// T67 the mutation cores no longer own a transaction (V71): atomicity is the
// caller's tx, so a core that fails mid-way persists nothing only because the
// caller (runMutation/apply — here this helper) rolls back.
func mustFailInTx(t *testing.T, db *sql.DB, fn func(tx *sql.Tx) error) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if err := fn(tx); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAddTaskWithCites(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	invID, err := addInvariant(db, pid, "inv")
	if err != nil {
		t.Fatalf("addInvariant: %v", err)
	}
	if _, err := addInterface(db, pid, "cmd", "init", "create db"); err != nil {
		t.Fatalf("addInterface: %v", err)
	}
	fid, err := addFeature(db, pid, "f")
	if err != nil {
		t.Fatalf("addFeature: %v", err)
	}

	tid, err := addTask(db, pid, fid, "t", []string{fmt.Sprintf("V%d", invID), "I.init"})
	if err != nil {
		t.Fatalf("addTask: %v", err)
	}
	if n := count(t, db, "task_cites_inv"); n != 1 {
		t.Errorf("task_cites_inv = %d, want 1", n)
	}
	if n := count(t, db, "task_cites_iface"); n != 1 {
		t.Errorf("task_cites_iface = %d, want 1", n)
	}
	cites, _ := taskCites(db, tid)
	if cites != fmt.Sprintf("V%d,I.init", invID) {
		t.Errorf("cites = %q", cites)
	}
}

// V2 + V5: an orphan invariant cite rolls back the whole task insert.
func TestAddTaskOrphanInvRollback(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fid, _ := addFeature(db, pid, "f")
	mustFailInTx(t, db, func(tx *sql.Tx) error {
		_, e := addTask(tx, pid, fid, "t", []string{"V999"})
		return e
	})
	if n := count(t, db, "task"); n != 0 {
		t.Errorf("task rows after failed add = %d, want 0 (not atomic)", n)
	}
}

// V2: an unknown interface cite rolls back the task insert.
func TestAddTaskUnknownInterfaceRollback(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fid, _ := addFeature(db, pid, "f")
	mustFailInTx(t, db, func(tx *sql.Tx) error {
		_, e := addTask(tx, pid, fid, "t", []string{"I.nope"})
		return e
	})
	if n := count(t, db, "task"); n != 0 {
		t.Errorf("task rows after failed add = %d, want 0 (not atomic)", n)
	}
}

// V15: the `-` empty sentinel and blanks are not treated as refs (B1).
func TestSplitRefsDropsSentinel(t *testing.T) {
	for _, in := range []string{"-", "", " , - , "} {
		if got := splitRefs(in); len(got) != 0 {
			t.Errorf("splitRefs(%q) = %v, want empty", in, got)
		}
	}
	if got := splitRefs("V1,-,I.init"); len(got) != 2 {
		t.Errorf("splitRefs dropped a real ref: %v", got)
	}
}

func TestAddBugFix(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	invID, _ := addInvariant(db, pid, "inv")
	if _, err := addBug(db, pid, "2026-06-24", "cause", []string{fmt.Sprintf("V%d", invID)}); err != nil {
		t.Fatalf("addBug: %v", err)
	}
	if n := count(t, db, "bug_fix"); n != 1 {
		t.Errorf("bug_fix = %d, want 1", n)
	}
}

// V2 + V5: an orphan fix invariant rolls back the bug insert.
func TestAddBugOrphanFixRollback(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	mustFailInTx(t, db, func(tx *sql.Tx) error {
		_, e := addBug(tx, pid, "2026-06-24", "cause", []string{"V999"})
		return e
	})
	if n := count(t, db, "bug"); n != 0 {
		t.Errorf("bug rows after failed add = %d, want 0 (not atomic)", n)
	}
}
