package sdd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	dbq "github.com/kirozen/sdd/internal/db"
)

// openProjectContext opens the global db and resolves the current project from
// cwd, returning the worktree-root SPEC.md path for export/check (V22, V23).
func openProjectContext() (*sql.DB, int64, string, error) {
	db, err := openGlobalDB()
	if err != nil {
		return nil, 0, "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		db.Close()
		return nil, 0, "", err
	}
	root, err := mainWorktree(cwd)
	if err != nil {
		db.Close()
		return nil, 0, "", fmt.Errorf("not in a git project: %w", err)
	}
	pid, err := resolveProject(db, cwd)
	if err != nil {
		db.Close()
		return nil, 0, "", err
	}
	return db, pid, filepath.Join(root, "SPEC.md"), nil
}

// nextOrd returns the next per-project display ordinal for a durable/feature
// table (V26): max(ord)+1 over the project's rows. The runtime table name is now
// a switch over one typed query per kind (V50) — no interpolated SQL. Accepts
// dbq.DBTX so it runs against *sql.DB or a *sql.Tx (e.g. inside addBug).
func nextOrd(q dbq.DBTX, table string, projectID int64) (int, error) {
	ctx := context.Background()
	qs := dbq.New(q)
	pid := projectID
	var (
		n   int64
		err error
	)
	switch table {
	case "invariant":
		n, err = qs.NextInvariantOrd(ctx, pid)
	case "bug":
		n, err = qs.NextBugOrd(ctx, pid)
	case "research":
		n, err = qs.NextResearchOrd(ctx, pid)
	case "feature":
		n, err = qs.NextFeatureOrd(ctx, pid)
	default:
		return 0, fmt.Errorf("nextOrd: unknown table %q", table)
	}
	return int(n), err
}

// nextTaskOrd is nextOrd for tasks, whose project is reached through feature.
func nextTaskOrd(q dbq.DBTX, projectID int64) (int, error) {
	n, err := dbq.New(q).NextTaskOrd(context.Background(), projectID)
	return int(n), err
}

// gitOutput runs git in dir and returns trimmed stdout.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// mainWorktree returns the absolute path of the repo's MAIN worktree, resolved
// from any directory inside it — including a linked worktree, whose own
// show-toplevel would point at itself. `git worktree list` lists the main
// worktree first (R8, V27).
func mainWorktree(dir string) (string, error) {
	out, err := gitOutput(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("not inside a git worktree: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			return strings.TrimSpace(p), nil
		}
	}
	return "", fmt.Errorf("no worktree in `git worktree list` output")
}

// gitRemoteURL returns the canonical origin url, or ok=false when there is no
// origin (R9). origin is the convention; other remotes are not guessed.
func gitRemoteURL(dir string) (string, bool) {
	out, err := gitOutput(dir, "remote", "get-url", "origin")
	if err != nil || out == "" {
		return "", false
	}
	return canonURL(out), true
}

// canonURL collapses the equivalent forms of a git url to a single key
// host/path (C28): strip scheme, user (git@), trailing .git and slash; rewrite
// the scp-like host:path to host/path; lowercase the host, preserve path case.
func canonURL(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")

	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if at := strings.Index(s, "@"); at >= 0 {
		if slash := strings.Index(s, "/"); slash < 0 || at < slash {
			s = s[at+1:]
		}
	}
	// scp-like "host:path" → "host/path" (only when the colon precedes any slash)
	if c := strings.Index(s, ":"); c >= 0 {
		if slash := strings.Index(s, "/"); slash < 0 || c < slash {
			s = s[:c] + "/" + s[c+1:]
		}
	}
	if slash := strings.Index(s, "/"); slash >= 0 {
		return strings.ToLower(s[:slash]) + s[slash:]
	}
	return strings.ToLower(s)
}

// projectIdentity computes the dual key for the repo at dir: the main worktree
// path (always) and the canonical origin url (when present).
func projectIdentity(dir string) (url string, hasURL bool, path string, err error) {
	path, err = mainWorktree(dir)
	if err != nil {
		return "", false, "", err
	}
	url, hasURL = gitRemoteURL(dir)
	return url, hasURL, path, nil
}

// lookupProject finds a project by the dual key, url first (so clones of one
// remote share a project), then path.
func lookupProject(db *sql.DB, url string, hasURL bool, path string) (int64, bool, error) {
	q := dbq.New(db)
	ctx := context.Background()
	if hasURL {
		switch id, err := q.ProjectByURL(ctx, sql.NullString{String: url, Valid: true}); err {
		case nil:
			return id, true, nil
		case sql.ErrNoRows:
			// fall through to path
		default:
			return 0, false, err
		}
	}
	switch id, err := q.ProjectByPath(ctx, path); err {
	case nil:
		return id, true, nil
	case sql.ErrNoRows:
		return 0, false, nil
	default:
		return 0, false, err
	}
}

// backfillURL records a url on a project that was first seen without one (a
// remote added after init), so adding a remote never orphans the project (V21).
func backfillURL(db *sql.DB, id int64, url string, hasURL bool) {
	if hasURL {
		dbq.New(db).BackfillProjectURL(context.Background(), dbq.BackfillProjectURLParams{
			Url: sql.NullString{String: url, Valid: true}, ID: id,
		})
	}
}

// findOrCreateProject resolves the project for dir, creating it if new. Used by
// init.
func findOrCreateProject(db *sql.DB, dir string) (int64, error) {
	url, hasURL, path, err := projectIdentity(dir)
	if err != nil {
		return 0, err
	}
	id, found, err := lookupProject(db, url, hasURL, path)
	if err != nil {
		return 0, err
	}
	if found {
		backfillURL(db, id, url, hasURL)
		return id, nil
	}
	u := sql.NullString{}
	if hasURL {
		u = sql.NullString{String: url, Valid: true}
	}
	return dbq.New(db).CreateProject(context.Background(), dbq.CreateProjectParams{Url: u, Path: path})
}

// resolveProject returns the project for dir, erroring if none is registered —
// commands never operate on an unknown project (V23).
func resolveProject(db *sql.DB, dir string) (int64, error) {
	url, hasURL, path, err := projectIdentity(dir)
	if err != nil {
		return 0, fmt.Errorf("not in a git project: %w", err)
	}
	id, found, err := lookupProject(db, url, hasURL, path)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fmt.Errorf("no sdd project here; run `sdd init` first")
	}
	backfillURL(db, id, url, hasURL)
	return id, nil
}
