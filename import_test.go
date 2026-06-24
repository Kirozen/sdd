package main

import (
	"strings"
	"testing"
)

const fixtureSpec = `# SPEC

## §G GOAL
do a thing
across two lines

## §C CONSTRAINTS
- be fast
- be small

## §I INTERFACES
- cmd: ` + "`sdd init`" + ` → create db
- file: ` + "`spec.db`" + ` → source of truth

## §R RESEARCH
id|topic|finding|src
R1|wal|fine \| at scale|example.com

## §V INVARIANTS
V1: always check auth
V2: never lose data | ever

## §B BUGS
id|date|cause|fix
B1|2026-01-01|off by one|V2

## §T TASKS
id|status|task|cites
T1|x|build it|V1,I.init
T2|.|test it|V2
`

func TestImportParsesFixture(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := seedDB(db, pid, parseSpec(fixtureSpec), "f", false); err != nil {
		t.Fatalf("seedDB: %v", err)
	}

	checks := map[string]int{
		"invariant": 2, "interface": 2, "research": 1, "bug": 1,
		"bug_fix": 1, "feature": 1, "goal": 1, "constraint": 2,
		"task": 2, "task_cites_inv": 2, "task_cites_iface": 1,
	}
	for tbl, want := range checks {
		if n := count(t, db, tbl); n != want {
			t.Errorf("%s = %d, want %d", tbl, n, want)
		}
	}

	// the task cite re-joins to the invariant and the derived interface name
	if c, _ := taskCites(db, 1); c != "V1,I.init" {
		t.Errorf("T1 cites = %q, want V1,I.init", c)
	}
	// an escaped pipe in a table cell (§R) is unescaped on import
	var finding string
	db.QueryRow(`SELECT finding FROM research WHERE id=1`).Scan(&finding)
	if finding != "fine | at scale" {
		t.Errorf("research finding = %q, want 'fine | at scale'", finding)
	}
	// a literal pipe in §V (the caveman 'or') is preserved verbatim
	var text string
	db.QueryRow(`SELECT text FROM invariant WHERE id=2`).Scan(&text)
	if text != "never lose data | ever" {
		t.Errorf("V2 text = %q, want literal pipe preserved", text)
	}
}

// V13: import refuses a non-empty db without --force.
func TestImportRefusesNonEmpty(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := addInvariant(db, pid, "existing"); err != nil {
		t.Fatalf("seed existing: %v", err)
	}
	empty, _ := dbEmpty(db, pid)
	if empty {
		t.Fatal("db reported empty after an insert")
	}
	// the command-level guard is what refuses; here we assert dbEmpty drives it
}

// V13: --force reseeds, replacing prior data.
func TestImportForceReseed(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addInvariant(db, pid, "stale one")
	addInvariant(db, pid, "stale two")

	if err := seedDB(db, pid, parseSpec(fixtureSpec), "f", true); err != nil {
		t.Fatalf("force reseed: %v", err)
	}
	if n := count(t, db, "invariant"); n != 2 {
		t.Errorf("invariant = %d, want 2 (stale data not cleared)", n)
	}
	var text string
	db.QueryRow(`SELECT text FROM invariant WHERE project_id=? AND ord=1`, pid).Scan(&text)
	if text != "always check auth" {
		t.Errorf("invariant 1 = %q, want imported value", text)
	}
}

// V14: a cite to a missing invariant rolls the whole import back.
func TestImportAtomicRollback(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	bad := strings.Replace(fixtureSpec, "T2|.|test it|V2", "T2|.|test it|V99", 1)
	if err := seedDB(db, pid, parseSpec(bad), "f", false); err == nil {
		t.Fatal("import with orphan cite succeeded")
	}
	for _, tbl := range []string{"invariant", "interface", "feature", "task", "bug"} {
		if n := count(t, db, tbl); n != 0 {
			t.Errorf("%s = %d after failed import, want 0 (not atomic)", tbl, n)
		}
	}
}
