package sdd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// V16: read commands are pure — running show/list/refs/status must neither
// re-export SPEC.md nor mutate the db. Driven end-to-end through the cobra root
// (the same path the CLI uses). The two assertions together are exhaustive: if a
// read re-exported, SPEC.md bytes change; if a read mutated the db without
// re-exporting, `check` fails (db ≠ frozen SPEC.md). Passing both ⇒ no write.
func TestReadCommandsArePure(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := gitRepo(t)
	t.Chdir(dir)

	run := func(args ...string) {
		t.Helper()
		root := newRootCmd()
		root.SetArgs(args)
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		if err := root.Execute(); err != nil {
			t.Fatalf("cmd %v: %v", args, err)
		}
	}

	// init registers the project + writes SPEC.md; then build a small spec.
	run("init")
	run("add-invariant", "always check auth")
	run("add-interface", "cmd", "init", "create db")
	run("new-feature", "f")
	run("add-task", "build it", "--feature", "1", "--cites", "V1,I.init")

	spec := filepath.Join(dir, "SPEC.md")
	before, err := os.ReadFile(spec)
	if err != nil {
		t.Fatalf("read SPEC.md: %v", err)
	}

	run("show", "V1")
	run("list", "task")
	run("refs", "V1")
	run("status")
	run("next")
	run("guide")
	run("list", "task", "--status", "x")
	run("list", "unknown")
	run("list", "goal")
	run("list", "constraint")
	run("todo")
	run("todo", "--pretty")
	run("cat")
	run("cat", "--feature", "1")
	run("projects")
	run("search", "auth")

	after, err := os.ReadFile(spec)
	if err != nil {
		t.Fatalf("re-read SPEC.md: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("a read command re-exported SPEC.md (V16: reads must not re-export)")
	}

	// check fails if a read mutated the db (db ≠ frozen SPEC.md).
	run("check")
}
