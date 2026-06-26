package sdd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInstrumented drives the EXPORTED NewRootCmd (the instrumented tree, V110),
// so invocations record usage — unlike rootRun, which uses the unexported
// newRootCmd and is never counted.
func runInstrumented(t *testing.T, dir, stdin string, args ...string) error {
	t.Helper()
	t.Chdir(dir)
	root := NewRootCmd()
	root.SetArgs(args)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetIn(strings.NewReader(stdin))
	return root.Execute()
}

func mustRunInstrumented(t *testing.T, dir, stdin string, args ...string) {
	t.Helper()
	if err := runInstrumented(t, dir, stdin, args...); err != nil {
		t.Fatalf("cmd %v: %v", args, err)
	}
}

// usageCounts reads command_usage from the current global store as
// command -> {ok, fail}, opening WITHOUT migrating (mirrors the telemetry path).
func usageCounts(t *testing.T) map[string][2]int64 {
	t.Helper()
	db, err := open(globalDBPath())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT command, ok_count, fail_count FROM command_usage`)
	if err != nil {
		t.Fatalf("query usage: %v", err)
	}
	defer rows.Close()
	out := map[string][2]int64{}
	for rows.Next() {
		var c string
		var ok, fail int64
		if err := rows.Scan(&c, &ok, &fail); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[c] = [2]int64{ok, fail}
	}
	return out
}

// V111: with the usage decorator ACTIVE (the exported NewRootCmd path), read
// commands still neither re-export SPEC.md nor fail `check`. Every read now
// writes a command_usage row, but that table is outside the renderer's scope, so
// SPEC.md stays byte-identical and check stays green — read purity holds at the
// spec-artifact level even though telemetry mutates the db.
func TestReadsStayPureUnderUsageRecording(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := gitRepo(t)

	mustRunInstrumented(t, dir, "", "init")
	mustRunInstrumented(t, dir, "", "add-invariant", "always check auth")
	mustRunInstrumented(t, dir, "", "new-feature", "f")

	spec := filepath.Join(dir, "SPEC.md")
	before, err := os.ReadFile(spec)
	if err != nil {
		t.Fatalf("read SPEC.md: %v", err)
	}

	for _, args := range [][]string{
		{"show", "V1"}, {"list", "invariant"}, {"status"},
		{"next"}, {"cat"}, {"search", "auth"}, {"stats"},
	} {
		mustRunInstrumented(t, dir, "", args...)
	}

	after, err := os.ReadFile(spec)
	if err != nil {
		t.Fatalf("re-read SPEC.md: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("a read re-exported SPEC.md under usage recording (V111)")
	}
	mustRunInstrumented(t, dir, "", "check") // db == frozen SPEC.md despite usage writes

	// prove the reads were ACTUALLY recorded (not silently no-op'd), so the
	// purity guarantee holds *with* live telemetry.
	if u := usageCounts(t); u["stats"][0] == 0 {
		t.Error("usage recording did not fire under the instrumented root — test is vacuous")
	}
}

// V112/V116: even when the usage table is missing, a command still succeeds —
// recording is best-effort and never fatal, and never recreates the table (no
// schema side-effect from telemetry).
func TestRecordingFailureNeverBreaksCommand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := gitRepo(t)
	mustRunInstrumented(t, dir, "", "init")

	// drop the usage table out from under the telemetry path; user_version stays
	// 7 so neither openGlobalDB nor recording will recreate it.
	db, err := open(globalDBPath())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE command_usage`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	db.Close()

	if err := runInstrumented(t, dir, "", "cat"); err != nil {
		t.Fatalf("read failed when usage table absent (V112 best-effort): %v", err)
	}

	db2, err := open(globalDBPath())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()
	if tableSQL(t, db2, "command_usage") != "" {
		t.Error("telemetry recreated command_usage (V116: no schema side-effect)")
	}
}

// V110/T136: a single `sdd apply` of N lines records exactly one `apply`
// invocation — its per-line add-* run on the UNINSTRUMENTED newRootCmd
// (apply.go) and must never be counted as user invocations (the re-entrance
// guard). Counting them would inflate add-* and conflate one user command with N.
func TestApplyDoesNotInflateUsage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repo := gitRepo(t)
	mustRunInstrumented(t, repo, "", "init")

	batch := strings.Join([]string{
		tab("new-feature", "f"),
		tab("add-invariant", "inv one"),
		tab("add-goal", "goal one"),
		tab("add-task", "--cites", "V1", "task one"),
	}, "\n") + "\n"
	mustRunInstrumented(t, repo, batch, "apply")

	u := usageCounts(t)
	for _, internal := range []string{"new-feature", "add-invariant", "add-goal", "add-task"} {
		if _, seen := u[internal]; seen {
			t.Errorf("internal apply line %q counted as a user invocation (re-entrance leak)", internal)
		}
	}
	if got := u["apply"]; got != [2]int64{1, 0} {
		t.Errorf("apply counts = %v, want one success {1 0}", got)
	}
	if got := u["init"]; got != [2]int64{1, 0} {
		t.Errorf("init counts = %v, want one success {1 0}", got)
	}
}

// V45/V113: the v7 command_usage table is byte-identical whether the db is
// created fresh (applySchema) or migrated up from v2 — single-source DDL, no
// divergence, same as V36/V45 prove for the earlier additive tables.
func TestFreshEqualsMigratedSchemaV7(t *testing.T) {
	fresh := openTestDB(t)
	freshSQL := tableSQL(t, fresh, "command_usage")

	v2 := openV2(t)
	if err := migrate(v2, 2); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	migratedSQL := tableSQL(t, v2, "command_usage")

	if freshSQL == "" {
		t.Fatal("fresh db has no command_usage table — the v7 step did not apply")
	}
	if freshSQL != migratedSQL {
		t.Errorf("schema divergence:\nfresh:    %q\nmigrated: %q", freshSQL, migratedSQL)
	}
	if uv := userVersionOf(t, v2); uv != userVersion {
		t.Errorf("migrated db stamped v%d, want v%d", uv, userVersion)
	}
}
