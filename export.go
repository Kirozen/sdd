package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	specPath      = "SPEC.md"
	generatedHead = "<!-- GENERATED — DO NOT EDIT -->"
)

// openProjectDB opens ./spec.db, erroring if the project was not init'd.
func openProjectDB() (*sql.DB, error) {
	if _, err := os.Stat("spec.db"); err != nil {
		return nil, fmt.Errorf("no spec.db here; run `sdd init` first")
	}
	return open("spec.db")
}

// esc escapes a literal pipe so it survives a pipe-table cell.
func esc(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

// --- per-row line formatters: the single source of caveman line rendering.
// Both renderSpec (whole sections) and `sdd show` (one row) go through these,
// so a show line is byte-identical to its SPEC.md line (V18).

func fmtInterfaceLine(kind, name, sig, status string) string {
	mark := ""
	if status == "deprecated" {
		mark = " [deprecated]"
	}
	return fmt.Sprintf("- %s: %s → %s (I.%s)%s", kind, name, sig, name, mark)
}

func fmtResearchLine(id int, topic, finding, src string) string {
	return fmt.Sprintf("R%d|%s|%s|%s", id, esc(topic), esc(finding), esc(src))
}

func fmtInvariantLine(id int, text string) string {
	return fmt.Sprintf("V%d: %s", id, text)
}

func fmtBugLine(id int, date, cause, fix string) string {
	return fmt.Sprintf("B%d|%s|%s|%s", id, esc(date), esc(cause), fix)
}

func fmtTaskLine(id int, status, text, cites string) string {
	return fmt.Sprintf("T%d|%s|%s|%s", id, status, esc(text), cites)
}

// renderSpec renders the whole db to the SPEC.md text. Pure function of db
// state: every query is ORDER BY id and nothing volatile is emitted (V1, V7).
func renderSpec(db *sql.DB) (string, error) {
	var b strings.Builder
	b.WriteString(generatedHead + "\n# SPEC\n")

	if err := renderInterfaces(db, &b); err != nil {
		return "", err
	}
	if err := renderResearch(db, &b); err != nil {
		return "", err
	}
	if err := renderInvariants(db, &b); err != nil {
		return "", err
	}
	if err := renderBugs(db, &b); err != nil {
		return "", err
	}
	if err := renderFeatures(db, &b); err != nil {
		return "", err
	}
	return b.String(), nil
}

func renderInterfaces(db *sql.DB, b *strings.Builder) error {
	rows, err := db.Query(`SELECT kind, name, sig, status FROM interface ORDER BY id`)
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

func renderResearch(db *sql.DB, b *strings.Builder) error {
	rows, err := db.Query(`SELECT id, topic, finding, src FROM research ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	b.WriteString("\n## §R RESEARCH\nid|topic|finding|src\n")
	for rows.Next() {
		var id int
		var topic, finding, src string
		if err := rows.Scan(&id, &topic, &finding, &src); err != nil {
			return err
		}
		fmt.Fprintln(b, fmtResearchLine(id, topic, finding, src))
	}
	return rows.Err()
}

func renderInvariants(db *sql.DB, b *strings.Builder) error {
	rows, err := db.Query(`SELECT id, text FROM invariant ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	b.WriteString("\n## §V INVARIANTS\n")
	for rows.Next() {
		var id int
		var text string
		if err := rows.Scan(&id, &text); err != nil {
			return err
		}
		fmt.Fprintln(b, fmtInvariantLine(id, text))
	}
	return rows.Err()
}

func renderBugs(db *sql.DB, b *strings.Builder) error {
	rows, err := db.Query(`SELECT id, date, cause FROM bug ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	b.WriteString("\n## §B BUGS\nid|date|cause|fix\n")
	type bug struct {
		id          int
		date, cause string
	}
	var bugs []bug
	for rows.Next() {
		var bg bug
		if err := rows.Scan(&bg.id, &bg.date, &bg.cause); err != nil {
			return err
		}
		bugs = append(bugs, bg)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, bg := range bugs {
		fix, err := bugFix(db, bg.id)
		if err != nil {
			return err
		}
		fmt.Fprintln(b, fmtBugLine(bg.id, bg.date, bg.cause, fix))
	}
	return nil
}

func bugFix(db *sql.DB, bugID int) (string, error) {
	rows, err := db.Query(`SELECT inv_id FROM bug_fix WHERE bug_id=? ORDER BY inv_id`, bugID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("V%d", n))
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(parts) == 0 {
		return "-", nil
	}
	return strings.Join(parts, ","), nil
}

func renderFeatures(db *sql.DB, b *strings.Builder) error {
	rows, err := db.Query(`SELECT id, name FROM feature ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type feat struct {
		id   int
		name string
	}
	var feats []feat
	for rows.Next() {
		var f feat
		if err := rows.Scan(&f.id, &f.name); err != nil {
			return err
		}
		feats = append(feats, f)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, f := range feats {
		fmt.Fprintf(b, "\n## FEATURE %d: %s\n", f.id, f.name)
		if err := renderTextList(db, b, "### §G GOAL", `SELECT text FROM goal WHERE feature_id=? ORDER BY id`, f.id, false); err != nil {
			return err
		}
		if err := renderTextList(db, b, "### §C CONSTRAINTS", `SELECT text FROM "constraint" WHERE feature_id=? ORDER BY id`, f.id, true); err != nil {
			return err
		}
		if err := renderTasks(db, b, f.id); err != nil {
			return err
		}
	}
	return nil
}

// renderTextList writes a header then each row's text, as bullets if bullet.
func renderTextList(db *sql.DB, b *strings.Builder, header, query string, featureID int, bullet bool) error {
	rows, err := db.Query(query, featureID)
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

func renderTasks(db *sql.DB, b *strings.Builder, featureID int) error {
	rows, err := db.Query(`SELECT id, status, text FROM task WHERE feature_id=? ORDER BY id`, featureID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type tk struct {
		id            int
		status, text  string
	}
	var tasks []tk
	for rows.Next() {
		var t tk
		if err := rows.Scan(&t.id, &t.status, &t.text); err != nil {
			return err
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	b.WriteString("### §T TASKS\nid|status|task|cites\n")
	for _, t := range tasks {
		cites, err := taskCites(db, t.id)
		if err != nil {
			return err
		}
		fmt.Fprintln(b, fmtTaskLine(t.id, t.status, t.text, cites))
	}
	return nil
}

// taskCites re-joins the typed cite tables into "V1,I.init" form, ordered.
func taskCites(db *sql.DB, taskID int) (string, error) {
	var parts []string
	ir, err := db.Query(`SELECT inv_id FROM task_cites_inv WHERE task_id=? ORDER BY inv_id`, taskID)
	if err != nil {
		return "", err
	}
	for ir.Next() {
		var n int
		if err := ir.Scan(&n); err != nil {
			ir.Close()
			return "", err
		}
		parts = append(parts, fmt.Sprintf("V%d", n))
	}
	ir.Close()
	if err := ir.Err(); err != nil {
		return "", err
	}

	fr, err := db.Query(`SELECT i.name FROM task_cites_iface j JOIN interface i ON i.id=j.iface_id WHERE j.task_id=? ORDER BY i.id`, taskID)
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

// exportSpec renders the db and atomically replaces SPEC.md (write temp +
// rename, V8). The temp file shares the dir so rename stays on one filesystem.
func exportSpec(db *sql.DB, path string) error {
	content, err := renderSpec(db)
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
		Short: "regenerate SPEC.md from spec.db",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openProjectDB()
			if err != nil {
				return err
			}
			defer db.Close()
			return exportSpec(db, specPath)
		},
	}
}
