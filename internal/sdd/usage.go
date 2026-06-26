package sdd

import (
	"context"
	"fmt"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// usageRow is the command-agnostic shape both the per-project and store-wide
// queries fold into, so a single renderer serves both views (V114).
type usageRow struct {
	command  string
	ok, fail int64
	lastSeen string
}

// usageLines renders the per-command counter table: command left-aligned, ok and
// err counts right-aligned, last-seen trailing. Column widths are derived so the
// output is deterministic; rows arrive already sorted busiest-first with command
// breaking ties (V114). Shared by the single-project and --all views so both
// render byte-identically.
func usageLines(rows []usageRow) []string {
	cmdW, okW, errW := 0, 1, 1
	for _, r := range rows {
		if len(r.command) > cmdW {
			cmdW = len(r.command)
		}
		if w := len(fmt.Sprintf("%d", r.ok)); w > okW {
			okW = w
		}
		if w := len(fmt.Sprintf("%d", r.fail)); w > errW {
			errW = w
		}
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprintf("  %-*s  %*d ok  %*d err  %s",
			cmdW, r.command, okW, r.ok, errW, r.fail, r.lastSeen))
	}
	return out
}

// usageReport is the single-project view (V114): a 'PROJECT <identity>' header
// then the per-command counter lines for that project. Read-pure (V16), scoped
// (V20).
func usageReport(db dbq.DBTX, pid int64) ([]string, error) {
	ctx := context.Background()
	q := dbq.New(db)
	p, err := q.ProjectByID(ctx, pid)
	if err != nil {
		return nil, err
	}
	rs, err := q.CommandUsageByProject(ctx, pid)
	if err != nil {
		return nil, err
	}
	rows := make([]usageRow, len(rs))
	for i, r := range rs {
		rows[i] = usageRow{r.Command, r.OkCount, r.FailCount, r.LastSeen}
	}
	out := []string{"PROJECT " + projID(p.Url, p.Path)}
	return append(out, usageLines(rows)...), nil
}

// allUsageReport is the store-wide view (V114): a 'STORE <db-path>' header then
// per-command totals summed across every project (bucket 0 included). Read-pure.
func allUsageReport(db dbq.DBTX) ([]string, error) {
	ctx := context.Background()
	rs, err := dbq.New(db).CommandUsageAcrossStore(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]usageRow, len(rs))
	for i, r := range rs {
		rows[i] = usageRow{r.Command, r.OkCount, r.FailCount, r.LastSeen}
	}
	out := []string{"STORE " + globalDBPath()}
	return append(out, usageLines(rows)...), nil
}

func newUsageCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "per-command invocation counts for the current project; --all for the whole store; read-only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var lines []string
			// V114: --all opens the global db DIRECTLY (usable outside any repo,
			// like stats --all / projects); the default view goes through
			// openProjectContext (aborts outside a git project, like status).
			if all {
				db, err := openGlobalDB()
				if err != nil {
					return err
				}
				defer db.Close()
				if lines, err = allUsageReport(db); err != nil {
					return err
				}
			} else {
				db, pid, _, err := openProjectContext()
				if err != nil {
					return err
				}
				defer db.Close()
				if lines, err = usageReport(db, pid); err != nil {
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
