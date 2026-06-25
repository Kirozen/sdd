package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// refID parses the numeric tail of a ref like V12/T3/B1/R7 — the per-project
// ordinal, not a global id.
func refID(ref string) (int, error) {
	n, err := strconv.Atoi(ref[1:])
	if err != nil {
		return 0, fmt.Errorf("bad ref %q", ref)
	}
	return n, nil
}

// showRef returns the single caveman line for a ref within the current project
// (V<ord>/I.<name>/T<ord>/B<ord>/R<ord>), formatted through the same fmt*Line
// helpers as renderSpec (V18). Read-pure (V16); scoped by project (V20).
// Unknown ref/kind → error (V17).
func showRef(db *sql.DB, projectID int64, ref string) (string, error) {
	ctx := context.Background()
	q := dbq.New(db)
	switch {
	case strings.HasPrefix(ref, "I."):
		name := ref[2:]
		r, err := q.ShowInterface(ctx, dbq.ShowInterfaceParams{ProjectID: projectID, Name: name})
		if err != nil {
			return "", fmt.Errorf("no interface %q in this project", ref)
		}
		return fmtInterfaceLine(r.Kind, name, r.Sig, r.Status), nil

	case strings.HasPrefix(ref, "V"):
		ord, err := refID(ref)
		if err != nil {
			return "", err
		}
		text, err := q.ShowInvariant(ctx, dbq.ShowInvariantParams{ProjectID: projectID, Ord: int64(ord)})
		if err != nil {
			return "", fmt.Errorf("no invariant %q in this project", ref)
		}
		return fmtInvariantLine(ord, text), nil

	case strings.HasPrefix(ref, "T"):
		ord, err := refID(ref)
		if err != nil {
			return "", err
		}
		r, err := q.ShowTask(ctx, dbq.ShowTaskParams{ProjectID: projectID, Ord: int64(ord)})
		if err != nil {
			return "", fmt.Errorf("no task %q in this project", ref)
		}
		cites, err := taskCites(db, r.ID)
		if err != nil {
			return "", err
		}
		return fmtTaskLine(ord, r.Status, r.Text, cites), nil

	case strings.HasPrefix(ref, "B"):
		ord, err := refID(ref)
		if err != nil {
			return "", err
		}
		r, err := q.ShowBug(ctx, dbq.ShowBugParams{ProjectID: projectID, Ord: int64(ord)})
		if err != nil {
			return "", fmt.Errorf("no bug %q in this project", ref)
		}
		fix, err := bugFix(db, r.ID)
		if err != nil {
			return "", err
		}
		return fmtBugLine(ord, r.Date, r.Cause, fix), nil

	case strings.HasPrefix(ref, "R"):
		ord, err := refID(ref)
		if err != nil {
			return "", err
		}
		r, err := q.ShowResearch(ctx, dbq.ShowResearchParams{ProjectID: projectID, Ord: int64(ord)})
		if err != nil {
			return "", fmt.Errorf("no research %q in this project", ref)
		}
		return fmtResearchLine(ord, r.Topic, r.Finding, r.Src), nil

	default:
		return "", fmt.Errorf("unrecognized ref %q (want V<n>/I.<name>/T<n>/B<n>/R<n>)", ref)
	}
}

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <ref>",
		Short: "print the caveman line for one ref (V/I/T/B/R), read-only",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			line, err := showRef(db, pid, args[0])
			if err != nil {
				return err
			}
			fmt.Println(line)
			return nil
		},
	}
}
