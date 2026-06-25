-- F8 sqlc query layer (V50): every domain SQL statement that previously lived as
-- a hand-written string literal, as a named sqlc query. Generated into package db
-- by `go tool sqlc generate`. Parameters use `?` (sqlc names them from the column)
-- except where a value is reused or nullable, which use sqlc.arg/sqlc.narg.
-- The only domain SQL that stays hand-written is backup.go's `.dump` (sqlite_master
-- introspection + dynamic SELECT */INSERT over arbitrary tables) - structurally
-- inexpressible in sqlc (table/columns unknown at codegen); the documented V50
-- exception.

-- ============================================================ project (resolve.go)

-- name: ProjectByURL :one
SELECT id FROM project WHERE url = ?;

-- name: ProjectByPath :one
SELECT id FROM project WHERE path = ?;

-- name: BackfillProjectURL :exec
UPDATE project SET url = ? WHERE id = ? AND url IS NULL;

-- name: CreateProject :execlastid
-- url is the nullable column, so sqlc types this param sql.NullString - pass an
-- unset NullString for a project first seen without an origin.
INSERT INTO project(url, path) VALUES(?, ?);

-- ============================================================ ordinals (V26)
-- COALESCE(MAX(ord),0)+1, cast so sqlc yields a clean int64.

-- name: NextInvariantOrd :one
SELECT CAST(COALESCE(MAX(ord), 0) + 1 AS INTEGER) AS next_ord FROM invariant WHERE project_id = ?;

-- name: NextBugOrd :one
SELECT CAST(COALESCE(MAX(ord), 0) + 1 AS INTEGER) AS next_ord FROM bug WHERE project_id = ?;

-- name: NextResearchOrd :one
SELECT CAST(COALESCE(MAX(ord), 0) + 1 AS INTEGER) AS next_ord FROM research WHERE project_id = ?;

-- name: NextFeatureOrd :one
SELECT CAST(COALESCE(MAX(ord), 0) + 1 AS INTEGER) AS next_ord FROM feature WHERE project_id = ?;

-- name: NextTaskOrd :one
SELECT CAST(COALESCE(MAX(t.ord), 0) + 1 AS INTEGER) AS next_ord
FROM task t JOIN feature f ON f.id = t.feature_id
WHERE f.project_id = ?;

-- name: NextUnknownOrd :one
SELECT CAST(COALESCE(MAX(u.ord), 0) + 1 AS INTEGER) AS next_ord
FROM unknown u JOIN feature f ON f.id = u.feature_id
WHERE f.project_id = ?;

-- ============================================================ inserts (mutations.go / edit.go / import.go)

-- name: InsertFeature :execlastid
INSERT INTO feature(project_id, ord, name) VALUES(?, ?, ?);

-- name: FeaturePK :one
SELECT id FROM feature WHERE project_id = ? AND ord = ?;

-- name: FeatureOrdByID :one
SELECT ord FROM feature WHERE id = ?;

-- name: TaskOrdByID :one
SELECT ord FROM task WHERE id = ?;

-- name: TaskPKByOrd :one
SELECT t.id FROM task t JOIN feature f ON f.id = t.feature_id WHERE f.project_id = ? AND t.ord = ?;

-- name: InsertGoal :exec
INSERT INTO goal(feature_id, text) VALUES(?, ?);

-- name: InsertConstraint :exec
INSERT INTO "constraint"(feature_id, text) VALUES(?, ?);

-- name: InsertInvariant :exec
INSERT INTO invariant(project_id, ord, text) VALUES(?, ?, ?);

-- name: InsertInterface :execlastid
INSERT INTO interface(project_id, kind, name, sig) VALUES(?, ?, ?, ?);

-- name: InsertResearch :exec
INSERT INTO research(project_id, ord, topic, finding, src) VALUES(?, ?, ?, ?, ?);

-- name: InsertBug :execlastid
INSERT INTO bug(project_id, ord, date, cause) VALUES(?, ?, ?, ?);

-- name: InsertBugFix :exec
INSERT INTO bug_fix(bug_id, inv_id) VALUES(?, ?);

-- name: InsertTask :execlastid
INSERT INTO task(feature_id, ord, text) VALUES(?, ?, ?);

