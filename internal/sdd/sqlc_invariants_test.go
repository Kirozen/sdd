package sdd

import (
	"bytes"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"
)

// V57: project_id and ord are non-null int64 across the generated query layer
// (the sqlc *.project_id / *.ord overrides), reflecting V20/V26. No sql.NullInt64
// leaks into the query API and no nz()-style wrapping is needed at any call-site;
// a NULL would fail loudly at scan. Reverting the override would reintroduce the
// ~130 wrap/unwrap sites the deepen pass removed — this keeps it gone.
func TestGeneratedScopeIsNonNull(t *testing.T) {
	b, err := os.ReadFile("../db/query.sql.go")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("sql.NullInt64")) {
		t.Error("query.sql.go contains sql.NullInt64 — project_id/ord must stay non-null int64 (overrides, V57)")
	}
}

// rawSQLCall matches a hand-written database/sql execution call.
var rawSQLCall = regexp.MustCompile(`\.(Query|QueryRow|Exec)(Context)?\(`)

// V50/V55: every domain SQL statement goes through the generated sqlc package;
// no command or render code calls database/sql's Query/Exec/QueryRow directly.
// The only files allowed to are the documented exceptions: backup.go's `.dump`
// (dynamic SELECT */INSERT over arbitrary tables, structurally inexpressible in
// sqlc) and the connection/migrator infrastructure (db.go, store.go, schema.go),
// whose PRAGMA + DDL sqlc cannot express and which V52 keeps hand-managed. A
// forgotten call-site — the kind the inline import.go/unknown.go ordinals were —
// turns this red instead of silently leaving a literal behind.
func TestNoHandwrittenDomainSQL(t *testing.T) {
	allowed := map[string]bool{
		"backup.go": true, "db.go": true, "store.go": true, "schema.go": true,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || allowed[name] {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if loc := rawSQLCall.FindIndex(b); loc != nil {
			t.Errorf("%s calls database/sql directly (%q) — route it through the sqlc db package (V50)", name, b[loc[0]:loc[1]])
		}
	}
}

// V51: the runtime migrator's embedded DDL is byte-identical to the db/schema
// files sqlc reads at `sqlc generate` time, so the codegen schema and the runtime
// schema are the same source and cannot diverge (extends fresh==migrated, V45).
func TestSchemaIsSingleSource(t *testing.T) {
	entries, err := fs.ReadDir(schemaFS, "schema")
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no embedded schema files — the walk is vacuous")
	}
	for _, e := range entries {
		file := "schema/" + e.Name()
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		// mustDDL is the byte-exact source the migrator embeds; comparing it to
		// the on-disk file sqlc reads proves they cannot diverge. Walking
		// fs.ReadDir (not a hand-kept list) means every file is covered — a new
		// migration is checked automatically, closing the drift that left
		// 005_indexes.sql unverified (V59).
		if mustDDL(e.Name()) != string(b) {
			t.Errorf("%s: migrator DDL diverges from the file sqlc reads", file)
		}
	}
}

// V52: sqlc generates query wrappers only — it never emits schema DDL or version
// stamping, so migration stays the runtime migrator's job (schema.go), not the
// codegen's. A query that ran CREATE TABLE / PRAGMA user_version would leak into
// the generated package and trip this.
func TestSqlcEmitsNoSchemaExecution(t *testing.T) {
	for _, f := range []string{"../db/db.go", "../db/models.go", "../db/query.sql.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		s := string(b)
		if strings.Contains(s, "CREATE TABLE") || strings.Contains(s, "user_version") {
			t.Errorf("%s contains schema/migration code — sqlc must emit queries only (V52)", f)
		}
	}
}
