package sdd

import (
	"bytes"
	"strings"
	"testing"
)

// statCount pulls the count rendered for a given type label out of a stats
// report (lines like "  invariants  2"); the first field is the label, the
// second its count.
func statCount(t *testing.T, lines []string, label string) string {
	t.Helper()
	for _, l := range lines {
		f := strings.Fields(l)
		if len(f) >= 2 && f[0] == label {
			return f[1]
		}
	}
	t.Fatalf("label %q not found in %v", label, lines)
	return ""
}

// V104: a project's stats count ONLY its own rows — another project's rows never
// leak in (this is the cross-project isolation the new command must inherit).
func TestStatsScopedToProject(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)

	// A: a known shape.
	inv1, _ := addInvariant(db, a, "a-inv-1")
	addInvariant(db, a, "a-inv-2")
	addResearch(db, a, "topic", "finding", "src")
	if err := addTest(db, a, inv1, "TestA"); err != nil {
		t.Fatalf("addTest: %v", err)
	}
	fa, _ := addFeature(db, a, "fa")
	addTask(db, a, fa, "todo", nil)
	tDoing, _ := addTask(db, a, fa, "doing", nil)
	setTaskStatus(db, a, 1, tDoing, "~")
	tDone, _ := addTask(db, a, fa, "done", nil)
	setTaskStatus(db, a, 1, tDone, "x")
	addUnknown(db, a, fa, "an unknown")

	// B: deliberately different volumes that must NOT bleed into A's report.
	for i := 0; i < 5; i++ {
		addInvariant(db, b, "b-inv")
	}
	addFeature(db, b, "fb1")
	addFeature(db, b, "fb2")

	lines, err := statsReport(db, a)
	if err != nil {
		t.Fatalf("statsReport: %v", err)
	}
	if !strings.HasPrefix(lines[0], "PROJECT ") {
		t.Errorf("missing PROJECT header: %q", lines[0])
	}
	// A's own counts, NOT polluted by B's 5 invariants / 2 features.
	for label, want := range map[string]string{
		"invariants": "2", "research": "1", "tests": "1",
		"unknowns": "1", "features": "1", "tasks": "3",
	} {
		if got := statCount(t, lines, label); got != want {
			t.Errorf("%s = %s, want %s (cross-project leak?)", label, got, want)
		}
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "(. 1  ~ 1  x 1)") {
		t.Errorf("task status breakdown wrong:\n%s", joined)
	}
}

// V105: --all sums each project's scoped ProjectStats over the registry — the
// totals equal the per-project sums, and the project count matches.
func TestStatsAllSumsRegistry(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)

	addInvariant(db, a, "a1")
	addInvariant(db, a, "a2")
	fa, _ := addFeature(db, a, "fa")
	addTask(db, a, fa, "t", nil)
	for i := 0; i < 5; i++ {
		addInvariant(db, b, "b")
	}
	addFeature(db, b, "fb")

	lines, err := allStatsReport(db)
	if err != nil {
		t.Fatalf("allStatsReport: %v", err)
	}
	if !strings.HasPrefix(lines[0], "STORE ") {
		t.Errorf("missing STORE header: %q", lines[0])
	}
	if got := statCount(t, lines, "projects"); got != "2" {
		t.Errorf("projects = %s, want 2", got)
	}
	if got := statCount(t, lines, "invariants"); got != "7" { // 2 + 5
		t.Errorf("invariants total = %s, want 7", got)
	}
	if got := statCount(t, lines, "features"); got != "2" { // 1 + 1
		t.Errorf("features total = %s, want 2", got)
	}
	if got := statCount(t, lines, "tasks"); got != "1" {
		t.Errorf("tasks total = %s, want 1", got)
	}
}

// V104 edge: an empty project reports zeros (not a missing line / panic), and an
// empty registry reports projects:0 with zero totals.
func TestStatsEmpty(t *testing.T) {
	db := openTestDB(t)

	// empty registry (no projects at all).
	all, err := allStatsReport(db)
	if err != nil {
		t.Fatalf("allStatsReport on empty registry: %v", err)
	}
	if got := statCount(t, all, "projects"); got != "0" {
		t.Errorf("empty registry projects = %s, want 0", got)
	}
	if got := statCount(t, all, "invariants"); got != "0" {
		t.Errorf("empty registry invariants = %s, want 0", got)
	}

	// empty project.
	c := mustProject(t, db)
	lines, err := statsReport(db, c)
	if err != nil {
		t.Fatalf("statsReport empty project: %v", err)
	}
	for _, label := range []string{"invariants", "features", "tasks"} {
		if got := statCount(t, lines, label); got != "0" {
			t.Errorf("empty project %s = %s, want 0", label, got)
		}
	}
}

// V106: `stats --all` opens the global db directly, so it works OUTSIDE any git
// project (and reports a positive db file size); the default view, by contrast,
// aborts outside a git project like every scoped read.
func TestStatsAllOutsideRepo(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Chdir(t.TempDir()) // a plain dir, NOT a git repo

	run := func(args ...string) error {
		root := newRootCmd()
		root.SetArgs(args)
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		return root.Execute()
	}

	// default view must abort outside a git project (V106 / openProjectContext);
	// --all must NOT (it opens the global db directly).
	if err := run("stats"); err == nil {
		t.Error("stats (default) should fail outside a git project")
	}
	if err := run("stats", "--all"); err != nil {
		t.Fatalf("stats --all should work outside a repo: %v", err)
	}

	// db-size is the real spec.db file size (V105): inspect the rendered line.
	db, err := openGlobalDB()
	if err != nil {
		t.Fatalf("openGlobalDB: %v", err)
	}
	defer db.Close()
	lines, err := allStatsReport(db)
	if err != nil {
		t.Fatalf("allStatsReport: %v", err)
	}
	// db-size is humanized (e.g. "151.5 KiB"): a positive value plus a binary unit.
	var sizeLine string
	for _, l := range lines {
		if f := strings.Fields(l); len(f) >= 2 && f[0] == "db-size" {
			sizeLine = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "db-size"))
		}
	}
	if sizeLine == "0 B" || sizeLine == "" {
		t.Errorf("db-size = %q, want a positive humanized size", sizeLine)
	}
	if !strings.HasSuffix(sizeLine, "B") {
		t.Errorf("db-size = %q, want a byte unit suffix", sizeLine)
	}
}

// V105: humanBytes renders binary units deterministically at the boundaries.
func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		0:               "0 B",
		512:             "512 B",
		1023:            "1023 B",
		1024:            "1.0 KiB",
		290816:          "284.0 KiB",
		1024 * 1024:     "1.0 MiB",
		1024*1024*1024 + 1: "1.0 GiB",
	}
	for n, want := range cases {
		if got := humanBytes(n); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", n, got, want)
		}
	}
}
