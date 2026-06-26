package sdd

import (
	"context"
	"fmt"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// setTaskStatus updates a task's status, addressing it by (feature ord, task ord)
// since task ords are per-feature now (V117). The schema CHECK rejects any value
// outside {.,~,x} (V10); an unknown task is an error.
func setTaskStatus(db dbq.DBTX, projectID, featureOrd, taskOrd int64, status string) error {
	n, err := dbq.New(db).SetTaskStatus(context.Background(), dbq.SetTaskStatusParams{
		Status: status, Ord: taskOrd, ProjectID: projectID, Ord_2: featureOrd,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no task T%d in feature F%d of this project", taskOrd, featureOrd)
	}
	return nil
}

func newSetTaskCmd() *cobra.Command {
	var status string
	var feature int
	c := &cobra.Command{
		Use:   "set-task <T-ord> --feature <f>",
		Short: "set a task status (. todo / ~ wip / x done)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ord, err := ordArg(args[0], "T")
			if err != nil {
				return err
			}
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				return fmt.Sprintf("T%d @F%d → %s", int64(ord), feature, status), setTaskStatus(db, pid, int64(feature), int64(ord), status)
			})
		},
	}
	c.Flags().StringVar(&status, "status", "", "one of . ~ x (required)")
	c.MarkFlagRequired("status")
	c.Flags().IntVar(&feature, "feature", 0, "feature ordinal owning the task (required)")
	c.MarkFlagRequired("feature")
	return c
}
