package main

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func addResearch(db *sql.DB, topic, finding, src string) (int64, error) {
	res, err := db.Exec(`INSERT INTO research(topic, finding, src) VALUES(?, ?, ?)`, topic, finding, src)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// editTargets whitelists which table+column an `edit <kind>` touches. The kind
// is user input, so the table/column are looked up here, never interpolated raw.
var editTargets = map[string][2]string{
	"invariant":  {"invariant", "text"},
	"interface":  {"interface", "sig"},
	"bug":        {"bug", "cause"},
	"research":   {"research", "finding"},
	"goal":       {"goal", "text"},
	"constraint": {`"constraint"`, "text"},
	"task":       {"task", "text"},
}

// editRow updates a row's primary text field in place. The id is the key, never
// changed, so any citation pointing at it stays valid (V12).
func editRow(db *sql.DB, kind string, id int64, text string) error {
	tgt, ok := editTargets[kind]
	if !ok {
		return fmt.Errorf("unknown kind %q", kind)
	}
	q := fmt.Sprintf(`UPDATE %s SET %s=? WHERE id=?`, tgt[0], tgt[1])
	res, err := db.Exec(q, text, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no %s with id %d", kind, id)
	}
	return nil
}

// deprecateInterface flips an interface to deprecated. Interfaces are never
// hard-deleted; deprecation preserves history (V11).
func deprecateInterface(db *sql.DB, id int64) error {
	res, err := db.Exec(`UPDATE interface SET status='deprecated' WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no interface with id %d", id)
	}
	return nil
}

func newAddResearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-research <topic> <finding> <src>",
		Short: "add a durable research finding",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB) (string, error) {
				id, err := addResearch(db, args[0], args[1], args[2])
				return fmt.Sprintf("R%d", id), err
			})
		},
	}
}

func newEditCmd() *cobra.Command {
	var text string
	c := &cobra.Command{
		Use:   "edit <kind> <id>",
		Short: "edit a row's text in place (id stays stable)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("bad id %q", args[1])
			}
			return runMutation(func(db *sql.DB) (string, error) {
				return fmt.Sprintf("edited %s %d", args[0], id), editRow(db, args[0], id, text)
			})
		},
	}
	c.Flags().StringVar(&text, "text", "", "new text (required)")
	c.MarkFlagRequired("text")
	return c
}

func newDeprecateInterfaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deprecate-interface <id>",
		Short: "mark an interface deprecated (never hard-deleted)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("bad interface id %q", args[0])
			}
			return runMutation(func(db *sql.DB) (string, error) {
				return fmt.Sprintf("deprecated interface %d", id), deprecateInterface(db, id)
			})
		},
	}
}
