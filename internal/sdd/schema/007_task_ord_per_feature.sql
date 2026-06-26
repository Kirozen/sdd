-- F25 (V117): task ordinals become per-feature - each feature restarts at T1.
-- One-time renumber of pre-existing project-wide ords into a dense 1..N sequence
-- per feature, original order preserved. The renumber reads an IMMUTABLE snapshot
-- (_renum) rather than the table it mutates, so the new ord of each row is its
-- rank in the snapshot regardless of update order - no transient collision (V119).
-- (sqlc's sqlite parser rejects UPDATE...FROM and window funcs in schema files, so
-- the rank is a COUNT over the snapshot.) On a fresh db task is empty -> no-op, so
-- fresh == migrated (V45).
CREATE TEMP TABLE _renum AS SELECT id, feature_id, ord FROM task;

UPDATE task SET ord = (
	SELECT COUNT(*) FROM _renum s
	WHERE s.feature_id = task.feature_id
	  AND s.ord <= (SELECT ord FROM _renum WHERE _renum.id = task.id)
);

DROP TABLE _renum;

-- Created AFTER the renumber (V119): two tasks of one feature cannot share an ord.
CREATE UNIQUE INDEX idx_task_feature_ord ON task(feature_id, ord);
