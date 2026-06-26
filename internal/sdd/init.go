package sdd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// gitignoreEntries are added to the project .gitignore by init. The db now lives
// in the global store outside the repo (V22), so only the generated export is
// gitignored locally (N-7).
var gitignoreEntries = []string{"SPEC.md"}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "register the current repo as a project in the global store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if err := runInit(cwd); err != nil {
				return err
			}
			cmd.Println("initialized project in the global store")
			return nil
		},
	}
}

// runInit ensures the global db exists and registers (find-or-create) the repo
// at dir as a project (V20, V22). Idempotent — re-running on a known project is
// a no-op find. Gitignores the local SPEC.md and writes the project's (possibly
// empty) export at the worktree root.
func runInit(dir string) error {
	db, err := openGlobalDB()
	if err != nil {
		return err
	}
	defer db.Close()

	pid, err := findOrCreateProject(db, dir)
	if err != nil {
		return err
	}
	root, err := mainWorktree(dir)
	if err != nil {
		return err
	}
	if err := ensureGitignore(filepath.Join(root, ".gitignore"), gitignoreEntries...); err != nil {
		return err
	}
	return exportSpec(db, pid, filepath.Join(root, specName))
}

// ensureGitignore appends any missing entries to path, preserving existing
// content and creating the file if absent.
func ensureGitignore(path string, entries ...string) error {
	present := map[string]bool{}
	var existing []byte
	if data, err := os.ReadFile(path); err == nil {
		existing = data
		for line := range strings.SplitSeq(string(data), "\n") {
			present[strings.TrimSpace(line)] = true
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	var add []string
	for _, e := range entries {
		if !present[e] {
			add = append(add, e)
		}
	}
	if len(add) == 0 {
		return nil
	}

	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		existing = append(existing, '\n')
	}
	existing = append(existing, []byte(strings.Join(add, "\n")+"\n")...)
	return os.WriteFile(path, existing, 0o644)
}
