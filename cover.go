package main

import (
	"context"
	"database/sql"
	"fmt"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// coverReport lists every project invariant with the test(s) that prove it, or a
// `!` flag when none does (V43). It makes the "build cannot regress it" promise
// auditable: uncovered invariants are exactly the ones no test guards. Read-pure
// (V16, no mutation/re-export); scoped to the project (V20).
func coverReport(db *sql.DB, projectID int64) ([]string, error) {
	rows, err := dbq.New(db).InvariantCoverage(context.Background(), nz(projectID))
	if err != nil {
		return nil, err
	}
	var out []string
	var covered, total int
	for _, r := range rows {
		total++
		if r.Tests == "" {
			out = append(out, fmt.Sprintf("! V%d aucun test — %s", int(r.Ord.Int64), snippet(r.Text)))
		} else {
			covered++
			out = append(out, fmt.Sprintf("V%d ✓ %s", int(r.Ord.Int64), r.Tests))
		}
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