-- name: InsertTaskFull :execlastid
INSERT INTO task(feature_id, ord, text, status) VALUES(?, ?, ?, ?);

-- name: InsertTaskCiteInv :exec
INSERT INTO task_cites_inv(task_id, inv_id) VALUES(?, ?);

-- name: InsertTaskCiteIface :exec
INSERT INTO task_cites_iface(task_id, iface_id) VALUES(?, ?);

-- name: InvariantIDByOrd :one
SELECT id FROM invariant WHERE project_id = ? AND ord = ?;

-- name: InterfaceIDByName :one
SELECT id FROM interface WHERE project_id = ? AND name = ?;

-- name: InsertUnknown :exec
INSERT INTO unknown(feature_id, ord, text) VALUES(?, ?, ?);

-- name: InsertTest :exec
INSERT INTO test(invariant_id, name) VALUES(?, ?)
ON CONFLICT(invariant_id, name) DO NOTHING;

-- name: UpsertGate :exec
INSERT INTO gate(feature_id, verdict, note, recorded_at) VALUES(?, ?, ?, ?)
ON CONFLICT(feature_id) DO UPDATE SET
	verdict = excluded.verdict, note = excluded.note, recorded_at = excluded.recorded_at;

-- ============================================================ updates / deletes (return rows affected for the n==0 not-found check)

-- name: EditInvariant :execrows
UPDATE invariant SET text = ? WHERE project_id = ? AND ord = ?;

-- name: EditResearch :execrows
UPDATE research SET finding = ? WHERE project_id = ? AND ord = ?;

-- name: EditBug :execrows
UPDATE bug SET cause = ? WHERE project_id = ? AND ord = ?;

-- name: EditTask :execrows
UPDATE task SET text = ? WHERE task.ord = ? AND task.feature_id IN (SELECT feature.id FROM feature WHERE feature.project_id = ?);

-- name: EditInterfaceSig :execrows
UPDATE interface SET sig = ? WHERE project_id = ? AND name = ?;

-- name: EditGoal :execrows
UPDATE goal SET text = ? WHERE id = ?;

-- name: EditConstraint :execrows
UPDATE "constraint" SET text = ? WHERE id = ?;

-- name: DeprecateInterface :execrows
UPDATE interface SET status = 'deprecated' WHERE project_id = ? AND name = ?;

-- name: SetTaskStatus :execrows
UPDATE task SET status = ? WHERE task.ord = ? AND task.feature_id IN (SELECT feature.id FROM feature WHERE feature.project_id = ?);

-- name: ResolveUnknown :execrows
UPDATE unknown SET status = 'resolved' WHERE unknown.ord = ? AND unknown.feature_id IN (SELECT feature.id FROM feature WHERE feature.project_id = ?);

-- name: WipeFeature :execrows
DELETE FROM feature WHERE project_id = ? AND ord = ?;

-- ============================================================ seed/import wipe (import.go)

-- name: ProjectRowCount :one
-- Five positional params, all the same project id (sqlc names them ProjectID,
-- ProjectID_2, etc). Caller passes pid to each. Sum > 0 means the project has rows.
SELECT CAST(
	(SELECT count(*) FROM invariant WHERE invariant.project_id = ?)
	+ (SELECT count(*) FROM interface WHERE interface.project_id = ?)
	+ (SELECT count(*) FROM bug       WHERE bug.project_id = ?)
	+ (SELECT count(*) FROM research  WHERE research.project_id = ?)
	+ (SELECT count(*) FROM feature   WHERE feature.project_id = ?)
AS INTEGER) AS n;

-- name: DeleteProjectFeatures :exec
DELETE FROM feature WHERE project_id = ?;

-- name: DeleteProjectBugs :exec
DELETE FROM bug WHERE project_id = ?;

-- name: DeleteProjectInvariants :exec
DELETE FROM invariant WHERE project_id = ?;

-- name: DeleteProjectInterfaces :exec
DELETE FROM interface WHERE project_id = ?;

-- name: DeleteProjectResearch :exec
DELETE FROM research WHERE project_id = ?;

-- ============================================================ reads: canonical row lists (export.go + list.go share these, V18)

