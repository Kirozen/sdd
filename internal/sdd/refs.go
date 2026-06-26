package sdd

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbq "github.com/kirozen/sdd/internal/db"
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
		rows, err := q.CitersOfIface(ctx, iid)
		if err != nil {
			return nil, err
		}
		citers := make([]taskCiter, len(rows))
		for i, r := range rows {
			citers[i] = taskCiter{r.FeatureOrd, r.TaskOrd}
		}
		return taskCiterLines(db, projectID, citers)

	case strings.HasPrefix(ref, "V"):
		ord, err := refID(ref)
		if err != nil {
			return nil, err
		}
		iid, err := q.InvariantIDByOrd(ctx, dbq.InvariantIDByOrdParams{ProjectID: projectID, Ord: int64(ord)})
		if err != nil {
			return nil, fmt.Errorf("no invariant %q in this project", ref)
		}
		rows, err := q.TaskCitersOfInv(ctx, iid)
		if err != nil {
			return nil, err
		}
		citers := make([]taskCiter, len(rows))
		for i, r := range rows {
			citers[i] = taskCiter{r.FeatureOrd, r.TaskOrd}
		}
		tasks, err := taskCiterLines(db, projectID, citers)
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

// taskCiter pairs a citing task's per-feature ordinal with the feature that owns
// it, so the two stay aligned (V117) instead of riding two parallel slices.
type taskCiter struct{ featOrd, taskOrd int64 }

// taskCiterLines renders each citing task as "F<f> " + its §T line (V18), so a
// per-feature T<n> (V117) stays addressable from refs output — mirrors how search
// carries the feature (V118).
func taskCiterLines(db *sql.DB, projectID int64, citers []taskCiter) ([]string, error) {
	out := make([]string, 0, len(citers))
	for _, c := range citers {
		line, err := showRef(db, projectID, fmt.Sprintf("T%d", c.taskOrd), c.featOrd)
		if err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("F%d %s", c.featOrd, line))
	}
	return out, nil
}

// citerLines renders each non-task citer ordinal (bugs, B) as a "<prefix><ord>"
// ref through showRef (reusing V18 formatting, scoped to projectID). Durable ords
// are project-scoped, so featureOrd is irrelevant here (passed 0).
func citerLines(db *sql.DB, projectID int64, ords []int64, prefix string) ([]string, error) {
	var out []string
	for _, o := range ords {
		line, err := showRef(db, projectID, fmt.Sprintf("%s%d", prefix, o), 0)
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
