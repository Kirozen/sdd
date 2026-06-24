package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// refID parses the numeric tail of a ref like V12/T3/B1/R7.
func refID(ref string) (int, error) {
	n, err := strconv.Atoi(ref[1:])
	if err != nil {
		return 0, fmt.Errorf("bad ref %q", ref)
	}
	return n, nil
}

// showRef returns the single caveman line for a ref (V<n>/I.<name>/T<n>/B<n>/R<n>),
// formatted through the same fmt*Line helpers as renderSpec (V18). Read-pure:
// queries only, no txn, no re-export (V16). Unknown ref/kind → error (V17).
func showRef(db *sql.DB, ref string) (string, error) {
	switch {
	case strings.HasPrefix(ref, "I."):
		name := ref[2:]
		var kind, sig, status string
		if err := db.QueryRow(`SELECT kind, sig, status FROM interface WHERE name=?`, name).Scan(&kind, &sig, &status); err != nil {
			return "", fmt.Errorf("no interface %q", ref)
		}
		return fmtInterfaceLine(kind, name, sig, status), nil

	case strings.HasPrefix(ref, "V"):
		id, err := refID(ref)
		if err != nil {
			return "", err
		}
		var text string
		if err := db.QueryRow(`SELECT text FROM invariant WHERE id=?`, id).Scan(&text); err != nil {
			return "", fmt.Errorf("no invariant %q", ref)
		}
		return fmtInvariantLine(id, text), nil

	case strings.HasPrefix(ref, "T"):
		id, err := refID(ref)
		if err != nil {
			return "", err
		}
		var status, text string
		if err := db.QueryRow(`SELECT status, text FROM task WHERE id=?`, id).Scan(&status, &text); err != nil {
			return "", fmt.Errorf("no task %q", ref)
		}
		cites, err := taskCites(db, id)
		if err != nil {
			return "", err
		}
		return fmtTaskLine(id, status, text, cites), nil

	case strings.HasPrefix(ref, "B"):
		id, err := refID(ref)
		if err != nil {
			return "", err
		}
		var date, cause string
		if err := db.QueryRow(`SELECT date, cause FROM bug WHERE id=?`, id).Scan(&date, &cause); err != nil {
			return "", fmt.Errorf("no bug %q", ref)
		}
		fix, err := bugFix(db, id)
		if err != nil {
			return "", err
		}
		return fmtBugLine(id, date, cause, fix), nil

	case strings.HasPrefix(ref, "R"):
		id, err := refID(ref)
		if err != nil {
			return "", err
		}
		var topic, finding, src string
		if err := db.QueryRow(`SELECT topic, finding, src FROM research WHERE id=?`, id).Scan(&topic, &finding, &src); err != nil {
			return "", fmt.Errorf("no research %q", ref)
		}
		return fmtResearchLine(id, topic, finding, src), nil

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
			db, err := openProjectDB()
			if err != nil {
				return err
			}
			defer db.Close()
			line, err := showRef(db, args[0])
			if err != nil {
				return err
			}
			fmt.Println(line)
			return nil
		},
	}
}