-- name: InterfacesByProject :many
SELECT kind, name, sig, status FROM interface WHERE project_id = ? ORDER BY id;

-- name: ResearchByProject :many
SELECT ord, topic, finding, src FROM research WHERE project_id = ? ORDER BY ord;

-- name: InvariantsByProject :many
SELECT ord, text FROM invariant WHERE project_id = ? ORDER BY ord;

-- name: BugsByProject :many
SELECT id, ord, date, cause FROM bug WHERE project_id = ? ORDER BY ord;

-- name: FeaturesByProject :many
SELECT id, ord, name FROM feature WHERE project_id = ? ORDER BY ord;

-- name: OpenFeaturesByProject :many
-- Unfinished features = NOT in the built stage (V32): has a non-x task OR zero
-- tasks; a grilled/specced-but-untasked feature stays visible (V75).
SELECT id, ord, name FROM feature f
WHERE f.project_id = ?
  AND ( EXISTS (SELECT 1 FROM task t WHERE t.feature_id = f.id AND t.status != 'x')
        OR NOT EXISTS (SELECT 1 FROM task t WHERE t.feature_id = f.id) )
ORDER BY f.ord;

-- name: FeatureByOrd :many
-- Single feature by (project_id, ord); :many so an unknown ord yields zero rows
-- (cat maps empty -> exit!=0, V75) rather than sql.ErrNoRows.
SELECT id, ord, name FROM feature WHERE project_id = ? AND ord = ? ORDER BY ord;

-- name: TasksByFeature :many
SELECT id, ord, status, text FROM task WHERE feature_id = ? ORDER BY ord;

-- name: GoalsByFeature :many
SELECT text FROM goal WHERE feature_id = ? ORDER BY id;

-- name: ConstraintsByFeature :many
SELECT text FROM "constraint" WHERE feature_id = ? ORDER BY id;

-- name: UnknownsByProject :many
SELECT u.ord, u.status, u.text
FROM unknown u JOIN feature f ON f.id = u.feature_id
WHERE f.project_id = ? ORDER BY u.ord;

-- listTasksFiltered (V38/V53) picks one of these four by which filters are set;
-- sqlc's SQLite engine rejects narg-style optional params, so the combinations
-- are explicit. Feature existence is still resolved in Go (featurePK) before the
-- by-feature variants run, so a missing feature errors rather than returns empty.

-- name: TasksInProject :many
SELECT t.id, t.ord, t.status, t.text
FROM task t JOIN feature f ON f.id = t.feature_id
WHERE f.project_id = ? ORDER BY t.ord;

-- name: PendingTasks :many
SELECT f.ord AS feature_ord, f.name AS feature_name, t.id, t.ord, t.status, t.text
FROM task t JOIN feature f ON f.id = t.feature_id
WHERE f.project_id = ? AND t.status != 'x'
ORDER BY f.ord, t.ord;

-- name: TasksInProjectByStatus :many
SELECT t.id, t.ord, t.status, t.text
FROM task t JOIN feature f ON f.id = t.feature_id
WHERE f.project_id = ? AND t.status = ? ORDER BY t.ord;

-- name: TasksInProjectByFeature :many
SELECT t.id, t.ord, t.status, t.text
FROM task t JOIN feature f ON f.id = t.feature_id
WHERE f.project_id = ? AND t.feature_id = ? ORDER BY t.ord;

-- name: TasksInProjectByFeatureStatus :many
SELECT t.id, t.ord, t.status, t.text
FROM task t JOIN feature f ON f.id = t.feature_id
WHERE f.project_id = ? AND t.feature_id = ? AND t.status = ? ORDER BY t.ord;

-- ============================================================ reads: cite resolution (export.go / next.go)

-- name: BugFixInvOrds :many
SELECT i.ord FROM bug_fix j JOIN invariant i ON i.id = j.inv_id WHERE j.bug_id = ? ORDER BY i.ord;

-- name: TaskCiteInvOrds :many
SELECT i.ord FROM task_cites_inv j JOIN invariant i ON i.id = j.inv_id WHERE j.task_id = ? ORDER BY i.ord;

-- name: TaskCiteIfaceNames :many
SELECT i.name FROM task_cites_iface j JOIN interface i ON i.id = j.iface_id WHERE j.task_id = ? ORDER BY i.id;

