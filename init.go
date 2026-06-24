package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// gitignoreEntries are added to the project .gitignore by init: the db, its WAL
// sidecars, and the generated export — all local working artifacts (§C).
var gitignoreEntries = []string{"spec.db", "spec.db-wal", "spec.db-shm", "SPEC.md"}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "create spec.db + schema in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runInit("."); err != nil {
				return err
			}
			cmd.Println("initialized spec.db")
			return nil
		},
	}
}

// runInit creates spec.db with the schema in dir, refusing to clobber an
// existing db, and ensures the export/db artifacts are gitignored.
func runInit(dir string) error {
	dbPath := filepath.Join(dir, "spec.db")
	if _, err := os.Stat(dbPath); err == nil {
		return fmt.Errorf("spec.db already exists at %s", dbPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	db, err := open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := applySchema(db); err != nil {
		return err
	}

	return ensureGitignore(filepath.Join(dir, ".gitignore"), gitignoreEntries...)
}

// ensureGitignore appends any missing entries to path, preserving existing
// content and creating the file if absent.
func ensureGitignore(path string, entries ...string) error {
	present := map[string]bool{}
	var existing []byte
	if data, err := os.ReadFile(path); err == nil {
		existing = data
		for _, line := range strings.Split(string(data), "\n") {
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
