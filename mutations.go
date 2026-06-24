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
//
// Durable rows + features are project-scoped (V20) and carry a per-project
// display ordinal (V26) assigned at insert. Functions return that ordinal (the
// V<n>/T<n>/B<n>/FEATURE<n> the user sees), not the global PK.

// addFeature inserts a feature with the next per-project ordinal and returns its
// global PK (callers attach goals/constraints/tasks to it).
func addFeature(db *sql.DB, projectID int64, name string) (int64, error) {
	ord, err := nextOrd(db, "feature", projectID)
	if err != nil {
		return 0, err
	}
	res, err := db.Exec(`INSERT INTO feature(project_id, ord, name) VALUES(?, ?, ?)`, projectID, ord, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// featurePK resolves a feature's per-project ordinal to its global PK.
func featurePK(q queryer, projectID, ord int64) (int64, error) {
	var pk int64
	if err := q.QueryRow(`SELECT id FROM feature WHERE project_id=? AND ord=?`, projectID, ord).Scan(&pk); err != nil {
		return 0, fmt.Errorf("no feature %d in this project", ord)
	}
	return pk, nil
}

func addGoal(db *sql.DB, featurePK int64, text string) error {
	_, err := db.Exec(`INSERT INTO goal(feature_id, text) VALUES(?, ?)`, featurePK, text)
	return err
}

func addConstraint(db *sql.DB, featurePK int64, text string) error {
	_, err := db.Exec(`INSERT INTO "constraint"(feature_id, text) VALUES(?, ?)`, featurePK, text)
	return err
}

func addInvariant(db *sql.DB, projectID int64, text string) (int64, error) {
	ord, err := nextOrd(db, "invariant", projectID)
	if err != nil {
		return 0, err
	}
	if _, err := db.Exec(`INSERT INTO invariant(project_id, ord, text) VALUES(?, ?, ?)`, projectID, ord, text); err != nil {
		return 0, err
	}
	return int64(ord), nil
}

func addInterface(db *sql.DB, projectID int64, kind, name, sig string) (int64, error) {
	res, err := db.Exec(`INSERT INTO interface(project_id, kind, name, sig) VALUES(?, ?, ?, ?)`, projectID, kind, name, sig)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// addTask inserts a task and its cites in one transaction: a single bad cite
// rolls the whole thing back, so no orphan task survives (V2, V5). The task
// belongs to projectID (via featurePK); cites resolve within that project (V20).
func addTask(db *sql.DB, projectID, featurePK int64, text string, cites []string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	ord, err := nextTaskOrd(tx, projectID)
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(`INSERT INTO task(feature_id, ord, text) VALUES(?, ?, ?)`, featurePK, ord, text)
	if err != nil {
		return 0, err
	}
	taskID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, c := range cites {
		if err := insertCite(tx, projectID, taskID, c); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return taskID, nil
}

// insertCite links a task to a cited row, resolving the cite within the task's
// project (V20): V<ord> → the invariant with that ord, I.<name> → the interface
// with that name. A cross-project or unknown ord/name finds nothing and rejects.
func insertCite(tx *sql.Tx, projectID, taskID int64, cite string) error {
	switch {
	case strings.HasPrefix(cite, "V"):
		ord, err := strconv.Atoi(cite[1:])
		if err != nil {
			return fmt.Errorf("bad invariant cite %q", cite)
		}
		var invID int64
		if err := tx.QueryRow(`SELECT id FROM invariant WHERE project_id=? AND ord=?`, projectID, ord).Scan(&invID); err != nil {
			return fmt.Errorf("unknown invariant cite %q in this project", cite)
		}
		if _, err := tx.Exec(`INSERT INTO task_cites_inv(task_id, inv_id) VALUES(?, ?)`, taskID, invID); err != nil {
			return fmt.Errorf("cite %s: %w", cite, err)
		}
	case strings.HasPrefix(cite, "I."):
		name := cite[2:]
		var ifaceID int64
		if err := tx.QueryRow(`SELECT id FROM interface WHERE project_id=? AND name=?`, projectID, name).Scan(&ifaceID); err != nil {
			return fmt.Errorf("unknown interface cite %q in this project", cite)
		}
		if _, err := tx.Exec(`INSERT INTO task_cites_iface(task_id, iface_id) VALUES(?, ?)`, taskID, ifaceID); err != nil {
			return fmt.Errorf("cite %s: %w", cite, err)
		}
	default:
		return fmt.Errorf("unrecognized cite %q (want V<n> or I.<name>)", cite)
	}
	return nil
}

// addBug inserts a bug and its fix links in one transaction. Fix refs resolve
// to invariants by per-project ordinal (V20, V26).
func addBug(db *sql.DB, projectID int64, date, cause string, fixRefs []string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	ord, err := nextOrd(tx, "bug", projectID)
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(`INSERT INTO bug(project_id, ord, date, cause) VALUES(?, ?, ?, ?)`, projectID, ord, date, cause)
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
		var invID int64
		if err := tx.QueryRow(`SELECT id FROM invariant WHERE project_id=? AND ord=?`, projectID, n).Scan(&invID); err != nil {
			return 0, fmt.Errorf("unknown fix invariant %q in this project", ref)
		}
		if _, err := tx.Exec(`INSERT INTO bug_fix(bug_id, inv_id) VALUES(?, ?)`, bugID, invID); err != nil {
			return 0, fmt.Errorf("fix %s: %w", ref, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(ord), nil
}

// splitRefs parses a comma list, dropping blanks and the `-` empty sentinel
// (FORMAT.md renders "no refs" as `-`).
func splitRefs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" && p != "-" {
			out = append(out, p)
		}
	}
	return out
}

// --- cobra wrappers: resolve project, mutate, re-export (V2) ---

// runMutation opens the global db, resolves the current project, runs fn, and
// re-exports the project's SPEC.md at the worktree root on success.
func runMutation(fn func(*sql.DB, int64) (string, error)) error {
	db, pid, specFile, err := openProjectContext()
	if err != nil {
		return err
	}
	defer db.Close()
	msg, err := fn(db, pid)
	if err != nil {
		return err
	}
	if err := exportSpec(db, pid, specFile); err != nil {
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
		Short: "create a feature, print its number",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				pk, err := addFeature(db, pid, args[0])
				if err != nil {
					return "", err
				}
				var ord int
				db.QueryRow(`SELECT ord FROM feature WHERE id=?`, pk).Scan(&ord)
				return fmt.Sprintf("%d", ord), nil
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
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				pk, err := featurePK(db, pid, feature)
				if err != nil {
					return "", err
				}
				return "", addGoal(db, pk, args[0])
			})
		},
	}
	c.Flags().Int64Var(&feature, "feature", 0, "feature number (required)")
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
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				pk, err := featurePK(db, pid, feature)
				if err != nil {
					return "", err
				}
				return "", addConstraint(db, pk, args[0])
			})
		},
	}
	c.Flags().Int64Var(&feature, "feature", 0, "feature number (required)")
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
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				pk, err := featurePK(db, pid, feature)
				if err != nil {
					return "", err
				}
				taskPK, err := addTask(db, pid, pk, args[0], splitRefs(cites))
				if err != nil {
					return "", err
				}
				var ord int
				db.QueryRow(`SELECT ord FROM task WHERE id=?`, taskPK).Scan(&ord)
				return fmt.Sprintf("T%d", ord), nil
			})
		},
	}
	c.Flags().Int64Var(&feature, "feature", 0, "feature number (required)")
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
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				ord, err := addInvariant(db, pid, args[0])
				return fmt.Sprintf("V%d", ord), err
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
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				_, err := addInterface(db, pid, args[0], args[1], args[2])
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
			return runMutation(func(db *sql.DB, pid int64) (string, error) {
				ord, err := addBug(db, pid, date, args[0], splitRefs(fix))
				return fmt.Sprintf("B%d", ord), err
			})
		},
	}
	c.Flags().StringVar(&fix, "fix", "", "comma list of invariants V<n> (required)")
	c.Flags().StringVar(&date, "date", "", "ISO date (default: today)")
	c.MarkFlagRequired("fix")
	return c
}
