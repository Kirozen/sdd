package main

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

// statusReport is the current project's health view: one line per feature with
// its task counts by status, then one warning per task that cites a deprecated
// interface (V19). Read-pure (V16), scoped (V20); it does not touch check's
// drift contract (V6).
func statusReport(db *sql.DB, projectID int64) ([]string, error) {
	type feat struct {
		pk   int64
		ord  int
		name string
	}
	var feats []feat
	rows, err := db.Query(`SELECT id, ord, name FROM feature WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var f feat
		if err := rows.Scan(&f.pk, &f.ord, &f.name); err != nil {
			rows.Close()
			return nil, err
		}
		feats = append(feats, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []string
	for _, f := range feats {
		c := map[string]int{}
		cr, err := db.Query(`SELECT status, count(*) FROM task WHERE feature_id=? GROUP BY status`, f.pk)
		if err != nil {
			return nil, err
		}
		for cr.Next() {
			var s string
			var n int
			if err := cr.Scan(&s, &n); err != nil {
				cr.Close()
				return nil, err
			}
			c[s] = n
		}
		cr.Close()
		if err := cr.Err(); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("F%d %s  x:%d ~:%d .:%d", f.ord, f.name, c["x"], c["~"], c["."]))
	}

	// V19: flag every task in this project citing a deprecated interface.
	wr, err := db.Query(`SELECT t.ord, i.name
		FROM task_cites_iface j
		JOIN interface i ON i.id = j.iface_id
		JOIN task t ON t.id = j.task_id
		JOIN feature f ON f.id = t.feature_id
		WHERE f.project_id = ? AND i.status = 'deprecated'
		ORDER BY t.ord, i.name`, projectID)
	if err != nil {
		return nil, err
	}
	defer wr.Close()
	for wr.Next() {
		var ord int
		var name string
		if err := wr.Scan(&ord, &name); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("! T%d cites deprecated I.%s", ord, name))
	}
	return out, wr.Err()
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "per-feature task counts + deprecated-cite warnings, read-only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := statusReport(db, pid)
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
