-- v6 (V58): index every foreign-key child column not already covered by an
-- implicit PK/UNIQUE index, so the project_id every query filters on (V20/V26)
-- and the cascade / NO-ACTION FK checks on delete stop full-scanning. Additive
-- only (CREATE INDEX IF NOT EXISTS): a fresh db gets these via applySchema, an
-- existing v5 db via the migrate step, so fresh == migrated (V45). Pure DDL, no
-- PRAGMA/data (sqlc parses this; V52 keeps migration a runtime concern).
-- Already-covered columns are deliberately omitted (no redundant index):
-- interface.project_id (UNIQUE(project_id,name)), test.invariant_id &
-- gate.feature_id (their UNIQUE), and each join table's leading PK column.

-- durable layer: project_id scope
CREATE INDEX IF NOT EXISTS idx_invariant_project ON invariant(project_id);
CREATE INDEX IF NOT EXISTS idx_bug_project       ON bug(project_id);
CREATE INDEX IF NOT EXISTS idx_research_project  ON research(project_id);
CREATE INDEX IF NOT EXISTS idx_feature_project   ON feature(project_id);

-- feature layer: feature_id (cascade target)
CREATE INDEX IF NOT EXISTS idx_goal_feature       ON goal(feature_id);
CREATE INDEX IF NOT EXISTS idx_constraint_feature ON "constraint"(feature_id);
CREATE INDEX IF NOT EXISTS idx_task_feature       ON task(feature_id);
CREATE INDEX IF NOT EXISTS idx_unknown_feature    ON unknown(feature_id);

-- join tables: the non-leading FK column (the leading PK column is already indexed)
CREATE INDEX IF NOT EXISTS idx_task_cites_inv_inv     ON task_cites_inv(inv_id);
CREATE INDEX IF NOT EXISTS idx_task_cites_iface_iface ON task_cites_iface(iface_id);
CREATE INDEX IF NOT EXISTS idx_bug_fix_inv            ON bug_fix(inv_id);
