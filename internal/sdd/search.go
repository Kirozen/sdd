package sdd

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// contains is a case-insensitive substring test (the search match, V93). ASCII
// folding via ToLower is enough for the spec's content.
func contains(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// searchHits returns every row of the current project whose HUMAN content (not
// its cites/ordinals, V93) contains term, rendered through the same fmt*Line
// helpers as renderSpec (V18) and prefixed by its kind. Kinds are walked in the
// canonical order (V28) with unknown appended after task. It reuses the existing
// *ByProject queries — no bespoke LIKE query — so a hit line is byte-identical to
// its SPEC.md line. Read-pure (V16); scoped to the project (V20).
func searchHits(db *sql.DB, projectID int64, term string) ([]string, error) {
	ctx := context.Background()
	q := dbq.New(db)
	var out []string

	ifaces, err := q.InterfacesByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, r := range ifaces {
		if contains(r.Sig, term) {
			out = append(out, "interface "+fmtInterfaceLine(r.Kind, r.Name, r.Sig, r.Status))
		}
	}

	research, err := q.ResearchByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, r := range research {
		if contains(r.Topic, term) || contains(r.Finding, term) {
			out = append(out, "research "+fmtResearchLine(int(r.Ord), r.Topic, r.Finding, r.Src))
		}
	}

	invs, err := q.InvariantsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, r := range invs {
		if contains(r.Text, term) {
			out = append(out, "invariant "+fmtInvariantLine(int(r.Ord), r.Text))
		}
	}

	bugs, err := q.BugsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, b := range bugs {
		if contains(b.Cause, term) {
			fix, err := bugFix(db, b.ID)
			if err != nil {
				return nil, err
			}
			out = append(out, "bug "+fmtBugLine(int(b.Ord), b.Date, b.Cause, fix))
		}
	}

	tasks, err := q.TasksInProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if contains(t.Text, term) {
			cites, err := taskCites(db, t.ID)
			if err != nil {
				return nil, err
			}
			out = append(out, "task "+fmtTaskLine(int(t.Ord), t.Status, t.Text, cites))
		}
	}

	unknowns, err := q.UnknownsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, u := range unknowns {
		if contains(u.Text, term) {
			out = append(out, "unknown "+fmtUnknownLine(int(u.Ord), u.Status, u.Text))
		}
	}

	return out, nil
}

func newSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <term>",
		Short: "substring search over V/I/T/B/R/U of the current project; read-only",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := searchHits(db, pid, args[0])
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
