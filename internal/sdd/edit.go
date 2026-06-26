package sdd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

func addResearch(db dbq.DBTX, projectID int64, topic, finding, src string) (int64, error) {
	ord, err := nextOrd(db, "research", projectID)
	if err != nil {
		return 0, err
	}
	if err := dbq.New(db).InsertResearch(context.Background(), dbq.InsertResearchParams{
		ProjectID: projectID, Ord: int64(ord), Topic: topic, Finding: finding, Src: src,
	}); err != nil {
		return 0, err
	}
	return int64(ord), nil
}

// editRow updates a row's primary text field by the right typed query per kind
// (V50: no interpolated table/column). The row id never changes, so citations
// stay valid (V12); n==0 means the addressed row is absent in this project.
func editRow(db dbq.DBTX, projectID int64, kind, key, text string, featureOrd int64) error {
	ctx := context.Background()
	q := dbq.New(db)
	var (
		n   int64
		err error
	)
	switch kind {
	case "invariant", "research", "bug":
		ord, e := strconv.Atoi(key)
		if e != nil {
			return fmt.Errorf("bad %s ordinal %q", kind, key)
		}
		switch kind {
		case "invariant":
			n, err = q.EditInvariant(ctx, dbq.EditInvariantParams{Text: text, ProjectID: projectID, Ord: int64(ord)})
		case "research":
			n, err = q.EditResearch(ctx, dbq.EditResearchParams{Finding: text, ProjectID: projectID, Ord: int64(ord)})
		case "bug":
			n, err = q.EditBug(ctx, dbq.EditBugParams{Cause: text, ProjectID: projectID, Ord: int64(ord)})
		}
	case "task":
		ord, e := strconv.Atoi(strings.TrimPrefix(key, "T"))
		if e != nil {
			return fmt.Errorf("bad task ordinal %q", key)
		}
		if featureOrd == 0 {
			return fmt.Errorf("edit task needs --feature <f> (task ords are per-feature, V117)")
		}
		n, err = q.EditTask(ctx, dbq.EditTaskParams{Text: text, Ord: int64(ord), ProjectID: projectID, Ord_2: featureOrd})
	case "interface":
		n, err = q.EditInterfaceSig(ctx, dbq.EditInterfaceSigParams{Sig: text, ProjectID: projectID, Name: key})
	default:
		return fmt.Errorf("unknown kind %q", kind)
	}
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
func deprecateInterface(db dbq.DBTX, projectID int64, name string) error {
	n, err := dbq.New(db).DeprecateInterface(context.Background(), dbq.DeprecateInterfaceParams{
		ProjectID: projectID, Name: name,
	})
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
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				ord, err := addResearch(db, pid, args[0], args[1], args[2])
				return fmt.Sprintf("R%d", ord), err
			})
		},
	}
}

// editGoalByPos/editConstraintByPos edit the n-th goal/constraint (1-based,
// ORDER BY id as rendered) of the feature addressed by its ordinal, scoped to the
// current project (V20, V100). Addressing is by POSITION, never by global PK
// (V26/V100) — the B6 trap, where editing by bare PK leaked across the shared
// store. Mirrors rmGoal/rmConstraint. n==0 rows → error (no such feature/position
// in this project).
func editGoalByPos(db dbq.DBTX, projectID, featureOrd int64, n int, text string) error {
	rows, err := dbq.New(db).EditGoalByPosition(context.Background(), dbq.EditGoalByPositionParams{
		Text: text, ProjectID: projectID, Ord: featureOrd, Offset: int64(n - 1),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("no goal #%d in feature %d of this project", n, featureOrd)
	}
	return nil
}

func editConstraintByPos(db dbq.DBTX, projectID, featureOrd int64, n int, text string) error {
	rows, err := dbq.New(db).EditConstraintByPosition(context.Background(), dbq.EditConstraintByPositionParams{
		Text: text, ProjectID: projectID, Ord: featureOrd, Offset: int64(n - 1),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("no constraint #%d in feature %d of this project", n, featureOrd)
	}
	return nil
}

func newEditCmd() *cobra.Command {
	var text string
	var feature int
	c := &cobra.Command{
		Use:   "edit <kind> <key> [--feature <f> for task] | edit goal|constraint <F-ord> <n>",
		Short: "edit a row's text in place (goal/constraint by <F-ord> <n>; task by <T-ord> --feature; others by ordinal/name)",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, rest := args[0], args[1:]
			posKind := kind == "goal" || kind == "constraint"
			if posKind && len(rest) != 2 {
				return fmt.Errorf("edit %s takes <F-ord> <n>", kind)
			}
			if !posKind && len(rest) != 1 {
				return fmt.Errorf("edit %s takes a single <key>", kind)
			}
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				if !posKind {
					return fmt.Sprintf("edited %s %s", kind, rest[0]), editRow(db, pid, kind, rest[0], text, int64(feature))
				}
				fo, n, err := posArgs(rest)
				if err != nil {
					return "", err
				}
				if kind == "goal" {
					return fmt.Sprintf("edited goal #%d of feature %d", n, fo), editGoalByPos(db, pid, fo, n, text)
				}
				return fmt.Sprintf("edited constraint #%d of feature %d", n, fo), editConstraintByPos(db, pid, fo, n, text)
			})
		},
	}
	c.Flags().StringVar(&text, "text", "", "new text (required)")
	c.MarkFlagRequired("text")
	c.Flags().IntVar(&feature, "feature", 0, "feature ordinal owning the task (required for kind=task)")
	return c
}

func newDeprecateInterfaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deprecate-interface <name>",
		Short: "mark an interface deprecated (never hard-deleted)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				return fmt.Sprintf("deprecated interface %s", args[0]), deprecateInterface(db, pid, args[0])
			})
		},
	}
}
