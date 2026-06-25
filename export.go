package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	specName      = "SPEC.md"
	generatedHead = "<!-- GENERATED — DO NOT EDIT -->"
)

// esc escapes a literal pipe so it survives a pipe-table cell.
func esc(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

// --- per-row line formatters: the single source of caveman line rendering.
// Both renderSpec (whole sections) and `sdd show` (one row) go through these,
// so a show line is byte-identical to its SPEC.md line (V18). The leading int
// is the per-project ordinal (V26), not the global PK.

func fmtInterfaceLine(kind, name, sig, status string) string {
	mark := ""
	if status == "deprecated" {
		mark = " [deprecated]"
	}
	return fmt.Sprintf("- %s: %s → %s (I.%s)%s", kind, name, sig, name, mark)
}

func fmtResearchLine(ord int, topic, finding, src string) string {
	return fmt.Sprintf("R%d|%s|%s|%s", ord, esc(topic), esc(finding), esc(src))
}

func fmtInvariantLine(ord int, text string) string {
	return fmt.Sprintf("V%d: %s", ord, text)
}

func fmtBugLine(ord int, date, cause, fix string) string {
	return fmt.Sprintf("B%d|%s|%s|%s", ord, esc(date), esc(cause), fix)
}

func fmtTaskLine(ord int, status, text, cites string) string {
	return fmt.Sprintf("T%d|%s|%s|%s", ord, status, esc(text), cites)
}

func fmtUnknownLine(ord int, status, text string) string {
	return fmt.Sprintf("U%d|%s|%s", ord, status, esc(text))
}

// renderSpec renders one project's slice of the db to SPEC.md text. Pure
// function of (db, project) state: every query filters by project_id (V20),
// orders by the per-project ordinal, and emits nothing volatile (V1, V7).
func renderSpec(db *sql.DB, projectID int64) (string, error) {
	var b strings.Builder
	b.WriteString(generatedHead + "\n# SPEC\n")

	for _, render := range []func(*sql.DB, int64, *strings.Builder) error{
		renderInterfaces, renderResearch, renderInvariants, renderBugs, renderFeatures,
	} {
		if err := render(db, projectID, &b); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

func renderInterfaces(db *sql.DB, projectID int64, b *strings.Builder) error {
	rows, err := db.Query(`SELECT kind, name, sig, status FROM interface WHERE project_id=? ORDER BY id`, projectID)
	if err != nil {
		return err
	}
	defer rows.Close()
	b.WriteString("\n## §I INTERFACES\n")
	for rows.Next() {
		var kind, name, sig, status string
		if err := rows.Scan(&kind, &name, &sig, &status); err != nil {
			return err
		}
		fmt.Fprintln(b, fmtInterfaceLine(kind, name, sig, status))
	}
	return rows.Err()
}

func renderResearch(db *sql.DB, projectID int64, b *strings.Builder) error {
	rows, err := db.Query(`SELECT ord, topic, finding, src FROM research WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return err
	}
	defer rows.Close()
	b.WriteString("\n## §R RESEARCH\nid|topic|finding|src\n")
	for rows.Next() {
		var ord int
		var topic, finding, src string
		if err := rows.Scan(&ord, &topic, &finding, &src); err != nil {
			return err
		}
		fmt.Fprintln(b, fmtResearchLine(ord, topic, finding, src))
	}
	return rows.Err()
}

func renderInvariants(db *sql.DB, projectID int64, b *strings.Builder) error {
	rows, err := db.Query(`SELECT ord, text FROM invariant WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return err
	}
	defer rows.Close()
	b.WriteString("\n## §V INVARIANTS\n")
	for rows.Next() {
		var ord int
		var text string
		if err := rows.Scan(&ord, &text); err != nil {
			return err
		}
		fmt.Fprintln(b, fmtInvariantLine(ord, text))
	}
	return rows.Err()
}

func renderBugs(db *sql.DB, projectID int64, b *strings.Builder) error {
	rows, err := db.Query(`SELECT id, ord, date, cause FROM bug WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return err
	}
	defer rows.Close()
	b.WriteString("\n## §B BUGS\nid|date|cause|fix\n")
	type bug struct {
		pk          int64
		ord         int
		date, cause string
	}
	var bugs []bug
	for rows.Next() {
		var bg bug
		if err := rows.Scan(&bg.pk, &bg.ord, &bg.date, &bg.cause); err != nil {
			return err
		}
		bugs = append(bugs, bg)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, bg := range bugs {
		fix, err := bugFix(db, bg.pk)
		if err != nil {
			return err
		}
		fmt.Fprintln(b, fmtBugLine(bg.ord, bg.date, bg.cause, fix))
	}
	return nil
}

// bugFix renders a bug's fix links as the cited invariants' per-project ords.
func bugFix(db *sql.DB, bugPK int64) (string, error) {
	rows, err := db.Query(`SELECT i.ord FROM bug_fix j JOIN invariant i ON i.id=j.inv_id WHERE j.bug_id=? ORDER BY i.ord`, bugPK)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var ord int
		if err := rows.Scan(&ord); err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("V%d", ord))
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(parts) == 0 {
		return "-", nil
	}
	return strings.Join(parts, ","), nil
}

func renderFeatures(db *sql.DB, projectID int64, b *strings.Builder) error {
	rows, err := db.Query(`SELECT id, ord, name FROM feature WHERE project_id=? ORDER BY ord`, projectID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type feat struct {
		pk   int64
		ord  int
		name string
	}
	var feats []feat
	for rows.Next() {
		var f feat
		if err := rows.Scan(&f.pk, &f.ord, &f.name); err != nil {
			return err
		}
		feats = append(feats, f)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, f := range feats {
		fmt.Fprintf(b, "\n## FEATURE %d: %s\n", f.ord, f.name)
		if err := renderTextList(db, b, "### §G GOAL", `SELECT text FROM goal WHERE feature_id=? ORDER BY id`, f.pk, false); err != nil {
			return err
		}
		if err := renderTextList(db, b, "### §C CONSTRAINTS", `SELECT text FROM "constraint" WHERE feature_id=? ORDER BY id`, f.pk, true); err != nil {
			return err
		}
		if err := renderTasks(db, b, f.pk); err != nil {
			return err
		}
	}
	return nil
}

// renderTextList writes a header then each row's text, as bullets if bullet.
func renderTextList(db *sql.DB, b *strings.Builder, header, query string, featurePK int64, bullet bool) error {
	rows, err := db.Query(query, featurePK)
	if err != nil {
		return err
	}
	defer rows.Close()
	fmt.Fprintf(b, "%s\n", header)
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return err
		}
		if bullet {
			fmt.Fprintf(b, "- %s\n", text)
		} else {
			fmt.Fprintf(b, "%s\n", text)
		}
	}
	return rows.Err()
}

func renderTasks(db *sql.DB, b *strings.Builder, featurePK int64) error {
	rows, err := db.Query(`SELECT id, ord, status, text FROM task WHERE feature_id=? ORDER BY ord`, featurePK)
	if err != nil {
		return err
	}
	defer rows.Close()
	type tk struct {
		pk           int64
		ord          int
		status, text string
	}
	var tasks []tk
	for rows.Next() {
		var t tk
		if err := rows.Scan(&t.pk, &t.ord, &t.status, &t.text); err != nil {
			return err
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	b.WriteString("### §T TASKS\nid|status|task|cites\n")
	for _, t := range tasks {
		cites, err := taskCites(db, t.pk)
		if err != nil {
			return err
		}
		fmt.Fprintln(b, fmtTaskLine(t.ord, t.status, t.text, cites))
	}
	return nil
}

// taskCites re-joins the typed cite tables into "V1,I.init" form: invariants by
// their per-project ord, interfaces by name. Ordered.
func taskCites(db *sql.DB, taskPK int64) (string, error) {
	var parts []string
	ir, err := db.Query(`SELECT i.ord FROM task_cites_inv j JOIN invariant i ON i.id=j.inv_id WHERE j.task_id=? ORDER BY i.ord`, taskPK)
	if err != nil {
		return "", err
	}
	for ir.Next() {
		var ord int
		if err := ir.Scan(&ord); err != nil {
			ir.Close()
			return "", err
		}
		parts = append(parts, fmt.Sprintf("V%d", ord))
	}
	ir.Close()
	if err := ir.Err(); err != nil {
		return "", err
	}

	fr, err := db.Query(`SELECT i.name FROM task_cites_iface j JOIN interface i ON i.id=j.iface_id WHERE j.task_id=? ORDER BY i.id`, taskPK)
	if err != nil {
		return "", err
	}
	for fr.Next() {
		var name string
		if err := fr.Scan(&name); err != nil {
			fr.Close()
			return "", err
		}
		parts = append(parts, "I."+name)
	}
	fr.Close()
	if err := fr.Err(); err != nil {
		return "", err
	}

	if len(parts) == 0 {
		return "-", nil
	}
	return strings.Join(parts, ","), nil
}

// exportSpec renders the project and atomically replaces SPEC.md at path (write
// temp + rename, V8).
func exportSpec(db *sql.DB, projectID int64, path string) error {
	content, err := renderSpec(db, projectID)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "regenerate the current project's SPEC.md from the global db",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, pid, specFile, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()
			return exportSpec(db, pid, specFile)
		},
	}
}
