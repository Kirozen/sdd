package main

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

// listKind returns every row of a kind in the current project as caveman lines,
// ordered by the per-project ordinal, formatted through the same fmt*Line
// helpers as renderSpec (V18). Read-pure (V16); scoped by project (V20). An
// unknown kind errors (V17); a valid-but-empty kind returns no lines.
func listKind(db *sql.DB, projectID int64, kind string) ([]string, error) {
	switch kind {
	case "invariant":
		return listRows(db, `SELECT ord, text FROM invariant WHERE project_id=? ORDER BY ord`, projectID, func(rows *sql.Rows) (string, error) {
			var ord int
			var text string
			if err := rows.Scan(&ord, &text); err != nil {
				return "", err
			}
			return fmtInvariantLine(ord, text), nil
		})
	case "interface":
		return listRows(db, `SELECT kind, name, sig, status FROM interface WHERE project_id=? ORDER BY id`, projectID, func(rows *sql.Rows) (string, error) {
			var k, name, sig, status string
			if err := rows.Scan(&k, &name, &sig, &status); err != nil {
				return "", err
			}
			return fmtInterfaceLine(k, name, sig, status), nil
		})
	case "research":
		return listRows(db, `SELECT ord, topic, finding, src FROM research WHERE project_id=? ORDER BY ord`, projectID, func(rows *sql.Rows) (string, error) {
			var ord int
			var topic, finding, src string
			if err := rows.Scan(&ord, &topic, &finding, &src); err != nil {
				return "", err
			}
			return fmtResearchLine(ord, topic, finding, src), nil
		})
	case "feature":
		return listRows(db, `SELECT ord, name FROM feature WHERE project_id=? ORDER BY ord`, projectID, func(rows *sql.Rows) (string, error) {
			var ord int
			var name string
			if err := rows.Scan(&ord, &name); err != nil {
				return "", err
			}
			return fmt.Sprintf("FEATURE %d: %s", ord, name), nil
		})
	case "task":
		return listTasks(db, projectID)
	case "bug":
		return listBugs(db, projectID)
	case "unknown":
		// Feature-scoped, all statuses (open + resolved), per-project ordinal U<n>.
		// Not part of listAllKinds, so `list` (no kind) omits it (V28); only an
		// explicit `list unknown` surfaces them.
		return listRows(db, `SELECT u.ord, u.status, u.text FROM unknown u JOIN feature f ON f.id=u.feature_id WHERE f.project_id=? ORDER BY u.ord`, projectID, func(rows *sql.Rows) (string, error) {
			var ord int
			var status, text string
			if err := rows.Scan(&ord, &status, &text); err != nil {
				return "", err
			}
			return fmtUnknownLine(ord, status, text), nil
		})
	default:
		return nil, fmt.Errorf("unknown kind %q (want invariant|interface|task|bug|research|feature|unknown)", kind)
	}
}

// listAllKinds is the canonical order `list` (no kind) walks — the same order
// renderSpec emits sections (V28): interfaces, research, invariants, bugs, then
// features and their tasks.
var listAllKinds = []string{"interface", "research", "invariant", "bug", "feature", "task"}

// listAll concatenates listKind over listAllKinds, so every emitted line is
// byte-identical to its single-kind `list <kind>` line (V28, V18).
func listAll(db *sql.DB, projectID int64) ([]string, error) {
	var out []string
	for _, kind := range listAllKinds {
		lines, err := listKind(db, projectID, kind)
		if err != nil {
			return nil, err
		}
		out = append(out, lines...)
	}
	return out, nil
}

// statusGlyph maps a task status to a one-rune marker for the pretty view.
func statusGlyph(s string) string {
	switch s {
	case "x":
		return "✓"
	case "~":
		return "~"
	default:
		return "·"
	}
}

// maxLen returns the widest string in refs (for ref-column alignment).
func maxLen(refs []string) int {
	w := 0
	for _, r := range refs {
		if n := len([]rune(r)); n > w {
			w = n
		}
	}
	return w
}

// listPretty renders the whole project as a grouped, human-readable view for
// `list --pretty` (V29): a header per non-empty kind, refs aligned in a column,
// tasks nested under their feature, and no pipe-delimited machine rows. This
// view deliberately diverges from the canonical caveman lines (V18); it is
// opt-in and never the default (V28).
func listPretty(db *sql.DB, projectID int64) ([]string, error) {
	var out []string
	section := func(title string, rows []string) {
		if len(rows) == 0 {
			return
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, title)
		out = append(out, rows...)
	}
	ifaces, err := prettyInterfaces(db, projectID)
	if err != nil {
		return nil, err
	}
	section("INTERFACES", ifaces)

	research, err := prettyResearch(db, projectID)
	if err != nil {
		return nil, err
	}
	section("RESEARCH", research)

	invs, err := prettyInvariants(db, projectID)
	if err != nil {
		return nil, err
	}
	section("INVARIANTS", invs)

	bugs, err := prettyBugs(db, projectID)
	if err != nil {
		return nil, err
	}
	section("BUGS", bugs)

	feats, err := prettyFeatures(db, projectID)
	if err != nil {
		return nil, err
	}
	for _, f := range feats {
		section(f.header, f.tasks)
	}
	return out, nil
}

