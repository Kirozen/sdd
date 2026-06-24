package main

import (
	"bytes"
	"os"
	"testing"
)

// V16: read commands are pure — running show/list/refs/status must neither
// re-export SPEC.md nor mutate the db. Driven end-to-end through the cobra root
// (the same path the CLI uses). The two assertions together are exhaustive: if a
// read re-exported, SPEC.md bytes change; if a read mutated the db without
// re-exporting, `check` fails (db ≠ frozen SPEC.md). Passing both ⇒ no write.
func TestReadCommandsArePure(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := runInit("."); err != nil {
		t.Fatalf("init: %v", err)
	}
	db, err := openProjectDB()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := seedDB(db, parseSpec(fixtureSpec), "f", false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := exportSpec(db, specPath); err != nil {
		t.Fatalf("export: %v", err)
	}
	db.Close()

	before, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read SPEC.md: %v", err)
	}

	run := func(args ...string) {
		root := newRootCmd()
		root.SetArgs(args)
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		if err := root.Execute(); err != nil {
			t.Fatalf("cmd %v: %v", args, err)
		}
	}
	run("show", "V1")
	run("list", "task")
	run("refs", "V1")
	run("status")

	after, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("re-read SPEC.md: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("a read command re-exported SPEC.md (V16: reads must not re-export)")
	}

	db2, err := openProjectDB()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()
	if err := checkSpec(db2, specPath); err != nil {
		t.Errorf("check fails after reads — a read mutated the db (V16): %v", err)
	}
}
