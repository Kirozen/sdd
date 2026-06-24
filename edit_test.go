package main

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestAddResearch(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := addResearch(db, pid, "jwt", "use jose", "example.com"); err != nil {
		t.Fatalf("addResearch: %v", err)
	}
	out, _ := renderSpec(db, pid)
	if !strings.Contains(out, "R1|jwt|use jose|example.com") {
		t.Errorf("research not rendered; got:\n%s", out)
	}
}

// V12: editing a row changes its text but not its id, so citations still resolve.
func TestEditPreservesID(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	invOrd, _ := addInvariant(db, pid, "old text")
	fid, _ := addFeature(db, pid, "f")
	tid, _ := addTask(db, pid, fid, "t", []string{fmt.Sprintf("V%d", invOrd)})

	var idBefore int64
	db.QueryRow(`SELECT id FROM invariant WHERE project_id=? AND ord=?`, pid, invOrd).Scan(&idBefore)

	if err := editRow(db, pid, "invariant", strconv.FormatInt(invOrd, 10), "new text"); err != nil {
		t.Fatalf("editRow: %v", err)
	}

	var text string
	var id int64
	db.QueryRow(`SELECT id, text FROM invariant WHERE project_id=? AND ord=?`, pid, invOrd).Scan(&id, &text)
	if id != idBefore {
		t.Errorf("id changed: %d -> %d", idBefore, id)
	}
	if text != "new text" {
		t.Errorf("text = %q, want 'new text'", text)
	}
	cites, _ := taskCites(db, tid)
	if cites != fmt.Sprintf("V%d", invOrd) {
		t.Errorf("cite broke after edit: %q", cites)
	}
}

func TestEditUnknownKind(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := editRow(db, pid, "bogus", "1", "x"); err == nil {
		t.Error("edit on unknown kind succeeded")
	}
}

// V11: deprecate flips status and the export marks it; nothing is deleted.
func TestDeprecateInterface(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if _, err := addInterface(db, pid, "cmd", "old", "sig"); err != nil {
		t.Fatalf("addInterface: %v", err)
	}
	if err := deprecateInterface(db, pid, "old"); err != nil {
		t.Fatalf("deprecateInterface: %v", err)
	}
	var status string
	db.QueryRow(`SELECT status FROM interface WHERE project_id=? AND name='old'`, pid).Scan(&status)
	if status != "deprecated" {
		t.Errorf("status = %q, want deprecated", status)
	}
	if n := count(t, db, "interface"); n != 1 {
		t.Errorf("interface deleted; count = %d, want 1 (history kept)", n)
	}
	out, _ := renderSpec(db, pid)
	if !strings.Contains(out, "[deprecated]") {
		t.Errorf("export does not mark deprecated; got:\n%s", out)
	}
}

func TestDeprecateUnknownInterface(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	if err := deprecateInterface(db, pid, "nope"); err == nil {
		t.Error("deprecate unknown interface succeeded")
	}
}
