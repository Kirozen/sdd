package main

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

// listKind returns every row of a kind as caveman lines, ORDER BY id, formatted
// through the same fmt*Line helpers as renderSpec (V18). Read-pure (V16). An
// unknown kind errors (V17); a valid-but-empty kind returns no lines, no error.
func listKind(db *sql.DB, kind string) ([]string, error) {
	switch kind {
	case "invariant":
		return listRows(db, `SELECT id, text FROM invariant ORDER BY id`, func(rows *sql.Rows) (string, error) {
			var id int
			var text string
			if err := rows.Scan(&id, &text); err != nil {
				return "", err
			}
			return fmtInvariantLine(id, text), nil
		})
	case "interface":
		return listRows(db, `SELECT id, kind, name, sig, status FROM interface ORDER BY id`, func(rows *sql.Rows) (string, error) {
			var id int
			var k, name, sig, status string
			if err := rows.Scan(&id, &k, &name, &sig, &status); err != nil {
				return "", err
			}
			return fmtInterfaceLine(k, name, sig, status), nil
		})
	case "research":
		return listRows(db, `SELECT id, topic, finding, src FROM research ORDER BY id`, func(rows *sql.Rows) (string, error) {
			var id int
			var topic, finding, src string
			if err := rows.Scan(&id, &topic, &finding, &src); err != nil {
				return "", err
			}
			return fmtResearchLine(id, topic, finding, src), nil
		})
	case "feature":
		return listRows(db, `SELECT id, name FROM feature ORDER BY id`, func(rows *sql.Rows) (string, error) {
			var id int
			var name string
			if err := rows.Scan(&id, &name); err != nil {
				return "", err
			}
			return fmt.Sprintf("FEATURE %d: %s", id, name), nil
		})
	case "task":
		return listTasks(db)
	case "bug":
		return listBugs(db)
	default:
		return nil, fmt.Errorf("unknown kind %q (want invariant|interface|task|bug|research|feature)", kind)
	}
}

// listRows runs query and maps each row to a line via fn.
func listRows(db *sql.DB, query string, fn func(*sql.Rows) (string, error)) ([]string, error) {
	rows, err := db.Query(query)
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
// drain the cursor before re-joining (a cursor can't be open during a query).
func listTasks(db *sql.DB) ([]string, error) {
	type tk struct {
		id           int
		status, text string
	}
	var tasks []tk
	rows, err := db.Query(`SELECT id, status, text FROM task ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t tk
		if err := rows.Scan(&t.id, &t.status, &t.text); err != nil {
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
		cites, err := taskCites(db, t.id)
		if err != nil {
			return nil, err
		}
		out = append(out, fmtTaskLine(t.id, t.status, t.text, cites))
	}
	return out, nil
}

func listBugs(db *sql.DB) ([]string, error) {
	type bg struct {
		id          int
		date, cause string
	}
	var bugs []bg
	rows, err := db.Query(`SELECT id, date, cause FROM bug ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var b bg
		if err := rows.Scan(&b.id, &b.date, &b.cause); err != nil {
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
		fix, err := bugFix(db, b.id)
		if err != nil {
			return nil, err
		}
		out = append(out, fmtBugLine(b.id, b.date, b.cause, fix))
	}
	return out, nil
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <kind>",
		Short: "print all rows of a kind (invariant|interface|task|bug|research|feature), read-only",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openProjectDB()
			if err != nil {
				return err
			}
			defer db.Close()
			lines, err := listKind(db, args[0])
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
