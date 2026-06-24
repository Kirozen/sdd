package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// --- core mutations (txn-wrapped, testable without cobra) ---

func addFeature(db *sql.DB, name string) (int64, error) {
	res, err := db.Exec(`INSERT INTO feature(name) VALUES(?)`, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func addGoal(db *sql.DB, featureID int64, text string) error {
	_, err := db.Exec(`INSERT INTO goal(feature_id, text) VALUES(?, ?)`, featureID, text)
	return err
}

func addConstraint(db *sql.DB, featureID int64, text string) error {
	_, err := db.Exec(`INSERT INTO "constraint"(feature_id, text) VALUES(?, ?)`, featureID, text)
	return err
}

func addInvariant(db *sql.DB, text string) (int64, error) {
	res, err := db.Exec(`INSERT INTO invariant(text) VALUES(?)`, text)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func addInterface(db *sql.DB, kind, name, sig string) (int64, error) {
	res, err := db.Exec(`INSERT INTO interface(kind, name, sig) VALUES(?, ?, ?)`, kind, name, sig)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// addTask inserts a task and its cites in one transaction: a single bad cite
// rolls the whole thing back, so no orphan task survives (V2, V5).
func addTask(db *sql.DB, featureID int64, text string, cites []string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO task(feature_id, text) VALUES(?, ?)`, featureID, text)
	if err != nil {
		return 0, err
	}
	taskID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, c := range cites {
		if err := insertCite(tx, taskID, c); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return taskID, nil
}

func insertCite(tx *sql.Tx, taskID int64, cite string) error {
	switch {
	case strings.HasPrefix(cite, "V"):
		n, err := strconv.Atoi(cite[1:])
		if err != nil {
			return fmt.Errorf("bad invariant cite %q", cite)
		}
		if _, err := tx.Exec(`INSERT INTO task_cites_inv(task_id, inv_id) VALUES(?, ?)`, taskID, n); err != nil {
			return fmt.Errorf("cite %s: %w", cite, err)
		}
	case strings.HasPrefix(cite, "I."):
		name := cite[2:]
		var ifaceID int64
		if err := tx.QueryRow(`SELECT id FROM interface WHERE name=?`, name).Scan(&ifaceID); err != nil {
			return fmt.Errorf("unknown interface cite %q", cite)
		}
		if _, err := tx.Exec(`INSERT INTO task_cites_iface(task_id, iface_id) VALUES(?, ?)`, taskID, ifaceID); err != nil {
			return fmt.Errorf("cite %s: %w", cite, err)
		}
	default:
		return fmt.Errorf("unrecognized cite %q (want V<n> or I.<name>)", cite)
	}
	return nil
}

// addBug inserts a bug and its fix links in one transaction.
func addBug(db *sql.DB, date, cause string, fixRefs []string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO bug(date, cause) VALUES(?, ?)`, date, cause)
	if err != nil {
		return 0, err
	}
	bugID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, ref := range fixRefs {
		n, err := strconv.Atoi(strings.TrimPrefix(ref, "V"))
		if err != nil {
			return 0, fmt.Errorf("bad fix invariant %q", ref)
		}
		if _, err := tx.Exec(`INSERT INTO bug_fix(bug_id, inv_id) VALUES(?, ?)`, bugID, n); err != nil {
			return 0, fmt.Errorf("fix %s: %w", ref, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return bugID, nil
}

// splitRefs parses a comma list, dropping blanks.
func splitRefs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// --- cobra wrappers: open db, mutate, re-export (V2) ---

// runMutation opens the project db, runs fn, and re-exports on success.
func runMutation(fn func(*sql.DB) (string, error)) error {
	db, err := openProjectDB()
	if err != nil {
		return err
	}
	defer db.Close()
	msg, err := fn(db)
	if err != nil {
		return err
	}
	if err := exportSpec(db, specPath); err != nil {
		return err
	}
	if msg != "" {
		fmt.Println(msg)
	}
	return nil
}

func newNewFeatureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new-feature <name>",
		Short: "create a feature, print its id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB) (string, error) {
				id, err := addFeature(db, args[0])
				return fmt.Sprintf("%d", id), err
			})
		},
	}
}

func newAddGoalCmd() *cobra.Command {
	var feature int64
	c := &cobra.Command{
		Use:   "add-goal <text>",
		Short: "add a goal to a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB) (string, error) {
				return "", addGoal(db, feature, args[0])
			})
		},
	}
	c.Flags().Int64Var(&feature, "feature", 0, "feature id (required)")
	c.MarkFlagRequired("feature")
	return c
}

func newAddConstraintCmd() *cobra.Command {
	var feature int64
	c := &cobra.Command{
		Use:   "add-constraint <text>",
		Short: "add a constraint to a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB) (string, error) {
				return "", addConstraint(db, feature, args[0])
			})
		},
	}
	c.Flags().Int64Var(&feature, "feature", 0, "feature id (required)")
	c.MarkFlagRequired("feature")
	return c
}

func newAddTaskCmd() *cobra.Command {
	var feature int64
	var cites string
	c := &cobra.Command{
		Use:   "add-task <text>",
		Short: "add a task to a feature, citing invariants/interfaces",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB) (string, error) {
				id, err := addTask(db, feature, args[0], splitRefs(cites))
				return fmt.Sprintf("T%d", id), err
			})
		},
	}
	c.Flags().Int64Var(&feature, "feature", 0, "feature id (required)")
	c.Flags().StringVar(&cites, "cites", "", "comma list of V<n>/I.<name>")
	c.MarkFlagRequired("feature")
	return c
}

func newAddInvariantCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-invariant <text>",
		Short: "add a durable invariant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB) (string, error) {
				id, err := addInvariant(db, args[0])
				return fmt.Sprintf("V%d", id), err
			})
		},
	}
}

func newAddInterfaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-interface <kind> <name> <sig>",
		Short: "add a durable interface (name is the cite key I.<name>)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB) (string, error) {
				_, err := addInterface(db, args[0], args[1], args[2])
				return "I." + args[1], err
			})
		},
	}
}

func newAddBugCmd() *cobra.Command {
	var fix, date string
	c := &cobra.Command{
		Use:   "add-bug <cause>",
		Short: "record a durable bug, linking the invariant(s) that catch it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			return runMutation(func(db *sql.DB) (string, error) {
				id, err := addBug(db, date, args[0], splitRefs(fix))
				return fmt.Sprintf("B%d", id), err
			})
		},
	}
	c.Flags().StringVar(&fix, "fix", "", "comma list of invariants V<n> (required)")
	c.Flags().StringVar(&date, "date", "", "ISO date (default: today)")
	c.MarkFlagRequired("fix")
	return c
}
