package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// open opens (or creates) the SQLite db at path. foreign_keys is enabled on
// every pooled connection via DSN pragma (V5 needs it ON, not just once), and
// the db is put in WAL mode (persistent, file-level).
func open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL: %w", err)
	}
	return db, nil
}
