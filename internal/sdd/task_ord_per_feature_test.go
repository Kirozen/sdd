package sdd

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dbq "github.com/kirozen/sdd/internal/db"
)

// V117: task ordinals are per-feature — each feature restarts at T1, independent
// of how many tasks earlier features hold.
func TestTaskOrdRestartsPerFeature(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f1, _ := addFeature(db, pid, "f1")
	f2, _ := addFeature(db, pid, "f2")

	addTask(db, pid, f1, "a", nil) // F1 T1
	addTask(db, pid, f1, "b", nil) // F1 T2
	addTask(db, pid, f2, "c", nil) // F2 T1 (not T3)
	addTask(db, pid, f2, "d", nil) // F2 T2

	want := map[string]int64{"a": 1, "b": 2, "c": 1, "d": 2}
	for text, wantOrd := range want {
		var ord int64
		if err := db.QueryRow(`SELECT ord FROM task WHERE text=?`, text).Scan(&ord); err != nil {
			t.Fatalf("scan %q: %v", text, err)
		}
		if ord != wantOrd {
			t.Errorf("task %q ord = %d, want %d (per-feature, V117)", text, ord, wantOrd)
		}
	}
}

// V117: a per-feature T<n> is addressed by (T-ord, --feature). The same T1 in two
// features is disambiguated by --feature; a bare T<n> without --feature is rejected.
func TestSetTaskAddressedByFeature(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := gitRepo(t)
	t.Chdir(dir)
	rootMustRun(t, dir, "", "init")
	rootMustRun(t, dir, "", "new-feature", "fa") // F1
	rootMustRun(t, dir, "", "new-feature", "fb") // F2
	rootMustRun(t, dir, "", "add-task", "ta", "--feature", "1")
	rootMustRun(t, dir, "", "add-task", "tb", "--feature", "2")

	// bare T1 without --feature is rejected (ambiguous, V117)
	if err := rootRun(t, dir, "", "set-task", "T1", "--status", "x"); err == nil {
		t.Error("set-task without --feature should fail (T<n> is per-feature, V117)")
	}

	// T1 --feature 2 hits only feature 2's task
	rootMustRun(t, dir, "", "set-task", "T1", "--feature", "2", "--status", "x")

	spec, err := os.ReadFile(filepath.Join(dir, "SPEC.md"))
	if err != nil {
		t.Fatalf("read SPEC.md: %v", err)
	}
	s := string(spec)
	if !strings.Contains(s, "T1|x|tb") {
		t.Errorf("feature 2 task not marked done via T1 --feature 2:\n%s", s)
	}
	if !strings.Contains(s, "T1|.|ta") {
		t.Errorf("feature 1 task wrongly touched (should stay todo):\n%s", s)
	}
}

// goal: deleting a feature leaves no hole in any OTHER feature's task numbering —
// trivially true once ords are per-feature, since features number independently.
func TestFeatureDeletionLeavesNoHoleElsewhere(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f1, _ := addFeature(db, pid, "f1")
	f2, _ := addFeature(db, pid, "f2")
	f3, _ := addFeature(db, pid, "f3")
	addTask(db, pid, f1, "a1", nil)
	addTask(db, pid, f1, "a2", nil)
	addTask(db, pid, f2, "b1", nil)
	addTask(db, pid, f3, "c1", nil)
	addTask(db, pid, f3, "c2", nil)

	// drop feature 2 entirely (cascade its tasks)
	if _, err := dbq.New(db).WipeFeature(context.Background(), dbq.WipeFeatureParams{ProjectID: pid, Ord: 2}); err != nil {
		t.Fatalf("wipe feature 2: %v", err)
	}

	// f1 and f3 keep a dense 1..N — no gap from the deletion
	for _, f := range []struct {
		pk   int64
		want []int64
	}{{f1, []int64{1, 2}}, {f3, []int64{1, 2}}} {
		rows, _ := db.Query(`SELECT ord FROM task WHERE feature_id=? ORDER BY ord`, f.pk)
		var got []int64
		for rows.Next() {
			var o int64
			rows.Scan(&o)
			got = append(got, o)
		}
		rows.Close()
		if len(got) != len(f.want) {
			t.Fatalf("feature pk %d: ords %v, want %v", f.pk, got, f.want)
		}
		for i := range got {
			if got[i] != f.want[i] {
				t.Errorf("feature pk %d: ords %v, want %v (no hole)", f.pk, got, f.want)
			}
		}
	}
}

// V97 preserved under per-feature: rm-task leaves the survivors' ords untouched —
// a gap, never a renumber.
func TestRmTaskNoRenumberPerFeature(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f1, _ := addFeature(db, pid, "f1")
	addTask(db, pid, f1, "a", nil) // T1
	addTask(db, pid, f1, "b", nil) // T2
	addTask(db, pid, f1, "c", nil) // T3

	if err := rmTask(db, pid, 1, 2); err != nil { // remove F1 T2
		t.Fatalf("rmTask: %v", err)
	}
	var ords []int64
	rows, _ := db.Query(`SELECT ord FROM task WHERE feature_id=? ORDER BY ord`, f1)
	for rows.Next() {
		var o int64
		rows.Scan(&o)
		ords = append(ords, o)
	}
	rows.Close()
	if len(ords) != 2 || ords[0] != 1 || ords[1] != 3 {
		t.Errorf("survivor ords = %v, want [1 3] (gap kept, no renumber, V97)", ords)
	}
}

