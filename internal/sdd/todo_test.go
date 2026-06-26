package sdd

import (
	"database/sql"
	"strings"
	"testing"
)

func mustSet(t *testing.T, db *sql.DB, pid, featOrd, ord int64, status string) {
	t.Helper()
	if err := setTaskStatus(db, pid, featOrd, ord, status); err != nil {
		t.Fatalf("set T%d @F%d %s: %v", ord, featOrd, status, err)
	}
}

// V68: todo emits only status != x tasks, ordered (feature ord, task ord); a
// fully-done feature contributes no rows.
func TestTodoSelectsUnfinishedOnly(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f1, _ := addFeature(db, pid, "f1")
	addTask(db, pid, f1, "todo task", nil) // T1 .
	addTask(db, pid, f1, "wip task", nil)  // T2
	addTask(db, pid, f1, "done task", nil) // T3
	mustSet(t, db, pid, 1, 2, "~")
	mustSet(t, db, pid, 1, 3, "x")
	// a fully-done feature must be absent entirely
	f2, _ := addFeature(db, pid, "f2")
	addTask(db, pid, f2, "f2 only task", nil) // T4
	mustSet(t, db, pid, 2, 1, "x")

	rows, err := todoRows(db, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (T1 ., T2 ~):\n%s", len(rows), strings.Join(rows, "\n"))
	}
	c0 := strings.Split(rows[0], "\t")
	if c0[0] != "F1" || c0[2] != "T1" || c0[3] != "." {
		t.Errorf("row0 = %v, want F1/T1/.", c0)
	}
	c1 := strings.Split(rows[1], "\t")
	if c1[2] != "T2" || c1[3] != "~" {
		t.Errorf("row1 = %v, want T2/~", c1)
	}
	joined := strings.Join(rows, "\n")
	if strings.Contains(joined, "done task") || strings.Contains(joined, "f2 only task") {
		t.Errorf("a done (x) task leaked:\n%s", joined)
	}
}

// V70: fixed 6-column TSV, cites reused from taskCites, text RAW (apostrophe and
// spaces intact, never pipe-escaped).
func TestTodoColumnsRawTextAndCites(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addInvariant(db, pid, "inv") // V1
	f, _ := addFeature(db, pid, "feat name")
	addTask(db, pid, f, "tache avec l'apostrophe | et pipe", []string{"V1"}) // T1

	rows, err := todoRows(db, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	c := strings.Split(rows[0], "\t")
	if len(c) != 6 {
		t.Fatalf("want 6 columns, got %d: %v", len(c), c)
	}
	if c[0] != "F1" || c[1] != "feat name" || c[2] != "T1" || c[3] != "." || c[4] != "V1" {
		t.Errorf("cols = %v", c)
	}
	if c[5] != "tache avec l'apostrophe | et pipe" {
		t.Errorf("text not raw/intact (V70): %q", c[5])
	}
}

// V20: todo is project-scoped — one project's pending tasks never leak into
// another's.
func TestTodoScopedToProject(t *testing.T) {
	db := openTestDB(t)
	pidA := mustProject(t, db)
	pidB := mustProject(t, db)
	fa, _ := addFeature(db, pidA, "fa")
	addTask(db, pidA, fa, "a task", nil)
	fb, _ := addFeature(db, pidB, "fb")
	addTask(db, pidB, fb, "b task", nil)

	rows, err := todoRows(db, pidA)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !strings.Contains(rows[0], "a task") {
		t.Fatalf("project A todo = %v, want only 'a task'", rows)
	}
	if strings.Contains(strings.Join(rows, "\n"), "b task") {
		t.Error("project B task leaked into A (V20)")
	}
}

// V70: zero pending tasks ⇒ no rows (the command then prints nothing, exit 0).
func TestTodoEmptyWhenNoPending(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f, _ := addFeature(db, pid, "f")
	addTask(db, pid, f, "t", nil) // T1
	mustSet(t, db, pid, 1, 1, "x")

	rows, err := todoRows(db, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("want empty, got %v", rows)
	}
}
