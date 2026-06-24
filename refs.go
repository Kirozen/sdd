package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// refsTo returns the reverse citations of a ref within the current project — who
// points at it — as caveman lines. Only invariants and interfaces are citation
// targets: an invariant is cited by tasks and fixed by bugs; an interface is
// cited by tasks. Each citer is rendered through showRef, so lines match
// SPEC.md (V18). Read-pure (V16), scoped (V20). Non-target/unknown ref → error
// (V17); an uncited target → no lines.
func refsTo(db *sql.DB, projectID int64, ref string) ([]string, error) {
	switch {
	case strings.HasPrefix(ref, "I."):
		name := ref[2:]
		var iid int64
		if err := db.QueryRow(`SELECT id FROM interface WHERE project_id=? AND name=?`, projectID, name).Scan(&iid); err != nil {
			return nil, fmt.Errorf("no interface %q in this project", ref)
		}
		return citerLines(db, projectID, `SELECT t.ord FROM task_cites_iface j JOIN task t ON t.id=j.task_id WHERE j.iface_id=? ORDER BY t.ord`, iid, "T")

	case strings.HasPrefix(ref, "V"):
		ord, err := refID(ref)
		if err != nil {
			return nil, err
		}
		var iid int64
		if err := db.QueryRow(`SELECT id FROM invariant WHERE project_id=? AND ord=?`, projectID, ord).Scan(&iid); err != nil {
			return nil, fmt.Errorf("no invariant %q in this project", ref)
		}
		tasks, err := citerLines(db, projectID, `SELECT t.ord FROM task_cites_inv j JOIN task t ON t.id=j.task_id WHERE j.inv_id=? ORDER BY t.ord`, iid, "T")
		if err != nil {
			return nil, err
		}
		bugs, err := citerLines(db, projectID, `SELECT b.ord FROM bug_fix j JOIN bug b ON b.id=j.bug_id WHERE j.inv_id=? ORDER BY b.ord`, iid, "B")
		if err != nil {
			return nil, err
		}
		return append(tasks, bugs...), nil

	default:
		return nil, fmt.Errorf("refs needs a cite target V<n> or I.<name>, got %q", ref)
	}
}

// citerLines runs an ordinal query then renders each "<prefix><ord>" ref through
// showRef (reusing V18 formatting, scoped to projectID).
func citerLines(db *sql.DB, projectID int64, query string, arg int64, prefix string) ([]string, error) {
	rows, err := db.Query(query, arg)
	if err != nil {
		return nil, err
	}
	var ords []int
	for rows.Next() {
		var ord int
		if err := rows.Scan(&ord); err != nil {
			rows.Close()
			return nil, err
		}
		ords = append(ords, ord)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []string
	for _, ord := range ords {
		line, err := showRef(db, projectID, fmt.Sprintf("%s%d", prefix, ord))
		if err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, nil
}

func newRefsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refs <ref>",
		Short: "print rows citing a ref (V<n>/I.<name>), read-only",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := refsTo(db, pid, args[0])
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
