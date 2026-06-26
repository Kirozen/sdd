package sdd

import (
	"context"
	"fmt"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// rmTask hard-deletes an ephemeral task by its per-project ordinal; its
// task_cites_inv/iface rows cascade away (task_id ON DELETE CASCADE, 001_base).
// A task is never a cite target, so rm-task is never refused (V96). Scoped to the
// project via the feature join (V20). Unknown ord → error (V17). The survivors'
// ordinals are never renumbered (V97).
func rmTask(db dbq.DBTX, projectID, featureOrd, taskOrd int64) error {
	n, err := dbq.New(db).DeleteTaskByOrd(context.Background(), dbq.DeleteTaskByOrdParams{
		Ord: taskOrd, ProjectID: projectID, Ord_2: featureOrd,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no task T%d in feature F%d of this project", taskOrd, featureOrd)
	}
	return nil
}

func newRmTaskCmd() *cobra.Command {
	var feature int
	c := &cobra.Command{
		Use:   "rm-task <T-ord> --feature <f>",
		Short: "hard-delete a task (its cites cascade); re-export",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ord, err := ordArg(args[0], "T")
			if err != nil {
				return err
			}
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				return fmt.Sprintf("removed T%d @F%d", ord, feature), rmTask(db, pid, int64(feature), int64(ord))
			})
		},
	}
	c.Flags().IntVar(&feature, "feature", 0, "feature ordinal owning the task (required)")
	c.MarkFlagRequired("feature")
	return c
}
