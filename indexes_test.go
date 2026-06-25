package main

import (
	"database/sql"
	"strings"
	"testing"
)

// userTables lists every non-internal table (skips sqlite_* bookkeeping).
func userTables(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		out = append(out, n)
	}
	return out
}

// leftmostIndexedCols returns the set of columns that are the FIRST column of
// some index on the table — explicit (CREATE INDEX) AND implicit, since
// PRAGMA index_list also reports the PK/UNIQUE auto-indexes. An FK child column
// in this set has a covering index (V58).
func leftmostIndexedCols(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	idxRows, err := db.Query(`PRAGMA index_list("` + table + `")`)
	if err != nil {
		t.Fatalf("index_list %s: %v", table, err)
	}
	var idxNames []string
	for idxRows.Next() {
		var seq, unique, partial int
		var name, origin string
		if err := idxRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			idxRows.Close()
			t.Fatalf("scan index_list %s: %v", table, err)
		}
		idxNames = append(idxNames, name)
	}
	idxRows.Close()

	set := map[string]bool{}
	for _, name := range idxNames {
		infoRows, err := db.Query(`PRAGMA index_info("` + name + `")`)
		if err != nil {
			t.Fatalf("index_info %s: %v", name, err)
		}
		for infoRows.Next() {
			var seqno, cid int
			var col sql.NullString
			if err := infoRows.Scan(&seqno, &cid, &col); err != nil {
				infoRows.Close()
				t.Fatalf("scan index_info %s: %v", name, err)
			}
			if seqno == 0 && col.Valid {
				set[col.String] = true
			}
		}
		infoRows.Close()
	}
	return set
}

// fkChildCols returns every foreign-key child ("from") column on the table.
func fkChildCols(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`PRAGMA foreign_key_list("` + table + `")`)
	if err != nil {
		t.Fatalf("foreign_key_list %s: %v", table, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id, seq int
		var refTable, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			t.Fatalf("scan foreign_key_list %s: %v", table, err)
		}
		out = append(out, from)
	}
	return out
}

// V58: every foreign-key child column is the leftmost column of some index, so a
// new FK that ships without a covering index fails here. Walks each table's FK
// list and asserts a matching index (explicit idx_* or an implicit PK/UNIQUE).
func TestEveryFKColumnIndexedV58(t *testing.T) {
	db := openTestDB(t)
	var checked int
	for _, table := range userTables(t, db) {
		indexed := leftmostIndexedCols(t, db, table)
		for _, col := range fkChildCols(t, db, table) {
			checked++
			if !indexed[col] {
				t.Errorf("FK %s.%s has no covering index (not the leftmost column of any index)", table, col)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no FK columns discovered — the walk is vacuous")
	}
}

// indexDDLSet returns "name=sql" for every explicit index (the implicit
// PK/UNIQUE auto-indexes carry NULL sql and are excluded), ordered by name.
func indexDDLSet(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name, sql FROM sqlite_master WHERE type='index' AND sql IS NOT NULL ORDER BY name`)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name, ddl string
		if err := rows.Scan(&name, &ddl); err != nil {
			t.Fatalf("scan index: %v", err)
		}
		out = append(out, name+"="+ddl)
	}
	return out
}

// V45/V58: the v6 index step yields the same explicit indexes whether the db is
// created fresh (applySchema) or migrated from v2 — single-source DDL, no
// divergence, same as V36/V45 prove for the additive tables.
func TestFreshEqualsMigratedSchemaV6(t *testing.T) {
	fresh := openTestDB(t)
	freshIdx := indexDDLSet(t, fresh)

	v2 := openV2(t)
	if err := migrate(v2, 2); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	migratedIdx := indexDDLSet(t, v2)

	if len(freshIdx) == 0 {
		t.Fatal("fresh db has no explicit indexes — the v6 step did not apply")
	}
	if strings.Join(freshIdx, "\n") != strings.Join(migratedIdx, "\n") {
		t.Errorf("index divergence:\nfresh:    %v\nmigrated: %v", freshIdx, migratedIdx)
	}
}
