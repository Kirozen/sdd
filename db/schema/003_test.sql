-- v4 (V42): a durable link from an invariant to a test that proves it.
-- UNIQUE(invariant_id, name) makes re-adding the same pair a no-op rather than a
-- silent duplicate; one test name may guard several invariants (distinct rows).
-- Applied after 001 in filename order (FK -> invariant).
CREATE TABLE test (
	id           INTEGER PRIMARY KEY,
	invariant_id INTEGER NOT NULL REFERENCES invariant(id) ON DELETE CASCADE,
	name         TEXT NOT NULL,
	UNIQUE (invariant_id, name)
);
