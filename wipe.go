package main

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// wipeFeature deletes a feature; ON DELETE CASCADE removes its goal/constraint/
// task rows (and their cite joins). Durable rows and other features are
// untouched (V4).
func wipeFeature(db *sql.DB, featureID int64) error {
	res, err := db.Exec(`DELETE FROM feature WHERE id=?`, featureID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no feature with id %d", featureID)
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
			return runMutation(func(db *sql.DB) (string, error) {
				return fmt.Sprintf("wiped feature %d", id), wipeFeature(db, id)
			})
		},
	}
}
