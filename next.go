package main

import (
	"database/sql"
	"fmt"
)

// nextResult is the chosen actionable task plus the context that explains it:
// the owning feature, all its goals (V33), the task's caveman line, and each
// cite resolved to its full V/I line. Nil when the project has no actionable
// (non-`x`) task — the caller decides the empty-case hint (V31).
type nextResult struct {
	featOrd   int
	featName  string
	goals     []string
	taskLine  string
	citeLines []string
}

// nextActionable selects the next task to work on (V30): the first feature by
// ord that owns a non-`x` task, and within it `~` (wip) before the smallest-ord
// `.` (todo). Read-pure (V16): SELECT only, no write, no re-export. Scoped to
// the project (V20): every query filters by project_id. Returns nil when no
// actionable task exists.
func nextActionable(db *sql.DB, projectID int64) (*nextResult, error) {
	var (
		featOrd, taskOrd       int
		featName, status, text string
		featPK, taskPK         int64
	)
	// Lowest-ord feature with a non-`x` task; within it `~` before `.`, then ord.
	err := db.QueryRow(`SELECT f.ord, f.name, f.id, t.ord, t.status, t.text, t.id
		FROM task t JOIN feature f ON f.id = t.feature_id
		WHERE f.project_id = ? AND t.status != 'x'
		ORDER BY f.ord ASC,
			CASE t.status WHEN '~' THEN 0 ELSE 1 END ASC,
			t.ord ASC
		LIMIT 1`, projectID).Scan(&featOrd, &featName, &featPK, &taskOrd, &status, &text, &taskPK)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	goals, err := featureGoals(db, featPK)
	if err != nil {
		return nil, err
	}

	cites, err := taskCites(db, taskPK)
	if err != nil {
		return nil, err
	}
	citeLines, err := resolveCites(db, projectID, cites)
	if err != nil {
		return nil, err
	}

	return &nextResult{
		featOrd:   featOrd,
		featName:  featName,
		goals:     goals,
		taskLine:  fmtTaskLine(taskOrd, status, text, cites),
		citeLines: citeLines,
	}, nil
}

// featureGoals returns every goal of a feature in display order (V33: 0..N,
// ORDER BY id like renderSpec). A feature with no goals yields an empty slice,
// never an error.
func featureGoals(db *sql.DB, featurePK int64) ([]string, error) {
	rows, err := db.Query(`SELECT text FROM goal WHERE feature_id=? ORDER BY id`, featurePK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var goals []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

// resolveCites maps a task's raw cite string (e.g. "V30,I.next" or the "-"
// sentinel for none) to each cite's full caveman line via showRef, so next
// renders through the same single source as the spec (V18).
func resolveCites(db *sql.DB, projectID int64, cites string) ([]string, error) {
	refs := splitRefs(cites)
	if len(refs) == 0 {
		return nil, nil
	}
	lines := make([]string, 0, len(refs))
	for _, r := range refs {
		line, err := showRef(db, projectID, r)
		if err != nil {
			return nil, fmt.Errorf("resolving cite %q: %w", r, err)
		}
		lines = append(lines, line)
	}
	return lines, nil
}
