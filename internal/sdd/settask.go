package sdd

import (
	"context"
	"fmt"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// setTaskStatus updates a task's status, addressing it by per-project ordinal.
// The schema CHECK rejects any value outside {.,~,x} (V10); an unknown task is
// an error.
func setTaskStatus(db dbq.DBTX, projectID, taskOrd int64, status string) error {
	n, err := dbq.New(db).SetTaskStatus(context.Background(), dbq.SetTaskStatusParams{
		Status: status, Ord: taskOrd, ProjectID: projectID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no task T%d in this project", taskOrd)
	}
	return nil
}

func newSetTaskCmd() *cobra.Command {
	var status string
	c := &cobra.Command{
		Use:   "set-task <id>",
		Short: "set a task status (. todo / ~ wip / x done)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ord, err := ordArg(args[0], "T")
			if err != nil {
				return err
			}
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				return fmt.Sprintf("T%d → %s", int64(ord), status), setTaskStatus(db, pid, int64(ord), status)
			})
		},
	}
	c.Flags().StringVar(&status, "status", "", "one of . ~ x (required)")
	c.MarkFlagRequired("status")
	return c
}
