package main

import (
	"os"
	"path/filepath"
	"testing"
)

// V56: every db/*.sql file must stay ASCII-only. sqlc miscomputes a query's byte
// span when the .sql holds a multibyte UTF-8 rune anywhere before it (even in a
// comment), silently scrambling the generated SQL string (e.g. a split ORDER BY)
// that parses at codegen yet fails only at runtime. This guards the trap hit
// while building the read layer (B6): a stray em-dash in query.sql corrupted
// bugsByProject/invariantsByProject/unknownsByProject.
func TestSQLFilesAreASCII(t *testing.T) {
	paths, err := filepath.Glob("db/schema/*.sql")
	if err != nil {
		t.Fatalf("glob schema: %v", err)
	}
	paths = append(paths, "db/query.sql")
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		for i, c := range b {
			if c > 127 {
				t.Fatalf("%s: non-ASCII byte 0x%02x at offset %d — sqlc corrupts query spans on multibyte UTF-8; keep .sql ASCII-only", p, c, i)
			}
		}
	}
}
