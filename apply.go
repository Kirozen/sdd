package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// batchCtx carries the shared transaction for an `sdd apply` run. While it is
// set, runMutation routes every mutation through this one tx and skips its own
// begin/commit/re-export — the batch owns those (V62, V66).
type batchCtx struct {
	tx      *sql.Tx
	lastMsg string
	pid     int64
}

// currentBatch is non-nil only for the duration of an `sdd apply` run. The CLI
// is single-threaded per process, so an ambient pointer is safe.
var currentBatch *batchCtx

// applyVerbs is the create-only family `apply` accepts (the F11 boundary): the
// durable + feature inserts a feature draft strings together. Reads, lifecycle,
// and destructive verbs are rejected. cover is read-only and excluded.
var applyVerbs = map[string]bool{
	"new-feature": true, "add-goal": true, "add-constraint": true,
	"add-task": true, "add-invariant": true, "add-interface": true,
	"add-bug": true, "add-research": true, "add-test": true, "add-unknown": true,
}

// featureBearing reports whether a verb takes the (required) --feature flag, so
// the implicit current feature is injected only for these (V64); the durable,
// project-scoped verbs ignore it.
func featureBearing(verb string) bool {
	switch verb {
	case "add-goal", "add-constraint", "add-task", "add-unknown":
		return true
	}
	return false
}

func hasFeatureFlag(fields []string) bool {
	for _, f := range fields {
		if f == "--feature" || strings.HasPrefix(f, "--feature=") {
			return true
		}
	}
	return false
}

func newApplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "apply TAB-delimited add-* subcommands from stdin in one transaction",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runApply(cmd.InOrStdin())
		},
	}
}

// runApply reads TAB-delimited add-* subcommands (one per line) and applies them
// all in a single transaction (V62), re-exporting SPEC.md exactly once on
// success (V66). Each line is dispatched through the real cobra root, so flag /
// arg parsing is the same as the unitary command — no second parser (V71) — and
// the same mutation cores run (V61); ordinals cited earlier in the batch resolve
// within the shared tx (V63). A new-feature line sets the implicit current
// feature, injected as --feature for feature-bearing verbs that omit it; an
// explicit --feature wins (V64). Lines are split on TAB only — no shell quoting,
// so apostrophes pass intact (V67). Any failing line rolls the whole batch back
// and names the physical 1-based line (V65, V72); blank lines are skipped.
func runApply(r io.Reader) error {
	db, pid, specFile, err := openProjectContext()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	b := &batchCtx{tx: tx, pid: pid}
	currentBatch = b
	defer func() { currentBatch = nil }()

	var currentFeature string
	haveFeature := false

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	applied := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue // V72: blank lines are skipped, not dispatched
		}
		fields := strings.Split(line, "\t")
		verb := fields[0]
		if !applyVerbs[verb] {
			tx.Rollback()
			return fmt.Errorf("line %d: %q is not an apply verb (create-only add-* family)", lineNo, verb)
		}
		if featureBearing(verb) && !hasFeatureFlag(fields) {
			if !haveFeature {
				tx.Rollback()
				return fmt.Errorf("line %d: %s needs a feature — no new-feature before it and no --feature given", lineNo, verb)
			}
			fields = append(fields, "--feature", currentFeature)
		}

		root := newRootCmd()
		root.SetArgs(fields)
		root.SilenceUsage = true
		root.SilenceErrors = true
		root.SetOut(io.Discard)
		if err := root.Execute(); err != nil {
			tx.Rollback()
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
		if verb == "new-feature" {
			currentFeature = b.lastMsg // new-feature's message is its ordinal
			haveFeature = true
		}
		applied++
	}
	if err := sc.Err(); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := exportSpec(db, pid, specFile); err != nil {
		return err
	}
	fmt.Printf("applied %d line(s)\n", applied)
	return nil
}
