package sdd

import (
	"strings"
	"testing"
)

// V35: add-unknown records an open unknown with a per-project ordinal;
// resolve-unknown flips it to resolved without deleting it.
func TestUnknownLifecycle(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f, _ := addFeature(db, pid, "f")

	ord, err := addUnknown(db, pid, f, "is the API stable?")
	if err != nil {
		t.Fatalf("addUnknown: %v", err)
	}
	if ord != 1 {
		t.Fatalf("first unknown ord = %d, want 1", ord)
	}

	var status string
	db.QueryRow(`SELECT status FROM unknown WHERE ord=?`, ord).Scan(&status)
	if status != "open" {
		t.Errorf("new unknown status = %q, want open", status)
	}

	if err := resolveUnknown(db, pid, int64(ord)); err != nil {
		t.Fatalf("resolveUnknown: %v", err)
	}
	var n int
	db.QueryRow(`SELECT count(*) FROM unknown WHERE ord=? AND status='resolved'`, ord).Scan(&n)
	if n != 1 {
		t.Errorf("unknown not resolved (kept), count=%d", n)
	}
}

// V35: resolve-unknown on a missing ordinal errors.
func TestResolveUnknownMissing(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := resolveUnknown(db, pid, 99); err == nil {
		t.Error("resolveUnknown on missing ordinal should error")
	}
}

// V20/V26 (NOTE-6): an unknown in one project is not resolvable from another.
func TestResolveUnknownCrossProjectIsolation(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)
	fa, _ := addFeature(db, a, "fa")
	orda, _ := addUnknown(db, a, fa, "A's question")

	// Project B must not see or resolve A's U<orda>.
	if err := resolveUnknown(db, b, int64(orda)); err == nil {
		t.Error("project B resolved project A's unknown (V20 leak)")
	}
	// And A's unknown is still open.
	var status string
	db.QueryRow(`SELECT status FROM unknown WHERE ord=? AND feature_id=?`, orda, fa).Scan(&status)
	if status != "open" {
		t.Errorf("A's unknown status = %q, want open", status)
	}
}

// V18: list unknown renders open and resolved rows via the shared fmt helper.
func TestListUnknown(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f, _ := addFeature(db, pid, "f")
	addUnknown(db, pid, f, "open one")
	r2, _ := addUnknown(db, pid, f, "resolved one")
	resolveUnknown(db, pid, int64(r2))

	lines, err := listKind(db, pid, "unknown")
	if err != nil {
		t.Fatalf("listKind unknown: %v", err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "U1|open|open one") || !strings.Contains(joined, "U2|resolved|resolved one") {
		t.Fatalf("list unknown wrong:\n%s", joined)
	}
}

// V37: status warns once per feature carrying open unknowns; resolving silences it.
func TestStatusWarnsOpenUnknowns(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f, _ := addFeature(db, pid, "f")
	addTask(db, pid, f, "t", nil)
	u1, _ := addUnknown(db, pid, f, "q1")
	addUnknown(db, pid, f, "q2")

	lines, _ := statusReport(db, pid)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "! F1 f: 2 unknowns ouverts") {
		t.Fatalf("missing open-unknown warning:\n%s", joined)
	}

	resolveUnknown(db, pid, int64(u1))
	lines, _ = statusReport(db, pid)
	joined = strings.Join(lines, "\n")
	if !strings.Contains(joined, "! F1 f: 1 unknowns ouverts") {
		t.Fatalf("warning count not updated after resolve:\n%s", joined)
	}
}
