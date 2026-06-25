package main

import (
	"context"
	"database/sql"
	"fmt"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// statusReport is the current project's health view: one line per feature with
// its task counts by status, then one warning per task that cites a deprecated
// interface (V19). Read-pure (V16), scoped (V20); it does not touch check's
// drift contract (V6).
func statusReport(db *sql.DB, projectID int64) ([]string, error) {
	ctx := context.Background()
	q := dbq.New(db)
	feats, err := q.FeaturesByProject(ctx, nz(projectID))
	if err != nil {
		return nil, err
	}

	var out []string
	for _, f := range feats {
		counts, err := q.TaskStatusCounts(ctx, f.ID)
		if err != nil {
			return nil, err
		}
		c := map[string]int64{}
		for _, r := range counts {
			c[r.Status] = r.N
		}
		stage, err := featureStage(db, f.ID)
		if err != nil {
			return nil, err
		}
		// V34: stage appended AFTER the counts; counts substring + V19 lines unchanged.
		out = append(out, fmt.Sprintf("F%d %s  x:%d ~:%d .:%d [%s]", int(f.Ord.Int64), f.Name, c["x"], c["~"], c["."], stage))
	}

	// V19: flag every task in this project citing a deprecated interface.
	warnings, err := q.DeprecatedCiteWarnings(ctx, nz(projectID))
	if err != nil {
		return nil, err
	}
	for _, w := range warnings {
		out = append(out, fmt.Sprintf("! T%d cites deprecated I.%s", int(w.Ord.Int64), w.Name))
	}

	// V37: flag every feature carrying at least one open unknown (non-blocking).
	openUnknowns, err := q.OpenUnknownFeatures(ctx, nz(projectID))
	if err != nil {
		return nil, err
	}
	for _, u := range openUnknowns {
		out = append(out, fmt.Sprintf("! F%d %s: %d unknowns ouverts", int(u.Ord.Int64), u.Name, u.N))
	}
	return out, nil
}

// featureStage infers a feature's pipeline stage from its data (V32, first
// match): built(tasks ∧ all x) ▸ specced(tasks ∧ all .) ▸ building(tasks ∧
// neither) ▸ grilled(goal|constraint ∧ no task) ▸ seeded(none). "reviewed" is
// not inferable (no DB trace) → never returned (parked ?). Read-pure.
func featureStage(db *sql.DB, featurePK int64) (string, error) {
	ctx := context.Background()
	q := dbq.New(db)
	counts, err := q.FeatureStageCounts(ctx, featurePK)
	if err != nil {
		return "", err
	}
	if counts.Total > 0 {
		switch {
		case counts.Done == counts.Total:
			return "built", nil
		case counts.Todo == counts.Total:
			return "specced", nil
		default:
			return "building", nil
		}
	}
	has, err := q.FeatureGoalConstraintCount(ctx, dbq.FeatureGoalConstraintCountParams{
		FeatureID: featurePK, FeatureID_2: featurePK,
	})
	if err != nil {
		return "", err
	}
	if has > 0 {
		return "grilled", nil
	}
	return "seeded", nil
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "per-feature task counts + deprecated-cite warnings, read-only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := statusReport(db, pid)
			if err != nil {
				return err
			}
			for _, l := range lines {
				fmt.Println(l)
			}
			return nil
		},
	}
}