// V118: a search hit of kind task carries its owning feature ord, so a per-feature
// T<n> stays addressable from a flat result.
func TestSearchTaskCarriesFeature(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f2 := func() int64 {
		addFeature(db, pid, "f1") // F1, ord 1
		pk, _ := addFeature(db, pid, "f2")
		return pk
	}()
	addTask(db, pid, f2, "needle task", nil) // F2 T1

	hits, err := searchHits(db, pid, "needle")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || !strings.HasPrefix(hits[0], "task F2 T1|") {
		t.Errorf("search hit = %v, want a 'task F2 T1|...' line (V118)", hits)
	}
}

// V119: UNIQUE(feature_id, ord) — two tasks of one feature cannot share an ord.
func TestUniqueFeatureOrdRejectsDuplicate(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	f1, _ := addFeature(db, pid, "f1")
	q := dbq.New(db)
	if _, err := q.InsertTaskFull(context.Background(), dbq.InsertTaskFullParams{FeatureID: f1, Ord: 1, Text: "a", Status: "."}); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := q.InsertTaskFull(context.Background(), dbq.InsertTaskFullParams{FeatureID: f1, Ord: 1, Text: "dup", Status: "."}); err == nil {
		t.Error("duplicate (feature_id, ord) accepted — UNIQUE index missing (V119)")
	}
}

// openV7 builds a db frozen at the v7 schema (steps 001..006), the shape an
// existing global db has just before the F25 per-feature migration (007).
func openV7(t *testing.T) *sql.DB {
	t.Helper()
	db, err := open(filepath.Join(t.TempDir(), "spec.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	for i := 0; i < 6; i++ { // steps[0..5] = 001..006 => schema v2..v7
		if _, err := db.Exec(steps[i].sql); err != nil {
			t.Fatalf("apply step %d: %v", i, err)
		}
	}
	if _, err := db.Exec("PRAGMA user_version=7;"); err != nil {
		t.Fatalf("stamp v7: %v", err)
	}
	return db
}

// V119/V45: migrating a v7 db renumbers pre-existing project-wide task ords into a
// dense per-feature 1..N, original order preserved, regardless of insert order —
// the snapshot-based renumber is collision-free.
func TestMigrateRenumbersTaskOrdsPerFeature(t *testing.T) {
	db := openV7(t)
	pid := mustProject(t, db)
	fa, _ := addFeature(db, pid, "fa") // ord 1
	fb, _ := addFeature(db, pid, "fb") // ord 2
	q := dbq.New(db)
	ins := func(fpk, ord int64, text string) {
		if _, err := q.InsertTaskFull(context.Background(), dbq.InsertTaskFullParams{FeatureID: fpk, Ord: ord, Text: text, Status: "."}); err != nil {
			t.Fatalf("insert %q: %v", text, err)
		}
	}
	// Project-wide ords, deliberately non-dense and inserted high-then-low so a
	// naive mutating renumber would collide.
	ins(fa, 3, "a-first")
	ins(fa, 7, "a-second")
	ins(fb, 11, "b-first")
	ins(fb, 5, "b-second") // lower ord inserted after the higher one

	if err := migrate(db, 7); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// fa: 3->1, 7->2 ; fb: 5->1, 11->2 (dense, ordered by old ord)
	want := map[string]int64{"a-first": 1, "a-second": 2, "b-second": 1, "b-first": 2}
	for text, wantOrd := range want {
		var ord int64
		if err := db.QueryRow(`SELECT ord FROM task WHERE text=?`, text).Scan(&ord); err != nil {
			t.Fatalf("scan %q: %v", text, err)
		}
		if ord != wantOrd {
			t.Errorf("after renumber, %q ord = %d, want %d", text, ord, wantOrd)
		}
	}
	if uv := userVersionOf(t, db); uv != userVersion {
		t.Errorf("post-migrate user_version = %d, want %d", uv, userVersion)
	}
}

// V45: the v8 unique index is byte-identical fresh vs migrated, and the task table
// matches — the single-source DDL holds for the F25 step.
func TestFreshEqualsMigratedSchemaV8(t *testing.T) {
	fresh := openTestDB(t)
	freshIdx := indexSQL(t, fresh, "idx_task_feature_ord")

	v2 := openV2(t)
	if err := migrate(v2, 2); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	migratedIdx := indexSQL(t, v2, "idx_task_feature_ord")

	if freshIdx == "" {
		t.Fatal("fresh db missing idx_task_feature_ord — the v8 step did not apply")
	}
	if freshIdx != migratedIdx {
		t.Errorf("index divergence:\nfresh:    %q\nmigrated: %q", freshIdx, migratedIdx)
	}
	if tableSQL(t, fresh, "task") != tableSQL(t, v2, "task") {
		t.Error("task table differs fresh vs migrated")
	}
}

func indexSQL(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&ddl); err != nil {
		return ""
	}
	return ddl
}
