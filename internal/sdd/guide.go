package sdd

import (
	"context"
	"database/sql"
	"fmt"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// skillForStage maps an inferred pipeline stage (V32) to the skill that moves
// the feature forward (V39). The mapping is the spec workflow read backwards:
// grill → spec → review|build → build → deepen.
func skillForStage(stage string) string {
	switch stage {
	case "seeded":
		return "sdd-grill (define the goal)"
	case "grilled":
		return "sdd-spec (write invariants + tasks)"
	case "specced":
		return "sdd-review then sdd-build"
	case "building":
		return "sdd-build (next task)"
	case "built":
		return "done — sdd-deepen if spare budget"
	default:
		return "?"
	}
}

// guideReport is a per-feature {stage → recommended skill} map (V39): one line
// per feature pointing at the next move. An empty project points at the grill.
// Read-pure (V16); scoped to the project (V20).
func guideReport(db *sql.DB, projectID int64) ([]string, error) {
	feats, err := dbq.New(db).FeaturesByProject(context.Background(), projectID)
	if err != nil {
		return nil, err
	}

	if len(feats) == 0 {
		return []string{"no feature yet → sdd-grill"}, nil
	}
	var out []string
	for _, f := range feats {
		stage, err := featureStage(db, f.ID)
		if err != nil {
			return nil, err
		}
		next := skillForStage(stage)
		// A specced feature's next move depends on its review verdict (V47): no
		// gate → review then build; go → build; no-go → re-spec then re-review.
		if stage == "specced" {
			verdict, has, err := featureGate(db, f.ID)
			if err != nil {
				return nil, err
			}
			switch {
			case !has:
				next = "sdd-review then sdd-build"
			case verdict == "go":
				next = "sdd-build (reviewed:go)"
			default:
				next = "blocked: re-spec then re-review (no-go)"
			}
		}
		out = append(out, fmt.Sprintf("F%d %s  [%s] → %s", int(f.Ord), f.Name, stage, next))
	}
	return out, nil
}

func newGuideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guide",
		Short: "per-feature stage → recommended next skill, read-only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := guideReport(db, pid)
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
