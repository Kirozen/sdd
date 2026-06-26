package sdd

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"time"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// clock is the time source for usage last_seen; a package var so tests pin it
// for deterministic output (V114 render). Production uses the wall clock.
var clock = time.Now

// instrument wraps every leaf command's RunE under root so each genuine
// invocation records exactly one usage row (V110). It is applied ONLY by the
// exported NewRootCmd (the process entrypoint, cmd/sdd/main.go) — never the
// unexported newRootCmd that apply reuses per line (apply.go) or that the test
// harness drives directly. That is the re-entrance guard: a single `sdd apply`
// of N lines records one `apply`, not N internal add-* (V110, proven by
// TestApplyDoesNotInflateUsage / T136).
func instrument(root *cobra.Command) {
	for _, c := range root.Commands() {
		instrument(c) // recurse for any future nested subcommands
		if c.RunE == nil {
			continue // no RunE -> never reaches a recorded invocation (V115)
		}
		inner := c.RunE
		c.RunE = func(cc *cobra.Command, args []string) error {
			err := inner(cc, args)
			recordInvocation(cc, err == nil)
			return err
		}
	}
}

// recordInvocation appends one usage count for the resolved command, best-effort
// (V112): any failure here is swallowed and never alters the command's result.
// It opens the EXISTING store without migrating and no-ops if the store file or
// the command_usage table is absent (V116: telemetry never creates or upgrades
// the store). Records the cobra command path (positional args excluded, V110)
// with its ok/fail status — only commands whose RunE was reached are seen, so an
// unknown command or a flag-parse error is never counted (V115).
func recordInvocation(cmd *cobra.Command, ok bool) {
	path := globalDBPath()
	if _, err := os.Stat(path); err != nil {
		return // store absent -> never materialize it from telemetry (V116)
	}
	db, err := open(path)
	if err != nil {
		return // best-effort (V112)
	}
	defer db.Close()

	okD, failD := int64(0), int64(0)
	if ok {
		okD = 1
	} else {
		failD = 1
	}
	// CommandPath is "sdd <cmd>"; strip the root so the recorded name is the
	// command identity (V110). Positional args (e.g. the <kind> of `list`) are
	// deliberately NOT part of it — including them would unbound the table and
	// conflate arguments with commands (V113).
	name := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" ")
	_ = dbq.New(db).UpsertCommandUsage(context.Background(), dbq.UpsertCommandUsageParams{
		ProjectID: bestEffortProjectID(db),
		Command:   name,
		OkCount:   okD,
		FailCount: failD,
		LastSeen:  clock().UTC().Format(time.RFC3339),
	})
	// If command_usage is absent (a store on a schema older than v7), the Upsert
	// errors and is swallowed above — we never migrate from here (V116).
}

// bestEffortProjectID resolves the current project WITHOUT ever creating one
// (lookup, not find-or-create): an unregistered or unresolvable cwd falls back
// to the sentinel 0 bucket (V113). Mirrors the best-effort current-project
// resolution of projects/stats --all (V92/V106) — it never aborts.
func bestEffortProjectID(db *sql.DB) int64 {
	dir, err := os.Getwd()
	if err != nil {
		return 0
	}
	url, hasURL, path, err := projectIdentity(dir)
	if err != nil {
		return 0
	}
	id, found, err := lookupProject(db, url, hasURL, path)
	if err != nil || !found {
		return 0
	}
	return id
}
