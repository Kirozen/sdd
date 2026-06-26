package sdd

import (
	"context"
	"fmt"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// addCites attaches cites to an existing task, addressing it by per-project
// ordinal (V20, V26). It reuses insertCite — the same core add-task uses — so
// cites resolve and are FK-guarded identically (V5, V74). All cites go in one
// transaction: a single bad cite (orphan, or already present → join-table PK)
// rolls the whole thing back, so a partial attach never survives.
func addCites(db dbq.DBTX, projectID, featureOrd, taskOrd int64, cites []string) error {
	taskPK, err := dbq.New(db).TaskPKByOrd(context.Background(), dbq.TaskPKByOrdParams{
		ProjectID: projectID, Ord: featureOrd, Ord_2: taskOrd,
	})
	if err != nil {
		return fmt.Errorf("no task T%d in feature F%d of this project", taskOrd, featureOrd)
	}
	for _, c := range cites {
		if err := insertCite(db, projectID, taskPK, c); err != nil {
			return err
		}
	}
	return nil
}

func newAddCiteCmd() *cobra.Command {
	var feature int
	c := &cobra.Command{
		Use:   "add-cite <T-ord> --feature <f> <cite> [<cite>...]",
		Short: "attach cites (V<n>/I.<name>) to an existing task",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ord, err := ordArg(args[0], "T")
			if err != nil {
				return err
			}
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				if err := addCites(db, pid, int64(feature), int64(ord), args[1:]); err != nil {
					return "", err
				}
				return fmt.Sprintf("T%d @F%d += %v", ord, feature, args[1:]), nil
			})
		},
	}
	c.Flags().IntVar(&feature, "feature", 0, "feature ordinal owning the task (required)")
	c.MarkFlagRequired("feature")
	return c
}
