package main

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// setTaskStatus updates a task's status. The schema CHECK rejects any value
// outside {.,~,x} (V10); an unknown task id is an error.
func setTaskStatus(db *sql.DB, taskID int64, status string) error {
	res, err := db.Exec(`UPDATE task SET status=? WHERE id=?`, status, taskID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no task with id %d", taskID)
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
			return runMutation(func(db *sql.DB) (string, error) {
				return fmt.Sprintf("T%d → %s", id, status), setTaskStatus(db, id, status)
			})
		},
	}
	c.Flags().StringVar(&status, "status", "", "one of . ~ x (required)")
	c.MarkFlagRequired("status")
	return c
}
