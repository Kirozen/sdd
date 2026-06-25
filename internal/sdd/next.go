package sdd

import (
	"context"
	"database/sql"
	"fmt"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// newNextCmd wires `sdd next` (I.next): print the next actionable task with its
// context, or the empty-case hint. Read-only (V16), project-scoped (V20).
func newNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "next actionable task + its goal and resolved cites, read-only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := nextOutput(db, pid)
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

// nextResult is the chosen actionable task plus the context that explains it:
// the owning feature, all its goals (V33), the task's caveman line, and each
// cite resolved to its full V/I line. Nil when the project has no actionable
// (non-`x`) task — the caller decides the empty-case hint (V31).
type nextResult struct {
	featName  string
	taskLine  string
	goals     []string
	citeLines []string
	featOrd   int
}

// nextActionable selects the next task to work on (V30): the first feature by
// ord that owns a non-`x` task, and within it `~` (wip) before the smallest-ord
// `.` (todo). Read-pure (V16): SELECT only, no write, no re-export. Scoped to
// the project (V20): every query filters by project_id. Returns nil when no
// actionable task exists.
func nextActionable(db *sql.DB, projectID int64) (*nextResult, error) {
	// Lowest-ord feature with a non-`x` task; within it `~` before `.`, then ord.
	r, err := dbq.New(db).NextActionableTask(context.Background(), projectID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	goals, err := featureGoals(db, r.FeatID)
	if err != nil {
		return nil, err
	}

	cites, err := taskCites(db, r.TaskID)
	if err != nil {
		return nil, err
	}
	citeLines, err := resolveCites(db, projectID, cites)
	if err != nil {
		return nil, err
	}

	return &nextResult{
		featOrd:   int(r.FeatOrd),
		featName:  r.FeatName,
		goals:     goals,
		taskLine:  fmtTaskLine(int(r.TaskOrd), r.Status, r.Text, cites),
		citeLines: citeLines,
	}, nil
}

// nextOutput is the full line set `sdd next` prints: the actionable task block
// when one exists, else the empty-case hint (V31). Read-pure throughout.
func nextOutput(db *sql.DB, projectID int64) ([]string, error) {
	r, err := nextActionable(db, projectID)
	if err != nil {
		return nil, err
	}
	if r != nil {
		return renderNext(r), nil
	}
	return emptyHint(db, projectID)
}

// renderNext frames the chosen task with its context: the feature, every goal
// (V33), the task's caveman line and each resolved cite — task/cite lines are
// byte-identical to their spec rows (V18); the rest is framing.
func renderNext(r *nextResult) []string {
	out := []string{fmt.Sprintf("F%d %s", r.featOrd, r.featName)}
	for _, g := range r.goals {
		out = append(out, "§G "+g)
	}
	out = append(out, r.taskLine)
	for _, c := range r.citeLines {
		out = append(out, "  "+c)
	}
	return out
}

// emptyHint covers V31's no-actionable-task cases: a feature still awaiting
// spec (seeded/grilled) is pointed at; otherwise, with features present and all
// built, the project is up to date; with no feature at all, point at the grill.
func emptyHint(db *sql.DB, projectID int64) ([]string, error) {
	feats, err := dbq.New(db).FeaturesByProject(context.Background(), projectID)
	if err != nil {
		return nil, err
	}

	if len(feats) == 0 {
		return []string{"no feature yet — start with sdd-grill, then sdd new-feature"}, nil
	}
	var awaiting []string
	for _, f := range feats {
		stage, err := featureStage(db, f.ID)
		if err != nil {
			return nil, err
		}
		if stageBeforeSpec(stage) {
			awaiting = append(awaiting, fmt.Sprintf("F%d %s awaits spec (%s) — run sdd-spec", int(f.Ord), f.Name, stage))
		}
	}
	if len(awaiting) > 0 {
		return awaiting, nil
	}
	return []string{"project up to date — every task done"}, nil
}

// stageBeforeSpec reports whether a stage precedes "specced": a feature with no
// tasks yet still awaits sdd-spec, so next must not claim the project is done.
func stageBeforeSpec(stage string) bool {
	return stage == "seeded" || stage == "grilled"
}

// featureGoals returns every goal of a feature in display order (V33: 0..N,
// ORDER BY id like renderSpec). A feature with no goals yields an empty slice,
// never an error.
func featureGoals(db *sql.DB, featurePK int64) ([]string, error) {
	return dbq.New(db).GoalsByFeature(context.Background(), featurePK)
}

// resolveCites maps a task's raw cite string (e.g. "V30,I.next" or the "-"
// sentinel for none) to each cite's full caveman line via showRef, so next
// renders through the same single source as the spec (V18).
func resolveCites(db *sql.DB, projectID int64, cites string) ([]string, error) {
	refs := splitRefs(cites)
	if len(refs) == 0 {
		return nil, nil
	}
	lines := make([]string, 0, len(refs))
	for _, r := range refs {
		line, err := showRef(db, projectID, r)
		if err != nil {
			return nil, fmt.Errorf("resolving cite %q: %w", r, err)
		}
		lines = append(lines, line)
	}
	return lines, nil
}
