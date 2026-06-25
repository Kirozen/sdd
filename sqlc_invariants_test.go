package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

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
	for _, c := range []struct{ ddl, file string }{
		{schemaDDL, "db/schema/001_base.sql"},
		{unknownDDL, "db/schema/002_unknown.sql"},
		{testDDL, "db/schema/003_test.sql"},
		{gateDDL, "db/schema/004_gate.sql"},
	} {
		b, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		if c.ddl != string(b) {
			t.Errorf("%s: migrator DDL diverges from the file sqlc reads", c.file)
		}
	}
}

// V52: sqlc generates query wrappers only — it never emits schema DDL or version
// stamping, so migration stays the runtime migrator's job (schema.go), not the
// codegen's. A query that ran CREATE TABLE / PRAGMA user_version would leak into
// the generated package and trip this.
func TestSqlcEmitsNoSchemaExecution(t *testing.T) {
	for _, f := range []string{"db/db.go", "db/models.go", "db/query.sql.go"} {
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
