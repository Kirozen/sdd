package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.0.1"

func newRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "sdd",
		Short:   "spec-driven-dev: SQLite-backed spec engine",
		Version: version,
	}
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
