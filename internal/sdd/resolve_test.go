package sdd

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	return dir
}

// C28: equivalent url forms collapse to one host/path key, host lowercased,
// path case preserved.
func TestCanonURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/user/repo.git": "github.com/user/repo",
		"git@github.com:user/repo.git":     "github.com/user/repo",
		"ssh://git@github.com/user/repo":   "github.com/user/repo",
		"https://github.com/user/repo/":    "github.com/user/repo",
		"git://github.com/user/repo.git":   "github.com/user/repo",
		"https://GitHub.com/User/Repo.git": "github.com/User/Repo",
	}
	for in, want := range cases {
		if got := canonURL(in); got != want {
			t.Errorf("canonURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// V21: a repo first seen without a remote is keyed by path; adding a remote
// later backfills the url onto the SAME project — no orphan.
func TestDualKeyBackfill(t *testing.T) {
	dir := gitRepo(t)
	db := openTestDB(t)

	id1, err := findOrCreateProject(db, dir)
	if err != nil {
		t.Fatalf("findOrCreateProject: %v", err)
	}
	var url sql.NullString
	db.QueryRow(`SELECT url FROM project WHERE id=?`, id1).Scan(&url)
	if url.Valid {
		t.Errorf("url = %q before any remote, want NULL", url.String)
	}

	gitRun(t, dir, "remote", "add", "origin", "git@github.com:U/Repo.git")
	id2, err := resolveProject(db, dir)
	if err != nil {
		t.Fatalf("resolveProject after remote add: %v", err)
	}
	if id2 != id1 {
		t.Errorf("adding a remote orphaned the project: %d != %d", id2, id1)
	}
	db.QueryRow(`SELECT url FROM project WHERE id=?`, id1).Scan(&url)
	if url.String != "github.com/U/Repo" {
		t.Errorf("url not backfilled: %q", url.String)
	}
	if n := count(t, db, "project"); n != 1 {
		t.Errorf("project count = %d, want 1", n)
	}
}

// V21: two clones of one remote (different paths, different url forms) resolve
// to a single shared project.
func TestCloneSharing(t *testing.T) {
	db := openTestDB(t)
	a := gitRepo(t)
	gitRun(t, a, "remote", "add", "origin", "https://github.com/u/r.git")
	b := gitRepo(t)
	gitRun(t, b, "remote", "add", "origin", "git@github.com:u/r.git")

	ida, err := findOrCreateProject(db, a)
	if err != nil {
		t.Fatalf("findOrCreate a: %v", err)
	}
	idb, err := findOrCreateProject(db, b)
	if err != nil {
		t.Fatalf("findOrCreate b: %v", err)
	}
	if ida != idb {
		t.Errorf("clones of one remote got different projects: %d, %d", ida, idb)
	}
	if n := count(t, db, "project"); n != 1 {
		t.Errorf("project count = %d, want 1 (shared)", n)
	}
}

// V23: resolving a repo that was never init'd errors.
func TestResolveUnregistered(t *testing.T) {
	dir := gitRepo(t)
	db := openTestDB(t)
	if _, err := resolveProject(db, dir); err == nil {
		t.Error("resolveProject on an unregistered project succeeded, want error (V23)")
	}
}

// R8/V27: the main worktree is resolved identically from a nested subdirectory.
func TestMainWorktreeFromSubdir(t *testing.T) {
	dir := gitRepo(t)
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := mainWorktree(sub)
	if err != nil {
		t.Fatalf("mainWorktree(subdir): %v", err)
	}
	want, err := mainWorktree(dir)
	if err != nil {
		t.Fatalf("mainWorktree(root): %v", err)
	}
	if got != want {
		t.Errorf("subdir resolved to %q, want main worktree %q", got, want)
	}
}
