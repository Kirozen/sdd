package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// addTest records that a named test proves an invariant (V42). The invariant is
// resolved by its per-project ordinal V<n>; an ordinal absent from the project
// is rejected (the declared-link analogue of the cite FK, V5). Re-adding the
// same (invariant, name) is a no-op, not an error — UNIQUE(invariant_id,name)
// plus ON CONFLICT DO NOTHING (V42).
func addTest(db dbq.DBTX, projectID, invOrd int64, name string) error {
	q := dbq.New(db)
	ctx := context.Background()
	invPK, err := q.InvariantIDByOrd(ctx, dbq.InvariantIDByOrdParams{ProjectID: projectID, Ord: invOrd})
	if err != nil {
		return fmt.Errorf("no invariant V%d in this project", invOrd)
	}
	return q.InsertTest(ctx, dbq.InsertTestParams{InvariantID: invPK, Name: name})
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
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				return fmt.Sprintf("V%d ← %s", ord, args[1]), addTest(db, pid, ord, args[1])
			})
		},
	}
}
