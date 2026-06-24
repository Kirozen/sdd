package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestAddResearch(t *testing.T) {
	db := openTestDB(t)
	if _, err := addResearch(db, "jwt", "use jose", "example.com"); err != nil {
		t.Fatalf("addResearch: %v", err)
	}
	out, _ := renderSpec(db)
	if !strings.Contains(out, "R1|jwt|use jose|example.com") {
		t.Errorf("research not rendered; got:\n%s", out)
	}
}

// V12: editing a row changes its text but not its id, so citations still resolve.
func TestEditPreservesID(t *testing.T) {
	db := openTestDB(t)
	invID, _ := addInvariant(db, "old text")
	fid, _ := addFeature(db, "f")
	tid, _ := addTask(db, fid, "t", []string{fmt.Sprintf("V%d", invID)})

	if err := editRow(db, "invariant", invID, "new text"); err != nil {
		t.Fatalf("editRow: %v", err)
	}

	var text string
	var id int64
	db.QueryRow(`SELECT id, text FROM invariant WHERE id=?`, invID).Scan(&id, &text)
	if id != invID {
		t.Errorf("id changed: %d -> %d", invID, id)
	}
	if text != "new text" {
		t.Errorf("text = %q, want 'new text'", text)
	}
	cites, _ := taskCites(db, int(tid))
	if cites != fmt.Sprintf("V%d", invID) {
		t.Errorf("cite broke after edit: %q", cites)
	}
}

func TestEditUnknownKind(t *testing.T) {
	db := openTestDB(t)
	if err := editRow(db, "bogus", 1, "x"); err == nil {
		t.Error("edit on unknown kind succeeded")
	}
}

// V11: deprecate flips status and the export marks it; nothing is deleted.
func TestDeprecateInterface(t *testing.T) {
	db := openTestDB(t)
	id, _ := addInterface(db, "cmd", "old", "sig")
	if err := deprecateInterface(db, id); err != nil {
		t.Fatalf("deprecateInterface: %v", err)
	}
	var status string
	db.QueryRow(`SELECT status FROM interface WHERE id=?`, id).Scan(&status)
	if status != "deprecated" {
		t.Errorf("status = %q, want deprecated", status)
	}
	if n := count(t, db, "interface"); n != 1 {
		t.Errorf("interface deleted; count = %d, want 1 (history kept)", n)
	}
	out, _ := renderSpec(db)
	if !strings.Contains(out, "[deprecated]") {
		t.Errorf("export does not mark deprecated; got:\n%s", out)
	}
}

func TestDeprecateUnknownInterface(t *testing.T) {
	db := openTestDB(t)
	if err := deprecateInterface(db, 999); err == nil {
		t.Error("deprecate unknown interface succeeded")
	}
}
