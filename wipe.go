package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// wipeFeature deletes a feature; ON DELETE CASCADE removes its goal/constraint/
// task rows (and their cite joins). Durable rows and other features are
// untouched (V4).
func wipeFeature(db *sql.DB, projectID, featureOrd int64) error {
	n, err := dbq.New(db).WipeFeature(context.Background(), dbq.WipeFeatureParams{
		ProjectID: nz(projectID), Ord: nz(featureOrd),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no feature %d in this project", featureOrd)
	}
	return nil
}

func newWipeFeatureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "wipe-feature <id>",
		Short: "delete a feature's goal/constraint/task rows (durable kept)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("bad feature id %q", args[0])
			}
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				return fmt.Sprintf("wiped feature %d", id), wipeFeature(db, pid, id)
			})
		},
	}
}
