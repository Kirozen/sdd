package sdd

import (
	"strings"
	"testing"
)

// V18: `show <ref>` emits the exact caveman line renderSpec produces for that
// row — both go through the same fmt*Line helpers, so no drift is possible.
func TestShowRenderParity(t *testing.T) {
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

	// one ref of each kind (fixtureSpec seeds them all)
	for _, ref := range []string{"V1", "I.init", "T1", "B1", "R1"} {
		got, err := showRef(db, pid, ref)
		if err != nil {
			t.Fatalf("show %s: %v", ref, err)
		}
		if !inRender(got) {
			t.Errorf("show %s = %q, not found verbatim in renderSpec output (V18 parity broken)", ref, got)
		}
	}
}

// V17: an invalid query (unknown ref or bad grammar) errors instead of printing.
func TestShowUnknownRef(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := seedDB(db, pid, parseSpec(fixtureSpec), "f", false); err != nil {
		t.Fatalf("seedDB: %v", err)
	}
	for _, ref := range []string{"V99", "I.nope", "T42", "B7", "R3", "Z1", "garbage"} {
		if _, err := showRef(db, pid, ref); err == nil {
			t.Errorf("show %q succeeded, want error (V17)", ref)
		}
	}
}
