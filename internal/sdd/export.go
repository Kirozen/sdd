package sdd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	dbq "github.com/kirozen/sdd/internal/db"
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

// featureRow is the common shape the feature renderers need (V26 ordinal +
// name + PK for child lookups). sqlc emits a distinct row type per query even
// when columns match, so every selector maps into this one struct.
type featureRow struct {
	Name string
	ID   int64
	Ord  int64
}

// featureSel picks which features a render emits; durables are never scoped
// (V76). allFeatures is the full set used by export AND check (V77).
type featureSel func(*sql.DB, int64) ([]featureRow, error)

func allFeatures(db *sql.DB, projectID int64) ([]featureRow, error) {
	rows, err := dbq.New(db).FeaturesByProject(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	out := make([]featureRow, len(rows))
	for i, r := range rows {
		out[i] = featureRow{ID: r.ID, Ord: r.Ord, Name: r.Name}
	}
	return out, nil
}

// openFeatures = the unfinished features (V75): not in the built stage — a
// non-x task OR zero tasks. Empty (all built) is valid, not an error.
func openFeatures(db *sql.DB, projectID int64) ([]featureRow, error) {
	rows, err := dbq.New(db).OpenFeaturesByProject(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	out := make([]featureRow, len(rows))
	for i, r := range rows {
		out[i] = featureRow{ID: r.ID, Ord: r.Ord, Name: r.Name}
	}
	return out, nil
}

// featureByOrd selects exactly one feature by its ordinal; an unknown ord is an
// error so `sdd cat --feature N` exits non-zero (V75), unlike the open selector.
func featureByOrd(ord int64) featureSel {
	return func(db *sql.DB, projectID int64) ([]featureRow, error) {
		rows, err := dbq.New(db).FeatureByOrd(context.Background(), dbq.FeatureByOrdParams{
			ProjectID: projectID, Ord: ord,
		})
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, fmt.Errorf("no feature %d in this project", ord)
		}
		out := make([]featureRow, len(rows))
		for i, r := range rows {
			out[i] = featureRow{ID: r.ID, Ord: r.Ord, Name: r.Name}
		}
		return out, nil
	}
}

// renderSpec renders the FULL spec (all features) to SPEC.md text. Both
// exportSpec and checkSpec call it, so the whole-file render IS the V6 drift
// contract (V77); the scoped path (sdd cat) goes through renderSpecScoped.
func renderSpec(db *sql.DB, projectID int64) (string, error) {
	return renderSpecScoped(db, projectID, allFeatures)
}

// renderSpecScoped renders durables in full (V76) plus the features sel picks.
// Pure function of (db, project, sel): every query filters by project_id (V20),
// orders by the per-project ordinal, and emits nothing volatile (V1, V7).
func renderSpecScoped(db *sql.DB, projectID int64, sel featureSel) (string, error) {
	var b strings.Builder
	b.WriteString(generatedHead + "\n# SPEC\n")

	for _, render := range []func(*sql.DB, int64, *strings.Builder) error{
		renderInterfaces, renderResearch, renderInvariants, renderBugs,
	} {
		if err := render(db, projectID, &b); err != nil {
			return "", err
		}
	}
	feats, err := sel(db, projectID)
	if err != nil {
		return "", err
	}
	if err := renderFeatures(db, feats, &b); err != nil {
		return "", err
	}
	return b.String(), nil
}

func renderInterfaces(db *sql.DB, projectID int64, b *strings.Builder) error {
	rows, err := dbq.New(db).InterfacesByProject(context.Background(), projectID)
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
	rows, err := dbq.New(db).ResearchByProject(context.Background(), projectID)
	if err != nil {
		return err
	}
	b.WriteString("\n## §R RESEARCH\nid|topic|finding|src\n")
	for _, r := range rows {
		fmt.Fprintln(b, fmtResearchLine(int(r.Ord), r.Topic, r.Finding, r.Src))
	}
	return nil
}

func renderInvariants(db *sql.DB, projectID int64, b *strings.Builder) error {
	rows, err := dbq.New(db).InvariantsByProject(context.Background(), projectID)
	if err != nil {
		return err
	}
	b.WriteString("\n## §V INVARIANTS\n")
	for _, r := range rows {
		fmt.Fprintln(b, fmtInvariantLine(int(r.Ord), r.Text))
	}
	return nil
}

func renderBugs(db *sql.DB, projectID int64, b *strings.Builder) error {
	rows, err := dbq.New(db).BugsByProject(context.Background(), projectID)
	if err != nil {
		return err
	}
	b.WriteString("\n## §B BUGS\nid|date|cause|fix\n")
	for _, r := range rows {
		fix, err := bugFix(db, r.ID)
		if err != nil {
			return err
		}
		fmt.Fprintln(b, fmtBugLine(int(r.Ord), r.Date, r.Cause, fix))
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
		parts[i] = fmt.Sprintf("V%d", o)
	}
	return strings.Join(parts, ","), nil
}

func renderFeatures(db *sql.DB, feats []featureRow, b *strings.Builder) error {
	for _, f := range feats {
		fmt.Fprintf(b, "\n## FEATURE %d: %s\n", int(f.Ord), f.Name)
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
		fmt.Fprintln(b, fmtTaskLine(int(t.Ord), t.Status, t.Text, cites))
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
		parts = append(parts, fmt.Sprintf("V%d", o))
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
