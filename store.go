package main

import (
	"database/sql"
	"os"
	"path/filepath"
)

// globalDBDir is the directory holding the single global spec.db: $XDG_CONFIG_HOME/sdd
// when that var is set to an absolute path, else ~/.config/sdd (V22, XDG spec —
// a non-absolute XDG_CONFIG_HOME is ignored).
func globalDBDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "sdd")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "sdd")
}

func globalDBPath() string { return filepath.Join(globalDBDir(), "spec.db") }

// openGlobalDB opens (creating on demand) the global spec.db, applying the
// schema the first time. It lives outside any repo and is never gitignored
// (V22). Idempotent: a db already at the current user_version is left as-is.
func openGlobalDB() (*sql.DB, error) {
	if err := os.MkdirAll(globalDBDir(), 0o755); err != nil {
		return nil, err
	}
	db, err := open(globalDBPath())
	if err != nil {
		return nil, err
	}
	var uv int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		db.Close()
		return nil, err
	}
	// uv==0 = freshly created file (full schema); uv==2 = additive v3 migration;
	// uv==3 = current. Unsupported versions error (V36).
	if err := migrate(db, uv); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
