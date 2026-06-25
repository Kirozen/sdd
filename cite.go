package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// addCites attaches cites to an existing task, addressing it by per-project
// ordinal (V20, V26). It reuses insertCite — the same core add-task uses — so
// cites resolve and are FK-guarded identically (V5, V74). All cites go in one
// transaction: a single bad cite (orphan, or already present → join-table PK)
// rolls the whole thing back, so a partial attach never survives.
func addCites(db *sql.DB, projectID, taskOrd int64, cites []string) error {
	taskPK, err := dbq.New(db).TaskPKByOrd(context.Background(), dbq.TaskPKByOrdParams{
		ProjectID: projectID, Ord: taskOrd,
	})
	if err != nil {
		return fmt.Errorf("no task T%d in this project", taskOrd)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, c := range cites {
		if err := insertCite(tx, projectID, taskPK, c); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func newAddCiteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-cite <T-ord> <cite> [<cite>...]",
		Short: "attach cites (V<n>/I.<name>) to an existing task",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ord, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("bad task id %q", args[0])
			}
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				if err := addCites(db, pid, ord, args[1:]); err != nil {
					return "", err
				}
				return fmt.Sprintf("T%d += %v", ord, args[1:]), nil
			})
		},
	}
}
