package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// refsTo returns the reverse citations of a ref — who points at it — as caveman
// lines. Only invariants and interfaces are citation targets: an invariant is
// cited by tasks and fixed by bugs; an interface is cited by tasks. Each citer
// is rendered through showRef, so lines match SPEC.md (V18). Read-pure (V16).
// A non-target or unknown ref errors (V17); an uncited target → no lines.
func refsTo(db *sql.DB, ref string) ([]string, error) {
	switch {
	case strings.HasPrefix(ref, "I."):
		name := ref[2:]
		var iid int
		if err := db.QueryRow(`SELECT id FROM interface WHERE name=?`, name).Scan(&iid); err != nil {
			return nil, fmt.Errorf("no interface %q", ref)
		}
		return citerLines(db, `SELECT task_id FROM task_cites_iface WHERE iface_id=? ORDER BY task_id`, iid, "T")

	case strings.HasPrefix(ref, "V"):
		id, err := refID(ref)
		if err != nil {
			return nil, err
		}
		var x int
		if err := db.QueryRow(`SELECT id FROM invariant WHERE id=?`, id).Scan(&x); err != nil {
			return nil, fmt.Errorf("no invariant %q", ref)
		}
		tasks, err := citerLines(db, `SELECT task_id FROM task_cites_inv WHERE inv_id=? ORDER BY task_id`, id, "T")
		if err != nil {
			return nil, err
		}
		bugs, err := citerLines(db, `SELECT bug_id FROM bug_fix WHERE inv_id=? ORDER BY bug_id`, id, "B")
		if err != nil {
			return nil, err
		}
		return append(tasks, bugs...), nil

	default:
		return nil, fmt.Errorf("refs needs a cite target V<n> or I.<name>, got %q", ref)
	}
}

// citerLines runs an id query then renders each id as a "<prefix><id>" ref line
// through showRef (reusing V18 formatting). The cursor is drained before showRef
// re-queries.
func citerLines(db *sql.DB, query string, arg int, prefix string) ([]string, error) {
	rows, err := db.Query(query, arg)
	if err != nil {
		return nil, err
	}
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []string
	for _, id := range ids {
		line, err := showRef(db, fmt.Sprintf("%s%d", prefix, id))
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
			db, err := openProjectDB()
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := refsTo(db, args[0])
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
