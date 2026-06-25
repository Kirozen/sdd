package sdd

import (
	"context"
	"fmt"
	"strconv"

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
func editRow(db dbq.DBTX, projectID int64, kind, key, text string) error {
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
		ord, e := strconv.Atoi(key)
		if e != nil {
			return fmt.Errorf("bad task ordinal %q", key)
		}
		n, err = q.EditTask(ctx, dbq.EditTaskParams{Text: text, Ord: int64(ord), ProjectID: projectID})
	case "interface":
		n, err = q.EditInterfaceSig(ctx, dbq.EditInterfaceSigParams{Sig: text, ProjectID: projectID, Name: key})
	case "goal", "constraint":
		id, e := strconv.ParseInt(key, 10, 64)
		if e != nil {
			return fmt.Errorf("bad %s id %q", kind, key)
		}
		if kind == "goal" {
			n, err = q.EditGoal(ctx, dbq.EditGoalParams{Text: text, ID: id})
		} else {
			n, err = q.EditConstraint(ctx, dbq.EditConstraintParams{Text: text, ID: id})
		}
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

func newEditCmd() *cobra.Command {
	var text string
	c := &cobra.Command{
		Use:   "edit <kind> <key>",
		Short: "edit a row's text in place (key: ordinal, or interface name)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
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
			return runMutation(func(db dbq.DBTX, pid int64) (string, error) {
				return fmt.Sprintf("deprecated interface %s", args[0]), deprecateInterface(db, pid, args[0])
			})
		},
	}
}
