package sdd

import (
	"github.com/spf13/cobra"
)

// Version is the CLI version string, surfaced on `sdd --version`. The cmd/sdd
// entrypoint sets it from its own package var, which goreleaser injects via
// -X main.version at release build time.
var Version = "dev"

// NewRootCmd is the exported constructor for the sdd root command, used by the
// cmd/sdd entrypoint. It instruments the tree for usage recording (V110) —
// internal callers (apply's per-line dispatch, the test harness) use the
// unexported newRootCmd directly so their re-entrant executions are never
// counted as user invocations.
func NewRootCmd() *cobra.Command {
	root := newRootCmd()
	instrument(root)
	return root
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "sdd",
		Short:   "spec-driven-dev: SQLite-backed spec engine",
		Version: Version,
	}
	root.AddCommand(newInitCmd())
	root.AddCommand(newExportCmd())
	root.AddCommand(newCatCmd())
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
	root.AddCommand(newAddCiteCmd())
	root.AddCommand(newApplyCmd())
	root.AddCommand(newTodoCmd())
	root.AddCommand(newAddResearchCmd())
	root.AddCommand(newEditCmd())
	root.AddCommand(newDeprecateInterfaceCmd())
	root.AddCommand(newImportCmd())
	root.AddCommand(newShowCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newRefsCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newNextCmd())
	root.AddCommand(newAddUnknownCmd())
	root.AddCommand(newResolveUnknownCmd())
	root.AddCommand(newGuideCmd())
	root.AddCommand(newAddTestCmd())
	root.AddCommand(newCoverCmd())
	root.AddCommand(newGateCmd())
	root.AddCommand(newRmTaskCmd())
	root.AddCommand(newRetractInvariantCmd())
	root.AddCommand(newRetractInterfaceCmd())
	root.AddCommand(newRmGoalCmd())
	root.AddCommand(newRmConstraintCmd())
	root.AddCommand(newProjectsCmd())
	root.AddCommand(newSearchCmd())
	root.AddCommand(newStatsCmd())
	root.AddCommand(newUsageCmd())
	return root
}