func prettyInterfaces(db *sql.DB, projectID int64) ([]string, error) {
	rows, err := db.Query(`SELECT kind, name, sig, status FROM interface WHERE project_id=? ORDER BY id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs, bodies []string
	for rows.Next() {
		var kind, name, sig, status string
		if err := rows.Scan(&kind, &name, &sig, &status); err != nil {
			return nil, err
		}
		mark := ""
		if status == "deprecated" {
			mark = " [deprecated]"
		}
		refs = append(refs, "I."+name)
		bodies = append(bodies, fmt.Sprintf("(%s) %s%s", kind, sig, mark))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return alignRows(refs, bodies), nil
}

func prettyResearch(db *sql.DB, projectID int64) ([]string, error) {
	rows, err := db.Query(`SELECT ord, topic, finding, src FROM research WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs, bodies []string
	for rows.Next() {
		var ord int
		var topic, finding, src string
		if err := rows.Scan(&ord, &topic, &finding, &src); err != nil {
			return nil, err
		}
		refs = append(refs, fmt.Sprintf("R%d", ord))
		bodies = append(bodies, fmt.Sprintf("%s — %s  (%s)", topic, finding, src))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return alignRows(refs, bodies), nil
}

func prettyInvariants(db *sql.DB, projectID int64) ([]string, error) {
	rows, err := db.Query(`SELECT ord, text FROM invariant WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs, bodies []string
	for rows.Next() {
		var ord int
		var text string
		if err := rows.Scan(&ord, &text); err != nil {
			return nil, err
		}
		refs = append(refs, fmt.Sprintf("V%d", ord))
		bodies = append(bodies, text)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return alignRows(refs, bodies), nil
}

func prettyBugs(db *sql.DB, projectID int64) ([]string, error) {
	rows, err := db.Query(`SELECT id, ord, date, cause FROM bug WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type bg struct {
		pk          int64
		ord         int
		date, cause string
	}
	var bugs []bg
	for rows.Next() {
		var b bg
		if err := rows.Scan(&b.pk, &b.ord, &b.date, &b.cause); err != nil {
			return nil, err
		}
		bugs = append(bugs, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var refs, bodies []string
	for _, b := range bugs {
		fix, err := bugFix(db, b.pk)
		if err != nil {
			return nil, err
		}
		body := fmt.Sprintf("%s  %s", b.date, b.cause)
		if fix != "-" {
			body += "  → fixed " + fix
		}
		refs = append(refs, fmt.Sprintf("B%d", b.ord))
		bodies = append(bodies, body)
	}
	return alignRows(refs, bodies), nil
}

// prettyFeature is one feature header with its indented, aligned task rows.
type prettyFeature struct {
	header string
	tasks  []string
}

func prettyFeatures(db *sql.DB, projectID int64) ([]prettyFeature, error) {
	rows, err := db.Query(`SELECT id, ord, name FROM feature WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return nil, err
	}
	type feat struct {
		pk   int64
		ord  int
		name string
	}
	var feats []feat
	for rows.Next() {
		var f feat
		if err := rows.Scan(&f.pk, &f.ord, &f.name); err != nil {
			rows.Close()
			return nil, err
		}
		feats = append(feats, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []prettyFeature
	for _, f := range feats {
		tasks, err := prettyTasks(db, f.pk)
		if err != nil {
			return nil, err
		}
		out = append(out, prettyFeature{
			header: fmt.Sprintf("FEATURE %d — %s", f.ord, f.name),
			tasks:  tasks,
		})
	}
	return out, nil
}

func prettyTasks(db *sql.DB, featurePK int64) ([]string, error) {
	rows, err := db.Query(`SELECT id, ord, status, text FROM task WHERE feature_id=? ORDER BY ord`, featurePK)
	if err != nil {
		return nil, err
	}
	type tk struct {
		pk           int64
		ord          int
		status, text string
	}
	var tasks []tk
	for rows.Next() {
		var t tk
		if err := rows.Scan(&t.pk, &t.ord, &t.status, &t.text); err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var refs, bodies []string
	for _, t := range tasks {
		cites, err := taskCites(db, t.pk)
		if err != nil {
			return nil, err
		}
		body := fmt.Sprintf("%s  %s", statusGlyph(t.status), t.text)
		if cites != "-" {
			body += "  → " + cites
		}
		refs = append(refs, fmt.Sprintf("T%d", t.ord))
		bodies = append(bodies, body)
	}
	return alignRows(refs, bodies), nil
}

// alignRows pads each ref to the group's widest, under a 2-space indent.
func alignRows(refs, bodies []string) []string {
	w := maxLen(refs)
	lines := make([]string, len(refs))
	for i := range refs {
		lines[i] = fmt.Sprintf("  %-*s  %s", w, refs[i], bodies[i])
	}
	return lines
}

// listRows runs query (scoped to projectID) and maps each row to a line via fn.
func listRows(db *sql.DB, query string, projectID int64, fn func(*sql.Rows) (string, error)) ([]string, error) {
	rows, err := db.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		line, err := fn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, rows.Err()
}

// listTasks and listBugs need a second query per row (cites / fix), so they
// drain the cursor before re-joining.
func listTasks(db *sql.DB, projectID int64) ([]string, error) {
	return listTasksFiltered(db, projectID, "", 0)
}

// listTasksFiltered lists a project's tasks, optionally narrowed by status
// (V38: "" = any, else exactly one of .|~|x) and feature ordinal (0 = any, else
// the feature must exist or it errors). Lines render through fmtTaskLine (V18).
func listTasksFiltered(db *sql.DB, projectID int64, status string, featureOrd int64) ([]string, error) {
	q := `SELECT t.id, t.ord, t.status, t.text FROM task t JOIN feature f ON f.id=t.feature_id WHERE f.project_id=?`
	qargs := []any{projectID}
	if featureOrd > 0 {
		pk, err := featurePK(db, projectID, featureOrd) // missing feature → error (V38)
		if err != nil {
			return nil, err
		}
		q += ` AND t.feature_id=?`
		qargs = append(qargs, pk)
	}
	if status != "" {
		q += ` AND t.status=?`
		qargs = append(qargs, status)
	}
	q += ` ORDER BY t.ord`

	type tk struct {
		pk           int64
		ord          int
		status, text string
	}
	var tasks []tk
	rows, err := db.Query(q, qargs...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t tk
		if err := rows.Scan(&t.pk, &t.ord, &t.status, &t.text); err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []string
	for _, t := range tasks {
		cites, err := taskCites(db, t.pk)
		if err != nil {
			return nil, err
		}
		out = append(out, fmtTaskLine(t.ord, t.status, t.text, cites))
	}
	return out, nil
}

func listBugs(db *sql.DB, projectID int64) ([]string, error) {
	type bg struct {
		pk          int64
		ord         int
		date, cause string
	}
	var bugs []bg
	rows, err := db.Query(`SELECT id, ord, date, cause FROM bug WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var b bg
		if err := rows.Scan(&b.pk, &b.ord, &b.date, &b.cause); err != nil {
			rows.Close()
			return nil, err
		}
		bugs = append(bugs, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []string
	for _, b := range bugs {
		fix, err := bugFix(db, b.pk)
		if err != nil {
			return nil, err
		}
		out = append(out, fmtBugLine(b.ord, b.date, b.cause, fix))
	}
	return out, nil
}

func newListCmd() *cobra.Command {
	var pretty bool
	var status string
	var feature int64
	c := &cobra.Command{
		Use:   "list [kind]",
		Short: "print all rows of a kind (invariant|interface|task|bug|research|feature|unknown), or every kind if omitted; read-only",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if pretty && len(args) > 0 {
				return fmt.Errorf("--pretty applies to the full list only (drop the kind argument)")
			}
			// V41: --status/--feature are task-only filters.
			filtering := cmd.Flags().Changed("status") || cmd.Flags().Changed("feature")
			if filtering && (len(args) == 0 || args[0] != "task") {
				return fmt.Errorf("--status/--feature apply to `list task` only")
			}
			if cmd.Flags().Changed("status") && status != "." && status != "~" && status != "x" {
				return fmt.Errorf("invalid --status %q (want . ~ x)", status)
			}
			db, pid, _, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			var lines []string
			switch {
			case pretty:
				lines, err = listPretty(db, pid)
			case filtering:
				lines, err = listTasksFiltered(db, pid, status, feature)
			case len(args) == 0:
				lines, err = listAll(db, pid)
			default:
				lines, err = listKind(db, pid, args[0])
			}
			if err != nil {
				return err
			}
			for _, l := range lines {
				fmt.Println(l)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&pretty, "pretty", false, "grouped human-readable view (no kind argument)")
	c.Flags().StringVar(&status, "status", "", "filter `list task` by status (. ~ x)")
	c.Flags().Int64Var(&feature, "feature", 0, "filter `list task` by feature number")
	return c
}
