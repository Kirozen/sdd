package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// backupBinary writes a consistent binary snapshot via VACUUM INTO (R5). The
// target must not exist; SQLite refuses to overwrite, so this never clobbers.
func backupBinary(db *sql.DB, dest string) error {
	if _, err := db.Exec(`VACUUM INTO ?`, dest); err != nil {
		return fmt.Errorf("backup to %s: %w", dest, err)
	}
	return nil
}

// dumpSQL writes a portable, diffable SQL-text dump (R6, the `.dump` form):
// schema in creation order, then INSERTs for every row.
func dumpSQL(db *sql.DB, dest string) error {
	var b strings.Builder
	b.WriteString("PRAGMA foreign_keys=OFF;\nBEGIN TRANSACTION;\n")

	schema, err := db.Query(`SELECT sql FROM sqlite_master WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY rowid`)
	if err != nil {
		return err
	}
	for schema.Next() {
		var s string
		if err := schema.Scan(&s); err != nil {
			schema.Close()
			return err
		}
		b.WriteString(s)
		b.WriteString(";\n")
	}
	schema.Close()
	if err := schema.Err(); err != nil {
		return err
	}

	tables, err := tableNames(db)
	if err != nil {
		return err
	}
	for _, tbl := range tables {
		if err := dumpTable(db, &b, tbl); err != nil {
			return err
		}
	}

	b.WriteString("COMMIT;\n")
	return os.WriteFile(dest, []byte(b.String()), 0o644)
}

func tableNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func dumpTable(db *sql.DB, b *strings.Builder, table string) error {
	q := fmt.Sprintf(`SELECT * FROM "%s"`, strings.ReplaceAll(table, `"`, `""`))
	rows, err := db.Query(q)
	if err != nil {
		return err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		lits := make([]string, len(vals))
		for i, v := range vals {
			lits[i] = sqlLit(v)
		}
		fmt.Fprintf(b, `INSERT INTO "%s" VALUES(%s);`+"\n", table, strings.Join(lits, ","))
	}
	return rows.Err()
}

func sqlLit(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case []byte:
		return "'" + strings.ReplaceAll(string(x), "'", "''") + "'"
	case string:
		return "'" + strings.ReplaceAll(x, "'", "''") + "'"
	default:
		return fmt.Sprintf("'%v'", x)
	}
}

func newBackupCmd() *cobra.Command {
	var asSQL bool
	c := &cobra.Command{
		Use:   "backup <path>",
		Short: "snapshot spec.db (VACUUM INTO, or --sql for a text dump)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openGlobalDB() // a snapshot covers the whole global store
			if err != nil {
				return err
			}
			defer db.Close()
			if asSQL {
				return dumpSQL(db, args[0])
			}
			return backupBinary(db, args[0])
		},
	}
	c.Flags().BoolVar(&asSQL, "sql", false, "portable SQL-text dump instead of binary snapshot")
	return c
}
