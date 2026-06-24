package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.0.1"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "sdd",
		Short:   "spec-driven-dev: SQLite-backed spec engine",
		Version: version,
	}
	root.AddCommand(newInitCmd())
	root.AddCommand(newExportCmd())
	root.AddCommand(newNewFeatureCmd())
	root.AddCommand(newAddGoalCmd())
	root.AddCommand(newAddConstraintCmd())
	root.AddCommand(newAddTaskCmd())
	root.AddCommand(newAddInvariantCmd())
	root.AddCommand(newAddInterfaceCmd())
	root.AddCommand(newAddBugCmd())
	root.AddCommand(newWipeFeatureCmd())
	root.AddCommand(newCheckCmd())
	root.AddCommand(newBackupCmd())
	root.AddCommand(newSetTaskCmd())
	root.AddCommand(newAddResearchCmd())
	root.AddCommand(newEditCmd())
	root.AddCommand(newDeprecateInterfaceCmd())
	root.AddCommand(newImportCmd())
	root.AddCommand(newShowCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newRefsCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
