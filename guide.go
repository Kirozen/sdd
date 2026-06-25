package main

import (
	"database/sql"
	"fmt"

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
	rows, err := db.Query(`SELECT ord, name, id FROM feature WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return nil, err
	}
	type feat struct {
		ord  int
		name string
		pk   int64
	}
	var feats []feat
	for rows.Next() {
		var f feat
		if err := rows.Scan(&f.ord, &f.name, &f.pk); err != nil {
			rows.Close()
			return nil, err
		}
		feats = append(feats, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(feats) == 0 {
		return []string{"no feature yet → sdd-grill"}, nil
	}
	var out []string
	for _, f := range feats {
		stage, err := featureStage(db, f.pk)
		if err != nil {
			return nil, err
		}
		next := skillForStage(stage)
		// A specced feature's next move depends on its review verdict (V47): no
		// gate → review then build; go → build; no-go → re-spec then re-review.
		if stage == "specced" {
			verdict, has, err := featureGate(db, f.pk)
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
		out = append(out, fmt.Sprintf("F%d %s  [%s] → %s", f.ord, f.name, stage, next))
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
