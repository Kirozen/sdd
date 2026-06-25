package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

const (
	specName      = "SPEC.md"
	generatedHead = "<!-- GENERATED — DO NOT EDIT -->"
)

// nz wraps a never-null int64 (project/feature id, ordinal) as the sql.NullInt64
// the generated queries take: project_id/ord are nullable columns in the schema
// but always set in practice (V20, V26). Shared by every sqlc call-site.
func nz(i int64) sql.NullInt64 { return sql.NullInt64{Int64: i, Valid: true} }

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
	rows, err := dbq.New(db).InterfacesByProject(context.Background(), nz(projectID))
	if err != nil {
		return err
	}
	b.WriteString("\n## §I INTERFACES\n")
	for _, r := range rows {
		fmt.Fprintln(b, fmtInterfaceLine(r.Kind, r.Name, r.Sig, r.Status))
	}
	return nil
}

func renderResearch(db *sql.DB, projectID int64, b *strings.Builder) error {
	rows, err := dbq.New(db).ResearchByProject(context.Background(), nz(projectID))
	if err != nil {
		return err
	}
	b.WriteString("\n## §R RESEARCH\nid|topic|finding|src\n")
	for _, r := range rows {
		fmt.Fprintln(b, fmtResearchLine(int(r.Ord.Int64), r.Topic, r.Finding, r.Src))
	}
	return nil
}

func renderInvariants(db *sql.DB, projectID int64, b *strings.Builder) error {
	rows, err := dbq.New(db).InvariantsByProject(context.Background(), nz(projectID))
	if err != nil {
		return err
	}
	b.WriteString("\n## §V INVARIANTS\n")
	for _, r := range rows {
		fmt.Fprintln(b, fmtInvariantLine(int(r.Ord.Int64), r.Text))
	}
	return nil
}

func renderBugs(db *sql.DB, projectID int64, b *strings.Builder) error {
	rows, err := dbq.New(db).BugsByProject(context.Background(), nz(projectID))
	if err != nil {
		return err
	}
	b.WriteString("\n## §B BUGS\nid|date|cause|fix\n")
	for _, r := range rows {
		fix, err := bugFix(db, r.ID)
		if err != nil {
			return err
		}
		fmt.Fprintln(b, fmtBugLine(int(r.Ord.Int64), r.Date, r.Cause, fix))
	}
	return nil
}

// bugFix renders a bug's fix links as the cited invariants' per-project ords.
func bugFix(db *sql.DB, bugPK int64) (string, error) {
	ords, err := dbq.New(db).BugFixInvOrds(context.Background(), bugPK)
	if err != nil {
		return "", err
	}
	if len(ords) == 0 {
		return "-", nil
	}
	parts := make([]string, len(ords))
	for i, o := range ords {
		parts[i] = fmt.Sprintf("V%d", o.Int64)
	}
	return strings.Join(parts, ","), nil
}

func renderFeatures(db *sql.DB, projectID int64, b *strings.Builder) error {
	feats, err := dbq.New(db).FeaturesByProject(context.Background(), nz(projectID))
	if err != nil {
		return err
	}
	for _, f := range feats {
		fmt.Fprintf(b, "\n## FEATURE %d: %s\n", int(f.Ord.Int64), f.Name)
		if err := renderGoals(db, b, f.ID); err != nil {
			return err
		}
		if err := renderConstraints(db, b, f.ID); err != nil {
			return err
		}
		if err := renderTasks(db, b, f.ID); err != nil {
			return err
		}
	}
	return nil
}

// renderGoals / renderConstraints write a header then each row's text; goals are
// plain lines, constraints are bullets (matching the prior renderTextList shape).
func renderGoals(db *sql.DB, b *strings.Builder, featurePK int64) error {
	goals, err := dbq.New(db).GoalsByFeature(context.Background(), featurePK)
	if err != nil {
		return err
	}
	b.WriteString("### §G GOAL\n")
	for _, g := range goals {
		fmt.Fprintf(b, "%s\n", g)
	}
	return nil
}

func renderConstraints(db *sql.DB, b *strings.Builder, featurePK int64) error {
	cs, err := dbq.New(db).ConstraintsByFeature(context.Background(), featurePK)
	if err != nil {
		return err
	}
	b.WriteString("### §C CONSTRAINTS\n")
	for _, c := range cs {
		fmt.Fprintf(b, "- %s\n", c)
	}
	return nil
}

func renderTasks(db *sql.DB, b *strings.Builder, featurePK int64) error {
	tasks, err := dbq.New(db).TasksByFeature(context.Background(), featurePK)
	if err != nil {
		return err
	}
	b.WriteString("### §T TASKS\nid|status|task|cites\n")
	for _, t := range tasks {
		cites, err := taskCites(db, t.ID)
		if err != nil {
			return err
		}
		fmt.Fprintln(b, fmtTaskLine(int(t.Ord.Int64), t.Status, t.Text, cites))
	}
	return nil
}

// taskCites re-joins the typed cite tables into "V1,I.init" form: invariants by
// their per-project ord, interfaces by name. Ordered.
func taskCites(db *sql.DB, taskPK int64) (string, error) {
	q := dbq.New(db)
	ctx := context.Background()
	var parts []string

	invOrds, err := q.TaskCiteInvOrds(ctx, taskPK)
	if err != nil {
		return "", err
	}
	for _, o := range invOrds {
		parts = append(parts, fmt.Sprintf("V%d", o.Int64))
	}

	names, err := q.TaskCiteIfaceNames(ctx, taskPK)
	if err != nil {
		return "", err
	}
	for _, n := range names {
		parts = append(parts, "I."+n)
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
