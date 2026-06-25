package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// addTest records that a named test proves an invariant (V42). The invariant is
// resolved by its per-project ordinal V<n>; an ordinal absent from the project
// is rejected (the declared-link analogue of the cite FK, V5). Re-adding the
// same (invariant, name) is a no-op, not an error — UNIQUE(invariant_id,name)
// plus ON CONFLICT DO NOTHING (V42).
func addTest(db *sql.DB, projectID, invOrd int64, name string) error {
	var invPK int64
	if err := db.QueryRow(`SELECT id FROM invariant WHERE project_id=? AND ord=?`, projectID, invOrd).Scan(&invPK); err != nil {
		return fmt.Errorf("no invariant V%d in this project", invOrd)
	}
	_, err := db.Exec(`INSERT INTO test(invariant_id, name) VALUES(?, ?) ON CONFLICT(invariant_id, name) DO NOTHING`, invPK, name)
	return err
}

// parseInvariantRef reads a V<n> reference (the V is optional) into its ordinal.
func parseInvariantRef(ref string) (int64, error) {
	raw := strings.TrimPrefix(strings.TrimPrefix(ref, "V"), "v")
	ord, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad invariant ref %q (want V<n>)", ref)
	}
	return ord, nil
}

func newAddTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-test <V-ref> <test-name>",
		Short: "link a test that proves an invariant (declared, not executed)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ord, err := parseInvariantRef(args[0])
			if err != nil {
				return err
			}
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				return fmt.Sprintf("V%d ← %s", ord, args[1]), addTest(db, pid, ord, args[1])
			})
		},
	}
}
