package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// refsTo returns the reverse citations of a ref within the current project — who
// points at it — as caveman lines. Only invariants and interfaces are citation
// targets: an invariant is cited by tasks and fixed by bugs; an interface is
// cited by tasks. Each citer is rendered through showRef, so lines match
// SPEC.md (V18). Read-pure (V16), scoped (V20). Non-target/unknown ref → error
// (V17); an uncited target → no lines.
func refsTo(db *sql.DB, projectID int64, ref string) ([]string, error) {
	ctx := context.Background()
	q := dbq.New(db)
	switch {
	case strings.HasPrefix(ref, "I."):
		name := ref[2:]
		iid, err := q.InterfaceIDByName(ctx, dbq.InterfaceIDByNameParams{ProjectID: projectID, Name: name})
		if err != nil {
			return nil, fmt.Errorf("no interface %q in this project", ref)
		}
		ords, err := q.CitersOfIface(ctx, iid)
		if err != nil {
			return nil, err
		}
		return citerLines(db, projectID, ords, "T")

	case strings.HasPrefix(ref, "V"):
		ord, err := refID(ref)
		if err != nil {
			return nil, err
		}
		iid, err := q.InvariantIDByOrd(ctx, dbq.InvariantIDByOrdParams{ProjectID: projectID, Ord: int64(ord)})
		if err != nil {
			return nil, fmt.Errorf("no invariant %q in this project", ref)
		}
		taskOrds, err := q.TaskCitersOfInv(ctx, iid)
		if err != nil {
			return nil, err
		}
		tasks, err := citerLines(db, projectID, taskOrds, "T")
		if err != nil {
			return nil, err
		}
		bugOrds, err := q.BugCitersOfInv(ctx, iid)
		if err != nil {
			return nil, err
		}
		bugs, err := citerLines(db, projectID, bugOrds, "B")
		if err != nil {
			return nil, err
		}
		return append(tasks, bugs...), nil

	default:
		return nil, fmt.Errorf("refs needs a cite target V<n> or I.<name>, got %q", ref)
	}
}

// citerLines renders each citer ordinal as a "<prefix><ord>" ref through showRef
// (reusing V18 formatting, scoped to projectID). ords come from the generated
// citer queries as plain int64 (the ord override, non-null by V26).
func citerLines(db *sql.DB, projectID int64, ords []int64, prefix string) ([]string, error) {
	var out []string
	for _, o := range ords {
		line, err := showRef(db, projectID, fmt.Sprintf("%s%d", prefix, o))
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
