-- v5 (V46): a durable review verdict per feature. feature_id UNIQUE gives the
-- one-per-feature UPSERT a conflict target; the row cascades with its feature
-- (V4). Applied after 001 in filename order (FK -> feature).
CREATE TABLE gate (
	id          INTEGER PRIMARY KEY,
	feature_id  INTEGER NOT NULL UNIQUE REFERENCES feature(id) ON DELETE CASCADE,
	verdict     TEXT NOT NULL CHECK (verdict IN ('go','no-go')),
	note        TEXT,
	recorded_at TEXT NOT NULL
);
