package main

import (
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
		for _, l := range lines {
			if l == s {
				return true
			}
		}
		return false
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
