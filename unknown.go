package main

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// nextUnknownOrd is the next per-project display ordinal U<n> for unknowns,
// whose project is reached through their feature (V26, like nextTaskOrd).
func nextUnknownOrd(db *sql.DB, projectID int64) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COALESCE(MAX(u.ord),0)+1 FROM unknown u JOIN feature f ON f.id=u.feature_id WHERE f.project_id=?`, projectID).Scan(&n)
	return n, err
}

// addUnknown records a parked question on a feature as an open unknown (V35),
// returning its per-project ordinal U<n>.
func addUnknown(db *sql.DB, projectID, featurePK int64, text string) (int, error) {
	ord, err := nextUnknownOrd(db, projectID)
	if err != nil {
		return 0, err
	}
	if _, err := db.Exec(`INSERT INTO unknown(feature_id, ord, text) VALUES(?, ?, ?)`, featurePK, ord, text); err != nil {
		return 0, err
	}
	return ord, nil
}

// resolveUnknown marks an unknown resolved by its per-project ordinal, scoped to
// the project (V20, V26); never hard-deletes (V35). Unknown ordinal → error.
func resolveUnknown(db *sql.DB, projectID, ord int64) error {
	res, err := db.Exec(`UPDATE unknown SET status='resolved' WHERE ord=? AND feature_id IN (SELECT id FROM feature WHERE project_id=?)`, ord, projectID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no unknown U%d in this project", ord)
	}
	return nil
}

func newAddUnknownCmd() *cobra.Command {
	var feature int64
	c := &cobra.Command{
		Use:   "add-unknown <text>",
		Short: "park an open unknown (question) on a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				pk, err := featurePK(db, pid, feature)
				if err != nil {
					return "", err
				}
				ord, err := addUnknown(db, pid, pk, args[0])
				return fmt.Sprintf("U%d", ord), err
			})
		},
	}
	c.Flags().Int64Var(&feature, "feature", 0, "feature number (required)")
	c.MarkFlagRequired("feature")
	return c
}

func newResolveUnknownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resolve-unknown <U-ord>",
		Short: "mark an unknown resolved (kept, never deleted)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ord, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("bad unknown ordinal %q", args[0])
			}
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				return fmt.Sprintf("U%d → resolved", ord), resolveUnknown(db, pid, ord)
			})
		},
	}
}
