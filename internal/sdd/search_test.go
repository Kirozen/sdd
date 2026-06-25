package sdd

import (
	"strings"
	"testing"
)

// T97 / V93: search matches a case-insensitive substring across V/I/T/B/R/U and
// renders each hit prefixed by its kind.
func TestSearchAcrossKinds(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addInvariant(db, pid, "the WIDGET must validate")
	addInterface(db, pid, "cmd", "widget-cmd", "spins the widget")
	fid, _ := addFeature(db, pid, "f")
	addTask(db, pid, fid, "build the widget", nil)
	addResearch(db, pid, "widgets", "widgets are fine", "src")

	hits, err := searchHits(db, pid, "WiDgEt") // mixed case -> case-insensitive
	if err != nil {
		t.Fatalf("searchHits: %v", err)
	}
	if len(hits) != 4 {
		t.Fatalf("want 4 hits (V,I,T,R), got %d: %v", len(hits), hits)
	}
	// canonical order: interface, research, invariant, bug, task, unknown
	wantPrefixes := []string{"interface ", "research ", "invariant ", "task "}
	for i, p := range wantPrefixes {
		if !strings.HasPrefix(hits[i], p) {
			t.Errorf("hit[%d] = %q, want prefix %q", i, hits[i], p)
		}
	}
}

// T97 / V93: search matches HUMAN content only, never cites/ordinals (else it
// would overlap refs).
func TestSearchIgnoresCites(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	v1, _ := addInvariant(db, pid, "plain rule")
	fid, _ := addFeature(db, pid, "f")
	addTask(db, pid, fid, "does a thing", []string{"V1"}) // cites V1

	// "V1" appears in the task's cites, not its text -> no task hit.
	hits, err := searchHits(db, pid, "V1")
	if err != nil {
		t.Fatalf("searchHits: %v", err)
	}
	for _, h := range hits {
		if strings.HasPrefix(h, "task ") {
			t.Errorf("search matched a cite, not human content: %q", h)
		}
	}
	_ = v1
}

// T97 / V17: zero hits -> empty result.
func TestSearchNoHits(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addInvariant(db, pid, "something")
	hits, err := searchHits(db, pid, "zzz-absent")
	if err != nil {
		t.Fatalf("searchHits: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("want 0 hits, got %v", hits)
	}
}

// T97 / V20: search is project-scoped — it never returns another project's rows.
func TestSearchScoped(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)
	addInvariant(db, b, "beta-only secret")

	hits, err := searchHits(db, a, "beta-only")
	if err != nil {
		t.Fatalf("searchHits: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("search leaked B's rows into A (V20): %v", hits)
	}
}
