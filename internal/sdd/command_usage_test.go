package sdd

import "testing"

// V45/V113: the v7 command_usage table is byte-identical whether the db is
// created fresh (applySchema) or migrated up from v2 — single-source DDL, no
// divergence, same as V36/V45 prove for the earlier additive tables.
func TestFreshEqualsMigratedSchemaV7(t *testing.T) {
	fresh := openTestDB(t)
	freshSQL := tableSQL(t, fresh, "command_usage")

	v2 := openV2(t)
	if err := migrate(v2, 2); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	migratedSQL := tableSQL(t, v2, "command_usage")

	if freshSQL == "" {
		t.Fatal("fresh db has no command_usage table — the v7 step did not apply")
	}
	if freshSQL != migratedSQL {
		t.Errorf("schema divergence:\nfresh:    %q\nmigrated: %q", freshSQL, migratedSQL)
	}
	if uv := userVersionOf(t, v2); uv != userVersion {
		t.Errorf("migrated db stamped v%d, want v%d", uv, userVersion)
	}
}
