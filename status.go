package main

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

// statusReport is the health view: one line per feature with its task counts by
// status, then one warning per task that cites a deprecated interface (V19).
// Read-pure (V16); it does not touch check's drift contract (V6).
func statusReport(db *sql.DB) ([]string, error) {
	type feat struct {
		id   int
		name string
	}
	var feats []feat
	rows, err := db.Query(`SELECT id, name FROM feature ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var f feat
		if err := rows.Scan(&f.id, &f.name); err != nil {
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
		cr, err := db.Query(`SELECT status, count(*) FROM task WHERE feature_id=? GROUP BY status`, f.id)
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
		out = append(out, fmt.Sprintf("F%d %s  x:%d ~:%d .:%d", f.id, f.name, c["x"], c["~"], c["."]))
	}

	// V19: flag every task citing a deprecated interface.
	wr, err := db.Query(`SELECT t.id, i.name
		FROM task_cites_iface j
		JOIN interface i ON i.id = j.iface_id
		JOIN task t ON t.id = j.task_id
		WHERE i.status = 'deprecated'
		ORDER BY t.id, i.name`)
	if err != nil {
		return nil, err
	}
	defer wr.Close()
	for wr.Next() {
		var tid int
		var name string
		if err := wr.Scan(&tid, &name); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("! T%d cites deprecated I.%s", tid, name))
	}
	return out, wr.Err()
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "per-feature task counts + deprecated-cite warnings, read-only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openProjectDB()
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := statusReport(db)
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
