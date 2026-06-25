package main

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

// coverReport lists every project invariant with the test(s) that prove it, or a
// `!` flag when none does (V43). It makes the "build cannot regress it" promise
// auditable: uncovered invariants are exactly the ones no test guards. Read-pure
// (V16, no mutation/re-export); scoped to the project (V20).
func coverReport(db *sql.DB, projectID int64) ([]string, error) {
	rows, err := db.Query(`
		SELECT i.ord, i.text, COALESCE(GROUP_CONCAT(t.name, ', '), '')
		FROM invariant i LEFT JOIN test t ON t.invariant_id = i.id
		WHERE i.project_id = ?
		GROUP BY i.id ORDER BY i.ord`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	var covered, total int
	for rows.Next() {
		var ord int
		var text, names string
		if err := rows.Scan(&ord, &text, &names); err != nil {
			return nil, err
		}
		total++
		if names == "" {
			out = append(out, fmt.Sprintf("! V%d aucun test — %s", ord, snippet(text)))
		} else {
			covered++
			out = append(out, fmt.Sprintf("V%d ✓ %s", ord, names))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out = append(out, fmt.Sprintf("gardés: %d/%d invariants", covered, total))
	return out, nil
}

// snippet trims an invariant's text to a one-glance label for the cover report.
func snippet(s string) string {
	r := []rune(s)
	if len(r) > 50 {
		return string(r[:50]) + "…"
	}
	return s
}

func newCoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cover",
		Short: "per-invariant proving test(s), or `!` if none; read-only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := coverReport(db, pid)
			if err != nil {
				return err
			}
			for _, l := range lines {
				fmt.Println(l)
			}
			return nil
		},
	}
}
