package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// setTaskStatus updates a task's status, addressing it by per-project ordinal.
// The schema CHECK rejects any value outside {.,~,x} (V10); an unknown task is
// an error.
func setTaskStatus(db *sql.DB, projectID, taskOrd int64, status string) error {
	n, err := dbq.New(db).SetTaskStatus(context.Background(), dbq.SetTaskStatusParams{
		Status: status, Ord: nz(taskOrd), ProjectID: nz(projectID),
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
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("bad task id %q", args[0])
			}
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				return fmt.Sprintf("T%d → %s", id, status), setTaskStatus(db, pid, id, status)
			})
		},
	}
	c.Flags().StringVar(&status, "status", "", "one of . ~ x (required)")
	c.MarkFlagRequired("status")
	return c
}
