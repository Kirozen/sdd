package sdd

import (
	"os"
	"strings"
	"testing"
)

// V101: no UPDATE/DELETE on a project-owned table (every table except `project`,
// the identity root) may address its rows without a project constraint. Every
// such mutation must reference project_id — directly or via a feature join; a
// bare `WHERE id = ?` leaks across the shared store. This is the B6 trap, where
// EditGoal/EditConstraint edited by global PK and silently overwrote another
// project's row (query.sql:137/140). Guards the whole class, not just
// goal/constraint (model: TestSQLFilesAreASCII, V56).
func TestNoBarePKMutations(t *testing.T) {
	b, err := os.ReadFile("../db/query.sql")
	if err != nil {
		t.Fatalf("read query.sql: %v", err)
	}
	for stmt := range strings.SplitSeq(string(b), ";") {
		// Strip comment tails so keywords/tables are read from SQL only.
		var lines []string
		for ln := range strings.SplitSeq(stmt, "\n") {
			if i := strings.Index(ln, "--"); i >= 0 {
				ln = ln[:i]
			}
			lines = append(lines, ln)
		}
		sql := strings.TrimSpace(strings.Join(lines, " "))
		fields := strings.Fields(sql)
		if len(fields) < 2 {
			continue
		}
		var table string
		switch strings.ToUpper(fields[0]) {
		case "UPDATE":
			table = fields[1]
		case "DELETE":
			if strings.ToUpper(fields[1]) == "FROM" && len(fields) >= 3 {
				table = fields[2]
			}
		default:
			continue
		}
		table = strings.Trim(table, `"`)
		if table == "" || table == "project" {
			continue
		}
		if !strings.Contains(sql, "project_id") {
			t.Errorf("mutation on project-owned table %q lacks a project_id constraint (B6 class): %s", table, sql)
		}
	}
}
