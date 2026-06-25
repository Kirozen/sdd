package sdd

import (
	"context"
	"fmt"
	"os"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// projectLines lists every project in the global store with its counts — the one
// read that intentionally does NOT filter by project_id (V92). It opens the global
// db directly (NOT openProjectContext, which aborts outside a registered repo,
// resolve.go), so it works from anywhere. The current project, if resolvable, is
// marked with a leading '*'; resolution is best-effort — a failure just means no
// marker, never an abort. Read-pure: no mutation, no re-export (V16).
func projectLines(db dbq.DBTX, currentPID int64) ([]string, error) {
	rows, err := dbq.New(db).ProjectsWithCounts(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		identity := r.Path
		if r.Url.Valid && r.Url.String != "" {
			identity = r.Url.String
		}
		mark := "  "
		if r.ID == currentPID {
			mark = "* "
		}
		out = append(out, fmt.Sprintf("%s%s\tfeatures:%d invariants:%d open-tasks:%d",
			mark, identity, r.Features, r.Invariants, r.OpenTasks))
	}
	return out, nil
}

func newProjectsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "projects",
		Short: "list every project in the global store with counts; read-only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openGlobalDB()
			if err != nil {
				return err
			}
			defer db.Close()
			// best-effort current-project resolution for the '*' marker (V92):
			// outside a registered repo this fails and we simply mark nothing.
			var currentPID int64
			if cwd, err := os.Getwd(); err == nil {
				if pid, err := resolveProject(db, cwd); err == nil {
					currentPID = pid
				}
			}
			lines, err := projectLines(db, currentPID)
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
