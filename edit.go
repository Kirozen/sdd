package main

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func addResearch(db *sql.DB, projectID int64, topic, finding, src string) (int64, error) {
	ord, err := nextOrd(db, "research", projectID)
	if err != nil {
		return 0, err
	}
	if _, err := db.Exec(`INSERT INTO research(project_id, ord, topic, finding, src) VALUES(?, ?, ?, ?, ?)`, projectID, ord, topic, finding, src); err != nil {
		return 0, err
	}
	return int64(ord), nil
}

// editRow updates a row's primary text field in place, addressing it within the
// current project: by per-project ordinal (invariant/bug/research/task), by name
// (interface), or by global id (goal/constraint, which carry no display number).
// The row id is never changed, so citations stay valid (V12).
func editRow(db *sql.DB, projectID int64, kind, key, text string) error {
	var q string
	var args []any
	switch kind {
	case "invariant", "research", "bug":
		col := map[string]string{"invariant": "text", "research": "finding", "bug": "cause"}[kind]
		ord, err := strconv.Atoi(key)
		if err != nil {
			return fmt.Errorf("bad %s ordinal %q", kind, key)
		}
		q = fmt.Sprintf(`UPDATE %s SET %s=? WHERE project_id=? AND ord=?`, kind, col)
		args = []any{text, projectID, ord}
	case "task":
		ord, err := strconv.Atoi(key)
		if err != nil {
			return fmt.Errorf("bad task ordinal %q", key)
		}
		q = `UPDATE task SET text=? WHERE ord=? AND feature_id IN (SELECT id FROM feature WHERE project_id=?)`
		args = []any{text, ord, projectID}
	case "interface":
		q = `UPDATE interface SET sig=? WHERE project_id=? AND name=?`
		args = []any{text, projectID, key}
	case "goal", "constraint":
		tbl := map[string]string{"goal": "goal", "constraint": `"constraint"`}[kind]
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return fmt.Errorf("bad %s id %q", kind, key)
		}
		q = fmt.Sprintf(`UPDATE %s SET text=? WHERE id=?`, tbl)
		args = []any{text, id}
	default:
		return fmt.Errorf("unknown kind %q", kind)
	}

	res, err := db.Exec(q, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no %s %q in this project", kind, key)
	}
	return nil
}

// deprecateInterface flips an interface to deprecated within the project.
// Interfaces are never hard-deleted; deprecation preserves history (V11).
func deprecateInterface(db *sql.DB, projectID int64, name string) error {
	res, err := db.Exec(`UPDATE interface SET status='deprecated' WHERE project_id=? AND name=?`, projectID, name)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no interface %q in this project", name)
	}
	return nil
}

func newAddResearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-research <topic> <finding> <src>",
		Short: "add a durable research finding",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				ord, err := addResearch(db, pid, args[0], args[1], args[2])
				return fmt.Sprintf("R%d", ord), err
			})
		},
	}
}

func newEditCmd() *cobra.Command {
	var text string
	c := &cobra.Command{
		Use:   "edit <kind> <key>",
		Short: "edit a row's text in place (key: ordinal, or interface name)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				return fmt.Sprintf("edited %s %s", args[0], args[1]), editRow(db, pid, args[0], args[1], text)
			})
		},
	}
	c.Flags().StringVar(&text, "text", "", "new text (required)")
	c.MarkFlagRequired("text")
	return c
}

func newDeprecateInterfaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deprecate-interface <name>",
		Short: "mark an interface deprecated (never hard-deleted)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				return fmt.Sprintf("deprecated interface %s", args[0]), deprecateInterface(db, pid, args[0])
			})
		},
	}
}
