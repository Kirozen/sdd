package sdd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// --- core mutations (txn-wrapped, testable without cobra) ---
//
// Durable rows + features are project-scoped (V20) and carry a per-project
// display ordinal (V26) assigned at insert. Functions return that ordinal (the
// V<n>/T<n>/B<n>/FEATURE<n> the user sees), not the global PK.

// addFeature inserts a feature with the next per-project ordinal and returns its
// global PK (callers attach goals/constraints/tasks to it).
func addFeature(db dbq.DBTX, projectID int64, name string) (int64, error) {
	ord, err := nextOrd(db, "feature", projectID)
	if err != nil {
		return 0, err
	}
	return dbq.New(db).InsertFeature(context.Background(), dbq.InsertFeatureParams{
		ProjectID: projectID, Ord: int64(ord), Name: name,
	})
}

// featurePK resolves a feature's per-project ordinal to its global PK. Accepts
// dbq.DBTX so it runs on *sql.DB or inside a *sql.Tx.
func featurePK(q dbq.DBTX, projectID, ord int64) (int64, error) {
	pk, err := dbq.New(q).FeaturePK(context.Background(), dbq.FeaturePKParams{
		ProjectID: projectID, Ord: ord,
	})
	if err != nil {
		return 0, fmt.Errorf("no feature %d in this project", ord)
	}
	return pk, nil
}

func addGoal(db dbq.DBTX, featurePK int64, text string) error {
	return dbq.New(db).InsertGoal(context.Background(), dbq.InsertGoalParams{FeatureID: featurePK, Text: text})
}

func addConstraint(db dbq.DBTX, featurePK int64, text string) error {
	return dbq.New(db).InsertConstraint(context.Background(), dbq.InsertConstraintParams{FeatureID: featurePK, Text: text})
}

func addInvariant(db dbq.DBTX, projectID int64, text string) (int64, error) {
	ord, err := nextOrd(db, "invariant", projectID)
	if err != nil {
		return 0, err
	}
	if err := dbq.New(db).InsertInvariant(context.Background(), dbq.InsertInvariantParams{
		ProjectID: projectID, Ord: int64(ord), Text: text,
	}); err != nil {
		return 0, err
	}
	return int64(ord), nil
}

func addInterface(db dbq.DBTX, projectID int64, kind, name, sig string) (int64, error) {
	return dbq.New(db).InsertInterface(context.Background(), dbq.InsertInterfaceParams{
		ProjectID: projectID, Kind: kind, Name: name, Sig: sig,
	})
}

// addTask inserts a task and its cites in one transaction: a single bad cite
// rolls the whole thing back, so no orphan task survives (V2, V5). The task
// belongs to projectID (via featurePK); cites resolve within that project (V20).
func addTask(db dbq.DBTX, projectID, featurePK int64, text string, cites []string) (int64, error) {
	ord, err := nextTaskOrd(db, featurePK)
	if err != nil {
		return 0, err
	}
	taskID, err := dbq.New(db).InsertTask(context.Background(), dbq.InsertTaskParams{
		FeatureID: featurePK, Ord: int64(ord), Text: text,
	})
	if err != nil {
		return 0, err
	}
	for _, c := range cites {
		if err := insertCite(db, projectID, taskID, c); err != nil {
			return 0, err
		}
	}
	return taskID, nil
}

