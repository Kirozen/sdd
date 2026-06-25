package main

import (
	"context"
	"database/sql"
	"testing"

	dbq "github.com/kirozen/sdd/db"
)

func ordOf(t *testing.T, db *sql.DB, pk int64) int64 {
	t.Helper()
	o, err := dbq.New(db).TaskOrdByID(context.Background(), pk)
	if err != nil {
		t.Fatalf("task ord: %v", err)
	}
	return o
}

// V74/V5: add-cite reuses insertCite, so cites resolve and FK-guard identically;
// a clean attach lands both an invariant and an interface cite.
func TestAddCitesAttaches(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fid, _ := addFeature(db, pid, "f")
	tpk, _ := addTask(db, pid, fid, "t", nil)
	addInvariant(db, pid, "inv")            // V1
	addInterface(db, pid, "cmd", "x", "sig") // I.x

	if err := addCites(db, pid, ordOf(t, db, tpk), []string{"V1", "I.x"}); err != nil {
		t.Fatalf("addCites: %v", err)
	}
	if got, _ := taskCites(db, tpk); got != "V1,I.x" {
		t.Errorf("cites = %q, want V1,I.x", got)
	}
}

// V74: the N cites share one tx — a single orphan rolls back ALL, so the valid
// V1 must not survive the rejected V99 (V5 rejects the orphan).
func TestAddCitesOrphanRollsBackAll(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fid, _ := addFeature(db, pid, "f")
	tpk, _ := addTask(db, pid, fid, "t", nil)
	addInvariant(db, pid, "inv") // V1

	mustFailInTx(t, db, func(tx *sql.Tx) error {
		return addCites(tx, pid, ordOf(t, db, tpk), []string{"V1", "V99"})
	})
	if got, _ := taskCites(db, tpk); got != "-" {
		t.Errorf("partial attach survived rollback: cites = %q, want -", got)
	}
}

// V74: re-citing an already-present cite is fail-loud (join-table PK), never a
// silent no-op.
func TestAddCitesDuplicateErrors(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fid, _ := addFeature(db, pid, "f")
	tpk, _ := addTask(db, pid, fid, "t", nil)
	addInvariant(db, pid, "inv")
	ord := ordOf(t, db, tpk)

	if err := addCites(db, pid, ord, []string{"V1"}); err != nil {
		t.Fatalf("first attach: %v", err)
	}
	if err := addCites(db, pid, ord, []string{"V1"}); err == nil {
		t.Error("duplicate cite accepted (V74: dup is fail-loud via PK)")
	}
}

// V74: an unknown task ordinal is an error, never a silent miss.
func TestAddCitesUnknownTask(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addInvariant(db, pid, "inv")
	if err := addCites(db, pid, 999, []string{"V1"}); err == nil {
		t.Error("add-cite on unknown task succeeded")
	}
}

// V20/V74: TaskPKByOrd scopes by project — citing project A's ordinal never
// touches project B's same-ordinal task.
func TestAddCitesScopedToProject(t *testing.T) {
	db := openTestDB(t)
	pidA := mustProject(t, db)
	pidB := mustProject(t, db)
	fa, _ := addFeature(db, pidA, "fa")
	ta, _ := addTask(db, pidA, fa, "ta", nil)
	fb, _ := addFeature(db, pidB, "fb")
	tb, _ := addTask(db, pidB, fb, "tb", nil)
	addInvariant(db, pidA, "ia") // A: V1
	addInvariant(db, pidB, "ib") // B: V1

	if err := addCites(db, pidA, ordOf(t, db, ta), []string{"V1"}); err != nil {
		t.Fatalf("attach A: %v", err)
	}
	if got, _ := taskCites(db, ta); got != "V1" {
		t.Errorf("project A task cites = %q, want V1", got)
	}
	if got, _ := taskCites(db, tb); got != "-" {
		t.Errorf("cite leaked into project B: %q, want -", got)
	}
}
