package sdd

import (
	"slices"
	"strings"
	"testing"
)

// V18 + I.list: list emits, one per row, the exact lines renderSpec produces.
func TestListRenderParity(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := seedDB(db, pid, parseSpec(fixtureSpec), "f", false); err != nil {
		t.Fatalf("seedDB: %v", err)
	}
	full, err := renderSpec(db, pid)
	if err != nil {
		t.Fatalf("renderSpec: %v", err)
	}
	lines := strings.Split(full, "\n")
	inRender := func(s string) bool {
		return slices.Contains(lines, s)
	}

	want := map[string]int{"invariant": 2, "interface": 2, "task": 2, "bug": 1, "research": 1}
	for kind, n := range want {
		got, err := listKind(db, pid, kind)
		if err != nil {
			t.Fatalf("list %s: %v", kind, err)
		}
		if len(got) != n {
			t.Errorf("list %s = %d lines, want %d", kind, len(got), n)
		}
		for _, l := range got {
			if !inRender(l) {
				t.Errorf("list %s line %q not found verbatim in renderSpec (V18)", kind, l)
			}
		}
	}
}

// V28: list with no kind emits every kind in canonical order, each line
// byte-identical to its single-kind list. Building the expectation by walking
// listAllKinds and concatenating listKind makes any order/inclusion/rendering
// drift fail.
func TestListAllCanonicalOrder(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := seedDB(db, pid, parseSpec(fixtureSpec), "f", false); err != nil {
		t.Fatalf("seedDB: %v", err)
	}

	var want []string
	for _, kind := range listAllKinds {
		lines, err := listKind(db, pid, kind)
		if err != nil {
			t.Fatalf("list %s: %v", kind, err)
		}
		want = append(want, lines...)
	}

	got, err := listAll(db, pid)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("listAll mismatch (V28):\n got=%q\nwant=%q", got, want)
	}
	// guard the canonical order itself: interface rows before task rows
	if got[0] != want[0] || len(got) == 0 {
		t.Errorf("listAll first line = %q, want %q (interfaces lead)", got[0], want[0])
	}
}

// V29: --pretty renders a grouped human view — a header per kind, tasks nested
// under their feature, and no pipe-delimited machine rows. The default listAll
// output stays raw (V28).
func TestListPrettyGroupedNoPipes(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := seedDB(db, pid, parseSpec(fixtureSpec), "f", false); err != nil {
		t.Fatalf("seedDB: %v", err)
	}
	got, err := listPretty(db, pid)
	if err != nil {
		t.Fatalf("listPretty: %v", err)
	}
	joined := strings.Join(got, "\n")

	for _, h := range []string{"INTERFACES", "RESEARCH", "INVARIANTS", "BUGS", "FEATURE 1"} {
		if !slices.ContainsFunc(got, func(l string) bool { return strings.HasPrefix(l, h) }) {
			t.Errorf("pretty view missing header %q:\n%s", h, joined)
		}
	}

	// no machine pipe row survives: none of the raw listAll lines that carry a
	// pipe delimiter appear verbatim in the pretty view (parity deliberately
	// broken under the flag).
	raw, err := listAll(db, pid)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	for _, r := range raw {
		if strings.Contains(r, "|") && slices.Contains(got, r) {
			t.Errorf("pretty view leaked a raw pipe row: %q", r)
		}
	}

	// a task row sits indented AFTER its feature header and BEFORE the next one.
	featAt, taskAt := -1, -1
	for i, l := range got {
		if strings.HasPrefix(l, "FEATURE 1") {
			featAt = i
		}
		if featAt >= 0 && taskAt < 0 && strings.HasPrefix(l, "  T") {
			taskAt = i
			break
		}
	}
	if featAt < 0 || taskAt < 0 || taskAt <= featAt {
		t.Errorf("tasks not nested under their feature (feat@%d task@%d):\n%s", featAt, taskAt, joined)
	}
}

// V28: --pretty is opt-in; the default listAll path is byte-for-byte unchanged.
func TestListPrettyLeavesDefaultRaw(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := seedDB(db, pid, parseSpec(fixtureSpec), "f", false); err != nil {
		t.Fatalf("seedDB: %v", err)
	}
	raw, err := listAll(db, pid)
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	// the default still concatenates the canonical per-kind lines (the V28 contract)
	var want []string
	for _, kind := range listAllKinds {
		lines, _ := listKind(db, pid, kind)
		want = append(want, lines...)
	}
	if strings.Join(raw, "\n") != strings.Join(want, "\n") {
		t.Errorf("default list changed by pretty work (V28 regressed)")
	}
}

// V29 guard: --pretty combined with a kind argument errors instead of silently
// ignoring the flag.
func TestListPrettyWithKindErrors(t *testing.T) {
	cmd := newListCmd()
	cmd.SetArgs([]string{"task", "--pretty"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Error("list task --pretty succeeded, want error (V29: --pretty is no-kind only)")
	}
}

// V17: an unknown kind errors; a valid-but-empty kind returns no lines, no error.
func TestListUnknownKindAndEmpty(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := listKind(db, pid, "bogus"); err == nil {
		t.Error("list bogus succeeded, want error (V17)")
	}
	lines, err := listKind(db, pid, "bug") // fresh db: no bugs
	if err != nil {
		t.Fatalf("list bug on empty db: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("list bug on empty db = %d lines, want 0", len(lines))
	}
}
