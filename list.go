package main

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

// listKind returns every row of a kind in the current project as caveman lines,
// ordered by the per-project ordinal, formatted through the same fmt*Line
// helpers as renderSpec (V18). Read-pure (V16); scoped by project (V20). An
// unknown kind errors (V17); a valid-but-empty kind returns no lines.
func listKind(db *sql.DB, projectID int64, kind string) ([]string, error) {
	switch kind {
	case "invariant":
		return listRows(db, `SELECT ord, text FROM invariant WHERE project_id=? ORDER BY ord`, projectID, func(rows *sql.Rows) (string, error) {
			var ord int
			var text string
			if err := rows.Scan(&ord, &text); err != nil {
				return "", err
			}
			return fmtInvariantLine(ord, text), nil
		})
	case "interface":
		return listRows(db, `SELECT kind, name, sig, status FROM interface WHERE project_id=? ORDER BY id`, projectID, func(rows *sql.Rows) (string, error) {
			var k, name, sig, status string
			if err := rows.Scan(&k, &name, &sig, &status); err != nil {
				return "", err
			}
			return fmtInterfaceLine(k, name, sig, status), nil
		})
	case "research":
		return listRows(db, `SELECT ord, topic, finding, src FROM research WHERE project_id=? ORDER BY ord`, projectID, func(rows *sql.Rows) (string, error) {
			var ord int
			var topic, finding, src string
			if err := rows.Scan(&ord, &topic, &finding, &src); err != nil {
				return "", err
			}
			return fmtResearchLine(ord, topic, finding, src), nil
		})
	case "feature":
		return listRows(db, `SELECT ord, name FROM feature WHERE project_id=? ORDER BY ord`, projectID, func(rows *sql.Rows) (string, error) {
			var ord int
			var name string
			if err := rows.Scan(&ord, &name); err != nil {
				return "", err
			}
			return fmt.Sprintf("FEATURE %d: %s", ord, name), nil
		})
	case "task":
		return listTasks(db, projectID)
	case "bug":
		return listBugs(db, projectID)
	default:
		return nil, fmt.Errorf("unknown kind %q (want invariant|interface|task|bug|research|feature)", kind)
	}
}

// listRows runs query (scoped to projectID) and maps each row to a line via fn.
func listRows(db *sql.DB, query string, projectID int64, fn func(*sql.Rows) (string, error)) ([]string, error) {
	rows, err := db.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		line, err := fn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, rows.Err()
}

// listTasks and listBugs need a second query per row (cites / fix), so they
// drain the cursor before re-joining.
func listTasks(db *sql.DB, projectID int64) ([]string, error) {
	type tk struct {
		pk           int64
		ord          int
		status, text string
	}
	var tasks []tk
	rows, err := db.Query(`SELECT t.id, t.ord, t.status, t.text FROM task t JOIN feature f ON f.id=t.feature_id WHERE f.project_id=? ORDER BY t.ord`, projectID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t tk
		if err := rows.Scan(&t.pk, &t.ord, &t.status, &t.text); err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []string
	for _, t := range tasks {
		cites, err := taskCites(db, t.pk)
		if err != nil {
			return nil, err
		}
		out = append(out, fmtTaskLine(t.ord, t.status, t.text, cites))
	}
	return out, nil
}

func listBugs(db *sql.DB, projectID int64) ([]string, error) {
	type bg struct {
		pk          int64
		ord         int
		date, cause string
	}
	var bugs []bg
	rows, err := db.Query(`SELECT id, ord, date, cause FROM bug WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var b bg
		if err := rows.Scan(&b.pk, &b.ord, &b.date, &b.cause); err != nil {
			rows.Close()
			return nil, err
		}
		bugs = append(bugs, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []string
	for _, b := range bugs {
		fix, err := bugFix(db, b.pk)
		if err != nil {
			return nil, err
		}
		out = append(out, fmtBugLine(b.ord, b.date, b.cause, fix))
	}
	return out, nil
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <kind>",
		Short: "print all rows of a kind (invariant|interface|task|bug|research|feature), read-only",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := listKind(db, pid, args[0])
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