-- ============================================================ reads: show (show.go)

-- name: ShowInterface :one
SELECT kind, sig, status FROM interface WHERE project_id = ? AND name = ?;

-- name: ShowInvariant :one
SELECT text FROM invariant WHERE project_id = ? AND ord = ?;

-- name: ShowTask :one
SELECT t.id, t.status, t.text
FROM task t JOIN feature f ON f.id = t.feature_id
WHERE f.project_id = ? AND t.ord = ?;

-- name: ShowBug :one
SELECT id, date, cause FROM bug WHERE project_id = ? AND ord = ?;

-- name: ShowResearch :one
SELECT topic, finding, src FROM research WHERE project_id = ? AND ord = ?;

-- ============================================================ reads: refs (refs.go)

-- name: CitersOfIface :many
SELECT t.ord FROM task_cites_iface j JOIN task t ON t.id = j.task_id WHERE j.iface_id = ? ORDER BY t.ord;

-- name: TaskCitersOfInv :many
SELECT t.ord FROM task_cites_inv j JOIN task t ON t.id = j.task_id WHERE j.inv_id = ? ORDER BY t.ord;

-- name: BugCitersOfInv :many
SELECT b.ord FROM bug_fix j JOIN bug b ON b.id = j.bug_id WHERE j.inv_id = ? ORDER BY b.ord;

-- ============================================================ reads: status / next (status.go, next.go, gate.go)

-- name: TaskStatusCounts :many
SELECT status, CAST(count(*) AS INTEGER) AS n FROM task WHERE feature_id = ? GROUP BY status;

-- name: FeatureStageCounts :one
SELECT
	CAST(count(*) AS INTEGER) AS total,
	CAST(COALESCE(SUM(CASE WHEN status = 'x' THEN 1 ELSE 0 END), 0) AS INTEGER) AS done,
	CAST(COALESCE(SUM(CASE WHEN status = '.' THEN 1 ELSE 0 END), 0) AS INTEGER) AS todo
FROM task WHERE feature_id = ?;

-- name: FeatureGoalConstraintCount :one
-- Two positional params, both the same feature id (FeatureID, FeatureID_2).
SELECT CAST(
	(SELECT count(*) FROM goal WHERE goal.feature_id = ?)
	+ (SELECT count(*) FROM "constraint" WHERE "constraint".feature_id = ?)
AS INTEGER) AS n;

-- name: DeprecatedCiteWarnings :many
SELECT t.ord, i.name
FROM task_cites_iface j
JOIN interface i ON i.id = j.iface_id
JOIN task t ON t.id = j.task_id
JOIN feature f ON f.id = t.feature_id
WHERE f.project_id = ? AND i.status = 'deprecated'
ORDER BY t.ord, i.name;

-- name: OpenUnknownFeatures :many
SELECT f.ord, f.name, CAST(count(*) AS INTEGER) AS n
FROM unknown u
JOIN feature f ON f.id = u.feature_id
WHERE f.project_id = ? AND u.status = 'open'
GROUP BY f.id
ORDER BY f.ord;

-- name: NextActionableTask :one
SELECT f.ord AS feat_ord, f.name AS feat_name, f.id AS feat_id,
       t.ord AS task_ord, t.status, t.text, t.id AS task_id
FROM task t JOIN feature f ON f.id = t.feature_id
WHERE f.project_id = ? AND t.status != 'x'
ORDER BY f.ord ASC,
	CASE t.status WHEN '~' THEN 0 ELSE 1 END ASC,
	t.ord ASC
LIMIT 1;

-- name: GateVerdict :one
SELECT verdict FROM gate WHERE feature_id = ?;

-- ============================================================ reads: cover (cover.go)

-- name: InvariantCoverage :many
-- Each project invariant with its proving test names joined (empty when none).
-- CAST keeps GROUP_CONCAT a plain string for sqlc.
SELECT i.ord, i.text, CAST(COALESCE(GROUP_CONCAT(t.name, ', '), '') AS TEXT) AS tests
FROM invariant i LEFT JOIN test t ON t.invariant_id = i.id
WHERE i.project_id = ?
GROUP BY i.id ORDER BY i.ord;
