package main

import (
	"context"
	"database/sql"
	"fmt"

	dbq "github.com/kirozen/sdd/db"
	"github.com/spf13/cobra"
)

// listKind returns every row of a kind in the current project as caveman lines,
// ordered by the per-project ordinal, formatted through the same fmt*Line
// helpers as renderSpec (V18). Read-pure (V16); scoped by project (V20). An
// unknown kind errors (V17); a valid-but-empty kind returns no lines.
func listKind(db *sql.DB, projectID int64, kind string) ([]string, error) {
	ctx := context.Background()
	q := dbq.New(db)
	switch kind {
	case "invariant":
		rows, err := q.InvariantsByProject(ctx, projectID)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, fmtInvariantLine(int(r.Ord), r.Text))
		}
		return out, nil
	case "interface":
		rows, err := q.InterfacesByProject(ctx, projectID)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, fmtInterfaceLine(r.Kind, r.Name, r.Sig, r.Status))
		}
		return out, nil
	case "research":
		rows, err := q.ResearchByProject(ctx, projectID)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, fmtResearchLine(int(r.Ord), r.Topic, r.Finding, r.Src))
		}
		return out, nil
	case "feature":
		rows, err := q.FeaturesByProject(ctx, projectID)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, fmt.Sprintf("FEATURE %d: %s", int(r.Ord), r.Name))
		}
		return out, nil
	case "task":
		return listTasks(db, projectID)
	case "bug":
		return listBugs(db, projectID)
	case "unknown":
		// Feature-scoped, all statuses (open + resolved), per-project ordinal U<n>.
		// Not part of listAllKinds, so `list` (no kind) omits it (V28); only an
		// explicit `list unknown` surfaces them.
		rows, err := q.UnknownsByProject(ctx, projectID)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, fmtUnknownLine(int(r.Ord), r.Status, r.Text))
		}
		return out, nil
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
	rows, err := dbq.New(db).InterfacesByProject(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	var refs, bodies []string
	for _, r := range rows {
		mark := ""
		if r.Status == "deprecated" {
			mark = " [deprecated]"
		}
		refs = append(refs, "I."+r.Name)
		bodies = append(bodies, fmt.Sprintf("(%s) %s%s", r.Kind, r.Sig, mark))
	}
	return alignRows(refs, bodies), nil
}

func prettyResearch(db *sql.DB, projectID int64) ([]string, error) {
	rows, err := dbq.New(db).ResearchByProject(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	var refs, bodies []string
	for _, r := range rows {
		refs = append(refs, fmt.Sprintf("R%d", int(r.Ord)))
		bodies = append(bodies, fmt.Sprintf("%s — %s  (%s)", r.Topic, r.Finding, r.Src))
	}
	return alignRows(refs, bodies), nil
}

func prettyInvariants(db *sql.DB, projectID int64) ([]string, error) {
	rows, err := dbq.New(db).InvariantsByProject(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	var refs, bodies []string
	for _, r := range rows {
		refs = append(refs, fmt.Sprintf("V%d", int(r.Ord)))
		bodies = append(bodies, r.Text)
	}
	return alignRows(refs, bodies), nil
}

func prettyBugs(db *sql.DB, projectID int64) ([]string, error) {
	rows, err := dbq.New(db).BugsByProject(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	var refs, bodies []string
	for _, b := range rows {
		fix, err := bugFix(db, b.ID)
		if err != nil {
			return nil, err
		}
		body := fmt.Sprintf("%s  %s", b.Date, b.Cause)
		if fix != "-" {
			body += "  → fixed " + fix
		}
		refs = append(refs, fmt.Sprintf("B%d", int(b.Ord)))
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
	feats, err := dbq.New(db).FeaturesByProject(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	var out []prettyFeature
	for _, f := range feats {
		tasks, err := prettyTasks(db, f.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, prettyFeature{
			header: fmt.Sprintf("FEATURE %d — %s", int(f.Ord), f.Name),
			tasks:  tasks,
		})
	}
	return out, nil
}

func prettyTasks(db *sql.DB, featurePK int64) ([]string, error) {
	tasks, err := dbq.New(db).TasksByFeature(context.Background(), featurePK)
	if err != nil {
		return nil, err
	}
	var refs, bodies []string
	for _, t := range tasks {
		cites, err := taskCites(db, t.ID)
		if err != nil {
			return nil, err
		}
		body := fmt.Sprintf("%s  %s", statusGlyph(t.Status), t.Text)
		if cites != "-" {
			body += "  → " + cites
		}
		refs = append(refs, fmt.Sprintf("T%d", int(t.Ord)))
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

// task is a feature's task as the list/pretty paths consume it, decoupled from
// the per-query generated row types (which all carry the same four fields).
type taskRow struct {
	pk           int64
	ord          int
	status, text string
}

func listTasks(db *sql.DB, projectID int64) ([]string, error) {
	return listTasksFiltered(db, projectID, "", 0)
}

// listTasksFiltered lists a project's tasks, optionally narrowed by status
// (V38: "" = any, else exactly one of .|~|x) and feature ordinal (0 = any, else
// the feature must exist or it errors). The feature is resolved to its PK first
// so a missing one errors (V38/V53) rather than silently returning empty; the
// four generated queries cover the filter combinations. Lines render through
// fmtTaskLine (V18).
func listTasksFiltered(db *sql.DB, projectID int64, status string, featureOrd int64) ([]string, error) {
	ctx := context.Background()
	q := dbq.New(db)

	var featPK int64
	if featureOrd > 0 {
		pk, err := featurePK(db, projectID, featureOrd) // missing feature → error (V38)
		if err != nil {
			return nil, err
		}
		featPK = pk
	}

	var tasks []taskRow
	switch {
	case featureOrd > 0 && status != "":
		rows, err := q.TasksInProjectByFeatureStatus(ctx, dbq.TasksInProjectByFeatureStatusParams{
			ProjectID: projectID, FeatureID: featPK, Status: status,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			tasks = append(tasks, taskRow{r.ID, int(r.Ord), r.Status, r.Text})
		}
	case featureOrd > 0:
		rows, err := q.TasksInProjectByFeature(ctx, dbq.TasksInProjectByFeatureParams{
			ProjectID: projectID, FeatureID: featPK,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			tasks = append(tasks, taskRow{r.ID, int(r.Ord), r.Status, r.Text})
		}
	case status != "":
		rows, err := q.TasksInProjectByStatus(ctx, dbq.TasksInProjectByStatusParams{
			ProjectID: projectID, Status: status,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			tasks = append(tasks, taskRow{r.ID, int(r.Ord), r.Status, r.Text})
		}
	default:
		rows, err := q.TasksInProject(ctx, projectID)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			tasks = append(tasks, taskRow{r.ID, int(r.Ord), r.Status, r.Text})
		}
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
	rows, err := dbq.New(db).BugsByProject(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, b := range rows {
		fix, err := bugFix(db, b.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, fmtBugLine(int(b.Ord), b.Date, b.Cause, fix))
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
