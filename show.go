package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

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
	switch {
	case strings.HasPrefix(ref, "I."):
		name := ref[2:]
		var kind, sig, status string
		if err := db.QueryRow(`SELECT kind, sig, status FROM interface WHERE project_id=? AND name=?`, projectID, name).Scan(&kind, &sig, &status); err != nil {
			return "", fmt.Errorf("no interface %q in this project", ref)
		}
		return fmtInterfaceLine(kind, name, sig, status), nil

	case strings.HasPrefix(ref, "V"):
		ord, err := refID(ref)
		if err != nil {
			return "", err
		}
		var text string
		if err := db.QueryRow(`SELECT text FROM invariant WHERE project_id=? AND ord=?`, projectID, ord).Scan(&text); err != nil {
			return "", fmt.Errorf("no invariant %q in this project", ref)
		}
		return fmtInvariantLine(ord, text), nil

	case strings.HasPrefix(ref, "T"):
		ord, err := refID(ref)
		if err != nil {
			return "", err
		}
		var pk int64
		var status, text string
		if err := db.QueryRow(`SELECT t.id, t.status, t.text FROM task t JOIN feature f ON f.id=t.feature_id WHERE f.project_id=? AND t.ord=?`, projectID, ord).Scan(&pk, &status, &text); err != nil {
			return "", fmt.Errorf("no task %q in this project", ref)
		}
		cites, err := taskCites(db, pk)
		if err != nil {
			return "", err
		}
		return fmtTaskLine(ord, status, text, cites), nil

	case strings.HasPrefix(ref, "B"):
		ord, err := refID(ref)
		if err != nil {
			return "", err
		}
		var pk int64
		var date, cause string
		if err := db.QueryRow(`SELECT id, date, cause FROM bug WHERE project_id=? AND ord=?`, projectID, ord).Scan(&pk, &date, &cause); err != nil {
			return "", fmt.Errorf("no bug %q in this project", ref)
		}
		fix, err := bugFix(db, pk)
		if err != nil {
			return "", err
		}
		return fmtBugLine(ord, date, cause, fix), nil

	case strings.HasPrefix(ref, "R"):
		ord, err := refID(ref)
		if err != nil {
			return "", err
		}
		var topic, finding, src string
		if err := db.QueryRow(`SELECT topic, finding, src FROM research WHERE project_id=? AND ord=?`, projectID, ord).Scan(&topic, &finding, &src); err != nil {
			return "", fmt.Errorf("no research %q in this project", ref)
		}
		return fmtResearchLine(ord, topic, finding, src), nil

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
