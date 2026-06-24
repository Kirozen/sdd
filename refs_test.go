package main

import (
	"fmt"
	"strings"
	"testing"
)

// I.refs + V18: refs lists the citers of a target as their caveman lines.
// fixture: T1 cites V1,I.init ; T2 cites V2 ; B1 fixed by V2.
func TestRefsReverseCites(t *testing.T) {
	db := openTestDB(t)
	if err := seedDB(db, parseSpec(fixtureSpec), "f", false); err != nil {
		t.Fatalf("seedDB: %v", err)
	}

	// V2 is cited by task T2 and fixed by bug B1 → two citers, tasks before bugs
	got, err := refsTo(db, "V2")
	if err != nil {
		t.Fatalf("refs V2: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("refs V2 = %d lines, want 2: %v", len(got), got)
	}
	if !strings.HasPrefix(got[0], "T2|") {
		t.Errorf("refs V2[0] = %q, want the citing task T2", got[0])
	}
	if !strings.HasPrefix(got[1], "B1|") {
		t.Errorf("refs V2[1] = %q, want the fixing bug B1", got[1])
	}

	// I.init is cited only by T1
	got, err = refsTo(db, "I.init")
	if err != nil {
		t.Fatalf("refs I.init: %v", err)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], "T1|") {
		t.Errorf("refs I.init = %v, want [T1...]", got)
	}
}

// V17: a non-target ref or unknown ref errors; an existing-but-uncited target
// returns no lines and no error.
func TestRefsEmptyAndUnknown(t *testing.T) {
	db := openTestDB(t)
	if err := seedDB(db, parseSpec(fixtureSpec), "f", false); err != nil {
		t.Fatalf("seedDB: %v", err)
	}
	if _, err := refsTo(db, "V99"); err == nil {
		t.Error("refs V99 succeeded, want error (unknown invariant)")
	}
	if _, err := refsTo(db, "T1"); err == nil {
		t.Error("refs T1 succeeded, want error (task is not a cite target)")
	}

	id, err := addInvariant(db, "lonely")
	if err != nil {
		t.Fatalf("addInvariant: %v", err)
	}
	lines, err := refsTo(db, fmt.Sprintf("V%d", id))
	if err != nil {
		t.Fatalf("refs on uncited invariant: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("refs uncited = %d lines, want 0", len(lines))
	}
}
