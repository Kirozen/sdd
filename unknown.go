package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// nextUnknownOrd is the next per-project display ordinal U<n> for unknowns,
// whose project is reached through their feature (V26, like nextTaskOrd).
func nextUnknownOrd(db *sql.DB, projectID int64) (int, error) {
	n, err := dbq.New(db).NextUnknownOrd(context.Background(), nz(projectID))
	return int(n), err
}

// addUnknown records a parked question on a feature as an open unknown (V35),
// returning its per-project ordinal U<n>.
func addUnknown(db *sql.DB, projectID, featurePK int64, text string) (int, error) {
	ord, err := nextUnknownOrd(db, projectID)
	if err != nil {
		return 0, err
	}
	if err := dbq.New(db).InsertUnknown(context.Background(), dbq.InsertUnknownParams{
		FeatureID: featurePK, Ord: nz(int64(ord)), Text: text,
	}); err != nil {
		return 0, err
	}
	return ord, nil
}

// resolveUnknown marks an unknown resolved by its per-project ordinal, scoped to
// the project (V20, V26); never hard-deletes (V35). Unknown ordinal → error.
func resolveUnknown(db *sql.DB, projectID, ord int64) error {
	n, err := dbq.New(db).ResolveUnknown(context.Background(), dbq.ResolveUnknownParams{
		Ord: nz(ord), ProjectID: nz(projectID),
	})
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
