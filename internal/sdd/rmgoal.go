package sdd

import (
	"context"
	"fmt"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// rmGoal/rmConstraint hard-delete the n-th goal/constraint (1-based, ORDER BY id
// as rendered in SPEC.md) of the feature addressed by its ordinal, scoped to the
// current project (V20, V98). Addressing is by POSITION, never by global PK (V26).
// Goals/constraints are never cite targets, so these are never refused. n==0 rows
// (no such feature in this project, or n out of range) → error (V17). Re-exports
// (V8). Both share posArgs/runMutation; only the query differs.

func rmGoal(db dbq.DBTX, projectID, featureOrd int64, n int) error {
	rows, err := dbq.New(db).DeleteGoalByPosition(context.Background(), dbq.DeleteGoalByPositionParams{
		ProjectID: projectID, Ord: featureOrd, Offset: int64(n - 1),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("no goal #%d in feature %d of this project", n, featureOrd)
	}
	return nil
}

func rmConstraint(db dbq.DBTX, projectID, featureOrd int64, n int) error {
	rows, err := dbq.New(db).DeleteConstraintByPosition(context.Background(), dbq.DeleteConstraintByPositionParams{
		ProjectID: projectID, Ord: featureOrd, Offset: int64(n - 1),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("no constraint #%d in feature %d of this project", n, featureOrd)
	}
	return nil
}

// posArgs parses the shared <F-ord> <n> argument pair (n is 1-based).
func posArgs(args []string) (featureOrd int64, n int, err error) {
	fo, err := ordArg(args[0], "F")
	if err != nil {
		return 0, 0, err
	}
	pos, err := ordArg(args[1], "")
	if err != nil {
		return 0, 0, err
	}
	if pos < 1 {
		return 0, 0, fmt.Errorf("position must be >= 1, got %d", pos)
	}
	return int64(fo), pos, nil
}

func newRmGoalCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm-goal <F-ord> <n>",
		Short: "hard-delete the n-th goal (1-based) of a feature; re-export",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fo, n, err := posArgs(args)
			if err != nil {
				return err
			}
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				return fmt.Sprintf("removed goal #%d of feature %d", n, fo), rmGoal(db, pid, fo, n)
			})
		},
	}
}

func newRmConstraintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm-constraint <F-ord> <n>",
		Short: "hard-delete the n-th constraint (1-based) of a feature; re-export",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fo, n, err := posArgs(args)
			if err != nil {
				return err
			}
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				return fmt.Sprintf("removed constraint #%d of feature %d", n, fo), rmConstraint(db, pid, fo, n)
			})
		},
	}
}
