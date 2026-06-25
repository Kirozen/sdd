package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// todoRows renders the project's unfinished tasks (status != x) as TSV — one
// line per task, fixed columns [F-ord, F-name, T-ord, status, cites, text]
// separated by TAB, no header, text last (V70). The text is the RAW task text:
// no pipe-escaping (that belongs to the SPEC.md table, not a TSV cell), so the
// only forbidden character in a value is TAB. cites reuses taskCites (V70). A
// fully-done feature contributes no rows (V68); zero pending tasks ⇒ no lines.
// Read-pure (V69), project-scoped (V20).
func todoRows(db *sql.DB, projectID int64) ([]string, error) {
	rows, err := dbq.New(db).PendingTasks(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		cites, err := taskCites(db, r.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, strings.Join([]string{
			fmt.Sprintf("F%d", int(r.FeatureOrd)),
			r.FeatureName,
			fmt.Sprintf("T%d", int(r.Ord)),
			r.Status,
			cites,
			r.Text,
		}, "\t"))
	}
	return out, nil
}

// todoPretty is the grouped human view (--pretty): a feature header followed by
// its indented, aligned unfinished-task rows, mirroring `list --pretty` (V70).
// PendingTasks is ordered by feature then task, so a single pass groups them.
func todoPretty(db *sql.DB, projectID int64) ([]string, error) {
	rows, err := dbq.New(db).PendingTasks(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	var out []string
	for i := 0; i < len(rows); {
		fOrd, fName := rows[i].FeatureOrd, rows[i].FeatureName
		var refs, bodies []string
		for i < len(rows) && rows[i].FeatureOrd == fOrd {
			r := rows[i]
			cites, err := taskCites(db, r.ID)
			if err != nil {
				return nil, err
			}
			body := fmt.Sprintf("%s  %s", statusGlyph(r.Status), r.Text)
			if cites != "-" {
				body += "  → " + cites
			}
			refs = append(refs, fmt.Sprintf("T%d", int(r.Ord)))
			bodies = append(bodies, body)
			i++
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, fmt.Sprintf("FEATURE %d — %s", int(fOrd), fName))
		out = append(out, alignRows(refs, bodies)...)
	}
	return out, nil
}

func newTodoCmd() *cobra.Command {
	var pretty bool
	c := &cobra.Command{
		Use:   "todo",
		Short: "list unfinished tasks (status != x) as TSV; --pretty for a grouped human view",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			var lines []string
			if pretty {
				lines, err = todoPretty(db, pid)
			} else {
				lines, err = todoRows(db, pid)
			}
			if err != nil {
				return err
			}
			for _, l := range lines {
				fmt.Println(l)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&pretty, "pretty", false, "grouped human-readable view")
	return c
}
