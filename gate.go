package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// setGate records (or replaces) the review verdict for a feature (V46). One row
// per feature: feature_id UNIQUE turns the INSERT into an UPSERT, so the latest
// verdict wins and history is not kept. recorded_at stamps when the verdict was
// taken — the honest limit is that it does not auto-invalidate on later spec
// edits (V48). Scoped to the project via featurePK (V20).
func setGate(db *sql.DB, projectID, featOrd int64, verdict, note string) error {
	pk, err := featurePK(db, projectID, featOrd)
	if err != nil {
		return err
	}
	return dbq.New(db).UpsertGate(context.Background(), dbq.UpsertGateParams{
		FeatureID: pk, Verdict: verdict,
		Note:       sql.NullString{String: note, Valid: true},
		RecordedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// featureGate returns a feature's recorded verdict, has=false when none exists.
func featureGate(db *sql.DB, featurePK int64) (verdict string, has bool, err error) {
	switch v, e := dbq.New(db).GateVerdict(context.Background(), featurePK); e {
	case nil:
		return v, true, nil
	case sql.ErrNoRows:
		return "", false, nil
	default:
		return "", false, e
	}
}

func newGateCmd() *cobra.Command {
	var isGo, isNoGo bool
	var note string
	c := &cobra.Command{
		Use:   "gate <F-ord>",
		Short: "record a durable review verdict (--go|--no-go) for a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ord, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("bad feature ordinal %q", args[0])
			}
			if isGo == isNoGo { // both unset or both set
				return fmt.Errorf("exactly one of --go / --no-go is required")
			}
			verdict := "go"
			if isNoGo {
				verdict = "no-go"
			}
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				return fmt.Sprintf("F%d gate → %s", ord, verdict), setGate(db, pid, ord, verdict, note)
			})
		},
	}
	c.Flags().BoolVar(&isGo, "go", false, "verdict: go")
	c.Flags().BoolVar(&isNoGo, "no-go", false, "verdict: no-go")
	c.Flags().StringVar(&note, "note", "", "optional note")
	return c
}