// insertCite links a task to a cited row, resolving the cite within the task's
// project (V20): V<ord> → the invariant with that ord, I.<name> → the interface
// with that name. A cross-project or unknown ord/name finds nothing and rejects.
func insertCite(db dbq.DBTX, projectID, taskID int64, cite string) error {
	ctx := context.Background()
	q := dbq.New(db)
	switch {
	case strings.HasPrefix(cite, "V"):
		ord, err := strconv.Atoi(cite[1:])
		if err != nil {
			return fmt.Errorf("bad invariant cite %q", cite)
		}
		invID, err := q.InvariantIDByOrd(ctx, dbq.InvariantIDByOrdParams{ProjectID: projectID, Ord: int64(ord)})
		if err != nil {
			return fmt.Errorf("unknown invariant cite %q in this project", cite)
		}
		if err := q.InsertTaskCiteInv(ctx, dbq.InsertTaskCiteInvParams{TaskID: taskID, InvID: invID}); err != nil {
			return fmt.Errorf("cite %s: %w", cite, err)
		}
	case strings.HasPrefix(cite, "I."):
		name := cite[2:]
		ifaceID, err := q.InterfaceIDByName(ctx, dbq.InterfaceIDByNameParams{ProjectID: projectID, Name: name})
		if err != nil {
			return fmt.Errorf("unknown interface cite %q in this project", cite)
		}
		if err := q.InsertTaskCiteIface(ctx, dbq.InsertTaskCiteIfaceParams{TaskID: taskID, IfaceID: ifaceID}); err != nil {
			return fmt.Errorf("cite %s: %w", cite, err)
		}
	default:
		return fmt.Errorf("unrecognized cite %q (want V<n> or I.<name>)", cite)
	}
	return nil
}

// addBug inserts a bug and its fix links in one transaction. Fix refs resolve
// to invariants by per-project ordinal (V20, V26).
func addBug(db dbq.DBTX, projectID int64, date, cause string, fixRefs []string) (int64, error) {
	ord, err := nextOrd(db, "bug", projectID)
	if err != nil {
		return 0, err
	}
	ctx := context.Background()
	q := dbq.New(db)
	bugID, err := q.InsertBug(ctx, dbq.InsertBugParams{
		ProjectID: projectID, Ord: int64(ord), Date: date, Cause: cause,
	})
	if err != nil {
		return 0, err
	}
	for _, ref := range fixRefs {
		n, err := strconv.Atoi(strings.TrimPrefix(ref, "V"))
		if err != nil {
			return 0, fmt.Errorf("bad fix invariant %q", ref)
		}
		invID, err := q.InvariantIDByOrd(ctx, dbq.InvariantIDByOrdParams{ProjectID: projectID, Ord: int64(n)})
		if err != nil {
			return 0, fmt.Errorf("unknown fix invariant %q in this project", ref)
		}
		if err := q.InsertBugFix(ctx, dbq.InsertBugFixParams{BugID: bugID, InvID: invID}); err != nil {
			return 0, fmt.Errorf("fix %s: %w", ref, err)
		}
	}
	return int64(ord), nil
}

// runMutation opens the global db, resolves the current project, runs the
// mutation fn inside it, then re-exports SPEC.md atomically (V8). The returned
// message (if any) is printed on success.
func runMutation(fn func(dbq.DBTX, int64) (string, error)) error {
	// Inside an `sdd apply` run the batch owns the tx, the commit, and the single
	// re-export (V62, V66): route the mutation through the shared tx and return.
	if b := currentBatch; b != nil {
		msg, err := fn(b.tx, b.pid)
		if err != nil {
			return err
		}
		b.lastMsg = msg
		return nil
	}
	db, pid, specFile, err := openProjectContext()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	msg, err := fn(tx, pid)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
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

// splitRefs parses a comma list of cite refs, dropping blanks and the "-"
// sentinel (so an empty cites column round-trips to no cites, B1/V15).
func splitRefs(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" && p != "-" {
			out = append(out, p)
		}
	}
	return out
}

func newNewFeatureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new-feature <name>",
		Short: "create a feature, print its number",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				pk, err := addFeature(db, pid, args[0])
				if err != nil {
					return "", err
				}
				ord, _ := dbq.New(db).FeatureOrdByID(context.Background(), pk)
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
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
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
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
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
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				pk, err := featurePK(db, pid, feature)
				if err != nil {
					return "", err
				}
				taskPK, err := addTask(db, pid, pk, args[0], splitRefs(cites))
				if err != nil {
					return "", err
				}
				ord, _ := dbq.New(db).TaskOrdByID(context.Background(), taskPK)
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
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
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
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
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
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
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
