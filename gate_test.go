package main

import (
	"strings"
	"testing"
)

// V46: one verdict per feature — a second gate replaces the first (UPSERT on the
// UNIQUE feature_id), it does not accumulate.
func TestSetGateUpsert(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fpk, err := addFeature(db, pid, "f") // F1
	if err != nil {
		t.Fatalf("addFeature: %v", err)
	}
	if err := setGate(db, pid, 1, "go", ""); err != nil {
		t.Fatalf("setGate go: %v", err)
	}
	if err := setGate(db, pid, 1, "no-go", "found a hole"); err != nil {
		t.Fatalf("setGate no-go: %v", err)
	}
	if n := count(t, db, "gate"); n != 1 {
		t.Errorf("gate rows = %d, want 1 (UPSERT replaces)", n)
	}
	verdict, has, err := featureGate(db, fpk)
	if err != nil || !has {
		t.Fatalf("featureGate: has=%v err=%v", has, err)
	}
	if verdict != "no-go" {
		t.Errorf("verdict = %q, want no-go (latest wins)", verdict)
	}
}

// V46: a gate cascades with its feature (V4) — wiping the feature drops it.
func TestGateCascade(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fpk, err := addFeature(db, pid, "f")
	if err != nil {
		t.Fatalf("addFeature: %v", err)
	}
	if err := setGate(db, pid, 1, "go", ""); err != nil {
		t.Fatalf("setGate: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM feature WHERE id=?`, fpk); err != nil {
		t.Fatalf("delete feature: %v", err)
	}
	if n := count(t, db, "gate"); n != 0 {
		t.Errorf("gate rows after feature delete = %d, want 0 (cascade)", n)
	}
}

// V47: guide refines a specced feature's next move by its verdict — none →
// review then build, go → build, no-go → blocked.
func TestGuideRefinesSpeccedByGate(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fpk, _ := addFeature(db, pid, "f")
	if _, err := addTask(db, pid, fpk, "todo", nil); err != nil { // a single '.' task → specced
		t.Fatalf("addTask: %v", err)
	}

	specLine := func() string {
		t.Helper()
		lines, err := guideReport(db, pid)
		if err != nil {
			t.Fatalf("guideReport: %v", err)
		}
		return strings.Join(lines, "\n")
	}

	if got := specLine(); !strings.Contains(got, "[specced] → sdd-review then sdd-build") {
		t.Errorf("no gate should point at review:\n%s", got)
	}
	if err := setGate(db, pid, 1, "go", ""); err != nil {
		t.Fatalf("setGate go: %v", err)
	}
	if got := specLine(); !strings.Contains(got, "sdd-build (reviewed:go)") {
		t.Errorf("go verdict should point at build:\n%s", got)
	}
	if err := setGate(db, pid, 1, "no-go", ""); err != nil {
		t.Fatalf("setGate no-go: %v", err)
	}
	if got := specLine(); !strings.Contains(got, "blocked: re-spec then re-review (no-go)") {
		t.Errorf("no-go verdict should block:\n%s", got)
	}
}

// V46: gate rows are not rendered into SPEC.md — recording a verdict leaves the
// generated spec byte-identical (cf V44), so guide stays read-pure (V16) and the
// drift contract (V6) is untouched.
func TestGateAbsentFromSpec(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := addFeature(db, pid, "f"); err != nil {
		t.Fatalf("addFeature: %v", err)
	}
	before, err := renderSpec(db, pid)
	if err != nil {
		t.Fatalf("renderSpec: %v", err)
	}
	if err := setGate(db, pid, 1, "go", "looks fine"); err != nil {
		t.Fatalf("setGate: %v", err)
	}
	after, err := renderSpec(db, pid)
	if err != nil {
		t.Fatalf("renderSpec: %v", err)
	}
	if before != after {
		t.Error("a gate row changed SPEC.md (V46: verdicts must not be rendered)")
	}
}

// V49: the migrated gate table is byte-identical to the fresh one, which also
// proves migrate chains all the way to v5 (2→3→4→5) from a v2 db.
func TestFreshEqualsMigratedSchemaV5(t *testing.T) {
	fresh := openTestDB(t)
	freshSQL := tableSQL(t, fresh, "gate")

	v2 := openV2(t)
	if err := migrate(v2, 2); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	migratedSQL := tableSQL(t, v2, "gate")

	if freshSQL == "" || freshSQL != migratedSQL {
		t.Errorf("schema divergence:\nfresh:    %q\nmigrated: %q", freshSQL, migratedSQL)
	}
}
