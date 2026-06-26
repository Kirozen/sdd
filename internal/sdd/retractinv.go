package sdd

import (
	"context"
	"fmt"
	"strings"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// retractInvariant hard-deletes a durable invariant by its per-project ordinal
// (V20). It pre-checks the citers (tasks via task_cites_inv, bugs via bug_fix —
// both NO ACTION FKs) and refuses with the citer list BEFORE attempting the
// DELETE, so the user sees an actionable message, never a raw FK error (V95/V5).
// Proving tests are an exception: test.invariant_id is ON DELETE CASCADE
// (003_test.sql), so they cannot block — instead they are announced on stdout
// before the cascade, never lost silently (V95/V42). Re-exports (V8). The
// survivors' ordinals are never renumbered (V97).
func retractInvariant(db dbq.DBTX, projectID int64, ord int64) (string, error) {
	ctx := context.Background()
	q := dbq.New(db)

	iid, err := q.InvariantIDByOrd(ctx, dbq.InvariantIDByOrdParams{ProjectID: projectID, Ord: ord})
	if err != nil {
		return "", fmt.Errorf("no invariant V%d in this project", ord)
	}

	taskOrds, err := q.TaskCitersOfInv(ctx, iid)
	if err != nil {
		return "", err
	}
	bugOrds, err := q.BugCitersOfInv(ctx, iid)
	if err != nil {
		return "", err
	}
	if len(taskOrds)+len(bugOrds) > 0 {
		var cited []string
		for _, o := range taskOrds {
			cited = append(cited, fmt.Sprintf("T%d@F%d", o.TaskOrd, o.FeatureOrd))
		}
		for _, o := range bugOrds {
			cited = append(cited, fmt.Sprintf("B%d", o))
		}
		return "", fmt.Errorf("cannot retract V%d: cited by %s — retract those first", ord, strings.Join(cited, ", "))
	}

	// Uncited: safe to delete. Announce any proving tests we are about to cascade.
	var msg string
	tests, err := q.TestNamesByInvariantOrd(ctx, dbq.TestNamesByInvariantOrdParams{ProjectID: projectID, Ord: ord})
	if err != nil {
		return "", err
	}
	if len(tests) > 0 {
		fmt.Printf("V%d: removing %d proving test(s): %s\n", ord, len(tests), strings.Join(tests, ", "))
		msg = fmt.Sprintf("retracted V%d (+%d tests)", ord, len(tests))
	} else {
		msg = fmt.Sprintf("retracted V%d", ord)
	}

	if _, err := q.DeleteInvariantByOrd(ctx, dbq.DeleteInvariantByOrdParams{ProjectID: projectID, Ord: ord}); err != nil {
		return "", err
	}
	return msg, nil
}

func newRetractInvariantCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retract-invariant <V-ord>",
		Short: "hard-delete an invariant; refuses (listing citers) if cited; re-export",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ord, err := ordArg(args[0], "V")
			if err != nil {
				return err
			}
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				return retractInvariant(db, pid, int64(ord))
			})
		},
	}
}
