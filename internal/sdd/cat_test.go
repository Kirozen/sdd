package sdd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// durablesOf returns the durable prefix (I,R,V,B) of a rendered spec — the text
// before the first feature block.
func durablesOf(spec string) string {
	if i := strings.Index(spec, "\n## FEATURE "); i >= 0 {
		return spec[:i]
	}
	return spec
}

// featureBlock returns the exact bytes of one feature's block (leading "\n## "
// included) up to the next feature block or EOF; "" if absent.
func featureBlock(spec string, ord int) string {
	marker := fmt.Sprintf("\n## FEATURE %d:", ord)
	start := strings.Index(spec, marker)
	if start < 0 {
		return ""
	}
	rest := spec[start+1:]
	if next := strings.Index(rest, "\n## FEATURE "); next >= 0 {
		return spec[start : start+1+next]
	}
	return spec[start:]
}

// V75: cat's default selector (openFeatures) emits every unfinished feature — a
// non-x task OR zero tasks — and omits fully-built features.
func TestCatDefaultScopesToUnfinished(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f1, _ := addFeature(db, pid, "built") // ord 1
	f2, _ := addFeature(db, pid, "open")  // ord 2
	addFeature(db, pid, "grilled")        // ord 3, zero tasks
	addTask(db, pid, f1, "done", nil)     // T1
	addTask(db, pid, f2, "todo", nil)     // T2
	if err := setTaskStatus(db, pid, 1, 1, "x"); err != nil {
		t.Fatal(err)
	}

	out, err := renderSpecScoped(db, pid, openFeatures)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "## FEATURE 1:") {
		t.Error("built feature 1 leaked into default cat (V75)")
	}
	if !strings.Contains(out, "## FEATURE 2:") {
		t.Error("open feature 2 (non-x task) missing from default cat (V75)")
	}
	if !strings.Contains(out, "## FEATURE 3:") {
		t.Error("grilled feature 3 (zero tasks) hidden from default cat — the BLOCK the review caught (V75)")
	}
}

// V75: --feature N narrows to exactly that feature, even when it is built.
func TestCatFeatureByOrdShowsBuilt(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f1, _ := addFeature(db, pid, "built")
	addFeature(db, pid, "open")
	addTask(db, pid, f1, "done", nil)
	if err := setTaskStatus(db, pid, 1, 1, "x"); err != nil {
		t.Fatal(err)
	}

	out, err := renderSpecScoped(db, pid, featureByOrd(1))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## FEATURE 1:") {
		t.Error("--feature 1 must show feature 1 even when built (V75)")
	}
	if strings.Contains(out, "## FEATURE 2:") {
		t.Error("--feature 1 leaked feature 2 (V75)")
	}
}

// V75: an unknown ordinal is an error (the command exits non-zero), unlike the
// open selector whose empty result is valid.
func TestCatUnknownFeatureErrors(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addFeature(db, pid, "only")

	if _, err := featureByOrd(999)(db, pid); err == nil {
		t.Error("featureByOrd(999) must error so cat --feature 999 exits non-zero (V75)")
	}
	// open selector with everything built is NOT an error — empty is valid.
	f1, _ := addFeature(db, pid, "built") // ord 2
	addTask(db, pid, f1, "done", nil)     // T1
	if err := setTaskStatus(db, pid, 2, 1, "x"); err != nil {
		t.Fatal(err)
	}
}

// V75: all features built + default selector -> durables only, no feature block.
func TestCatAllBuiltDurablesOnly(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addInvariant(db, pid, "an invariant")
	f1, _ := addFeature(db, pid, "built")
	addTask(db, pid, f1, "done", nil)
	if err := setTaskStatus(db, pid, 1, 1, "x"); err != nil {
		t.Fatal(err)
	}

	out, err := renderSpecScoped(db, pid, openFeatures)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "## FEATURE") {
		t.Error("all features built: default cat must emit no feature block (V75)")
	}
	if !strings.Contains(out, "## §V INVARIANTS") || !strings.Contains(out, "an invariant") {
		t.Error("durables must always render in full (V76)")
	}
}

// V76/V77: every block cat emits is byte-identical to that block in the FULL
// spec, even when the rendered (open) feature sits between two built features so
// cat's output is not a contiguous substring of SPEC.md (NOTE-1).
func TestCatBlocksByteIdenticalToFullSpec(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addInvariant(db, pid, "v one")
	f1, _ := addFeature(db, pid, "built A") // ord 1
	f2, _ := addFeature(db, pid, "open B")  // ord 2
	f3, _ := addFeature(db, pid, "built C") // ord 3
	addTask(db, pid, f1, "done a", nil)     // T1
	addTask(db, pid, f2, "todo b", nil)     // T2
	addTask(db, pid, f3, "done c", nil)     // T3
	if err := setTaskStatus(db, pid, 1, 1, "x"); err != nil {
		t.Fatal(err)
	}
	if err := setTaskStatus(db, pid, 3, 1, "x"); err != nil {
		t.Fatal(err)
	}

	full, err := renderSpec(db, pid)
	if err != nil {
		t.Fatal(err)
	}
	scoped, err := renderSpecScoped(db, pid, openFeatures)
	if err != nil {
		t.Fatal(err)
	}

	if durablesOf(scoped) != durablesOf(full) {
		t.Error("durables block differs between cat and full spec (V76)")
	}
	if fb := featureBlock(full, 2); fb == "" || featureBlock(scoped, 2) != fb {
		t.Error("open feature 2 block not byte-identical to its full-spec slice (V76, NOTE-1)")
	}
	if strings.Contains(scoped, "## FEATURE 1:") || strings.Contains(scoped, "## FEATURE 3:") {
		t.Error("built features leaked into scoped render (V75)")
	}
}

// V77: export/check render the FULL spec — built features stay present, so the
// scoped cat view never narrows the V6 drift contract.
func TestExportStaysFullWithBuiltFeatures(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f1, _ := addFeature(db, pid, "built")
	addFeature(db, pid, "open")
	addTask(db, pid, f1, "done", nil)
	if err := setTaskStatus(db, pid, 1, 1, "x"); err != nil {
		t.Fatal(err)
	}

	full, err := renderSpec(db, pid)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(full, "## FEATURE 1:") || !strings.Contains(full, "## FEATURE 2:") {
		t.Error("renderSpec (export+check path) must contain ALL features incl. built (V77)")
	}
}

// V75/I.cat: the cat command exits non-zero on an unknown feature ordinal, via
// the real cobra root.
func TestCatCmdUnknownFeatureExitsNonZero(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := gitRepo(t)
	t.Chdir(dir)

	run := func(args ...string) error {
		root := newRootCmd()
		root.SetArgs(args)
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		return root.Execute()
	}
	if err := run("init"); err != nil {
		t.Fatal(err)
	}
	if err := run("cat", "--feature", "999"); err == nil {
		t.Error("cat --feature 999 must exit non-zero (V75)")
	}
}
