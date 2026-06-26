package sdd

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// humanBytes renders a file size in binary units (1024-based), deterministically:
// raw bytes below 1 KiB, else one decimal with the largest fitting unit.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// statsParams fans one project id into the eleven positional params ProjectStats
// expects, all the same pid — every subquery stays scoped to this project (V104).
func statsParams(pid int64) dbq.ProjectStatsParams {
	return dbq.ProjectStatsParams{
		ProjectID: pid, ProjectID_2: pid, ProjectID_3: pid, ProjectID_4: pid,
		ProjectID_5: pid, ProjectID_6: pid, ProjectID_7: pid, ProjectID_8: pid,
		ProjectID_9: pid, ProjectID_10: pid, ProjectID_11: pid,
	}
}

// projID renders a project's display identity: canonical url when present, else
// the worktree path (same rule as projects.go, V92).
func projID(url sql.NullString, path string) string {
	if url.Valid && url.String != "" {
		return url.String
	}
	return path
}

// statTypeLines renders the eight per-type volume lines from a stats row, label
// left-padded and counts right-aligned so columns line up deterministically. The
// tasks line carries its status breakdown (. ~ x). Shared by the single-project
// and --all views so both render byte-identically.
func statTypeLines(s dbq.ProjectStatsRow) []string {
	type row struct {
		label string
		n     int64
	}
	rows := []row{
		{"invariants", s.Invariants},
		{"interfaces", s.Interfaces},
		{"bugs", s.Bugs},
		{"research", s.Research},
		{"tests", s.Tests},
		{"unknowns", s.Unknowns},
		{"features", s.Features},
		{"tasks", s.TasksTotal},
	}
	labelW, countW := 0, 0
	for _, r := range rows {
		if len(r.label) > labelW {
			labelW = len(r.label)
		}
		if w := len(fmt.Sprintf("%d", r.n)); w > countW {
			countW = w
		}
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		line := fmt.Sprintf("  %-*s  %*d", labelW, r.label, countW, r.n)
		if r.label == "tasks" {
			line += fmt.Sprintf("  (. %d  ~ %d  x %d)", s.TasksTodo, s.TasksDoing, s.TasksDone)
		}
		out = append(out, line)
	}
	return out
}

// statsReport is the single-project view (V104): a 'PROJECT <identity>' header
// then the per-type volume lines. Read-pure (V16), scoped (V20).
func statsReport(db dbq.DBTX, pid int64) ([]string, error) {
	ctx := context.Background()
	q := dbq.New(db)
	p, err := q.ProjectByID(ctx, pid)
	if err != nil {
		return nil, err
	}
	s, err := q.ProjectStats(ctx, statsParams(pid))
	if err != nil {
		return nil, err
	}
	out := []string{"PROJECT " + projID(p.Url, p.Path)}
	return append(out, statTypeLines(s)...), nil
}

// allStatsReport is the store-wide view (V105): it enumerates the project
// registry (ProjectsWithCounts, the one sanctioned non-scoped read, V92) and
// SUMS each project's scoped ProjectStats — no content table is ever counted
// without a project_id filter. Header 'STORE <db-path>' carries the project
// count and the spec.db file size (os.Stat, WAL sidecar excluded). Read-pure.
func allStatsReport(db dbq.DBTX) ([]string, error) {
	ctx := context.Background()
	q := dbq.New(db)
	projects, err := q.ProjectsWithCounts(ctx)
	if err != nil {
		return nil, err
	}
	var total dbq.ProjectStatsRow
	for _, p := range projects {
		s, err := q.ProjectStats(ctx, statsParams(p.ID))
		if err != nil {
			return nil, err
		}
		total.Invariants += s.Invariants
		total.Interfaces += s.Interfaces
		total.Bugs += s.Bugs
		total.Research += s.Research
		total.Tests += s.Tests
		total.Unknowns += s.Unknowns
		total.Features += s.Features
		total.TasksTotal += s.TasksTotal
		total.TasksTodo += s.TasksTodo
		total.TasksDoing += s.TasksDoing
		total.TasksDone += s.TasksDone
	}
	var size int64
	if fi, err := os.Stat(globalDBPath()); err == nil {
		size = fi.Size()
	}
	out := []string{
		"STORE " + globalDBPath(),
		fmt.Sprintf("  projects  %d", len(projects)),
		fmt.Sprintf("  db-size   %s", humanBytes(size)),
	}
	return append(out, statTypeLines(total)...), nil
}

func newStatsCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "per-type volume counts for the current project; --all for the whole store; read-only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var lines []string
			// V106: --all opens the global db DIRECTLY (usable outside any repo,
			// like projects); the default view goes through openProjectContext
			// (aborts outside a git project, like status).
			if all {
				db, err := openGlobalDB()
				if err != nil {
					return err
				}
				defer db.Close()
				if lines, err = allStatsReport(db); err != nil {
					return err
				}
			} else {
				db, pid, _, err := openProjectContext()
				if err != nil {
					return err
				}
				defer db.Close()
				if lines, err = statsReport(db, pid); err != nil {
					return err
				}
			}
			for _, l := range lines {
				fmt.Println(l)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "aggregate across every project in the store")
	return cmd
}
