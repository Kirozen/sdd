package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// checkSpec re-renders the db and compares it byte-for-byte to the on-disk
// SPEC.md. Any difference (hand-edit, stale cache) is a drift error (V6).
func checkSpec(db *sql.DB, projectID int64, path string) error {
	want, err := renderSpec(db, projectID)
	if err != nil {
		return err
	}
	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if string(got) != want {
		return fmt.Errorf("%s drifted from spec.db (hand-edited or stale); run `sdd export`", path)
	}
	return nil
}

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "fail if SPEC.md drifted from spec.db",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, specFile, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			if err := checkSpec(db, pid, specFile); err != nil {
				return err
			}
			cmd.Println("SPEC.md matches spec.db")
			return nil
		},
	}
}
