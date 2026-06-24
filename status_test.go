package main

import (
	"fmt"
	"strings"
	"testing"
)

// I.status + V19: status reports per-feature task counts and flags every task
// that cites a deprecated interface.
func TestStatusCountsAndDeprecatedWarn(t *testing.T) {
	db := openTestDB(t)
	iid, err := addInterface(db, "cmd", "olditer", "sig")
	if err != nil {
		t.Fatalf("addInterface: %v", err)
	}
	fid, err := addFeature(db, "f")
	if err != nil {
		t.Fatalf("addFeature: %v", err)
	}
	tid, err := addTask(db, fid, "uses old", []string{"I.olditer"})
	if err != nil {
		t.Fatalf("addTask: %v", err)
	}

	// counts: the one task sits at the default status "."
	lines, err := statusReport(db)
	if err != nil {
		t.Fatalf("statusReport: %v", err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, fmt.Sprintf("F%d f  x:0 ~:0 .:1", fid)) {
		t.Errorf("counts wrong:\n%s", joined)
	}
	if strings.Contains(joined, "deprecated") {
		t.Errorf("warned on an active interface:\n%s", joined)
	}

	// deprecate the cited interface → the warning appears (V19)
	if err := deprecateInterface(db, iid); err != nil {
		t.Fatalf("deprecateInterface: %v", err)
	}
	lines, err = statusReport(db)
	if err != nil {
		t.Fatalf("statusReport after deprecate: %v", err)
	}
	joined = strings.Join(lines, "\n")
	if !strings.Contains(joined, fmt.Sprintf("! T%d cites deprecated I.olditer", tid)) {
		t.Errorf("missing deprecated-cite warning (V19):\n%s", joined)
	}
}
