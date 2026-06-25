package sdd

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedSpec inserts one of each kind into a fresh project, wiring a task that
// cites both an invariant and an interface so the re-join path is exercised.
// Returns the project id to scope render/export/check.
func seedSpec(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	pid := mustProject(t, db)
	exec := func(q string, a ...any) {
		t.Helper()
		if _, err := db.Exec(q, a...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	exec(`INSERT INTO invariant(id, project_id, ord, text) VALUES(1, ?, 1, 'auth check before handler')`, pid)
	exec(`INSERT INTO interface(id, project_id, kind, name, sig) VALUES(1, ?, 'cmd', 'init', 'create db')`, pid)
	exec(`INSERT INTO feature(id, project_id, ord, name) VALUES(7, ?, 1, 'auth-login')`, pid)
	exec(`INSERT INTO goal(feature_id, text) VALUES(7, 'login JWT')`)
	exec(`INSERT INTO "constraint"(feature_id, text) VALUES(7, 'tokens expire')`)
	exec(`INSERT INTO task(id, feature_id, ord, text, status) VALUES(1, 7, 1, 'impl mw', 'x')`)
	exec(`INSERT INTO task_cites_inv(task_id, inv_id) VALUES(1, 1)`)
	exec(`INSERT INTO task_cites_iface(task_id, iface_id) VALUES(1, 1)`)
	return pid
}

// V1, V7: render is a deterministic, volatile-free function of db state.
func TestExportDeterministic(t *testing.T) {
	db := openTestDB(t)
	pid := seedSpec(t, db)

	first, err := renderSpec(db, pid)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	second, err := renderSpec(db, pid)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if first != second {
		t.Errorf("render not deterministic:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// V3: the generated header marks the file do-not-edit.
func TestExportHeader(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	out, err := renderSpec(db, pid)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.HasPrefix(out, generatedHead) {
		t.Errorf("missing generated header; got start:\n%.80s", out)
	}
}

// cites re-join from the typed tables into V1,I.init form.
func TestExportCitesRejoin(t *testing.T) {
	db := openTestDB(t)
	pid := seedSpec(t, db)
	out, err := renderSpec(db, pid)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "T1|x|impl mw|V1,I.init") {
		t.Errorf("task cites not re-joined; got:\n%s", out)
	}
}

// V8: export atomically replaces the file and leaves no temp behind.
func TestExportAtomic(t *testing.T) {
	db := openTestDB(t)
	pid := seedSpec(t, db)
	dir := t.TempDir()
	path := filepath.Join(dir, "SPEC.md")

	if err := exportSpec(db, pid, path); err != nil {
		t.Fatalf("exportSpec: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	want, _ := renderSpec(db, pid)
	if string(got) != want {
		t.Error("exported file != render output")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file left behind after atomic replace")
	}
}
