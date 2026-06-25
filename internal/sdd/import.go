package sdd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	dbq "github.com/kirozen/sdd/internal/db"
	"github.com/spf13/cobra"
)

// --- parsed model ---

type (
	parsedIface struct{ kind, name, sig string }
	parsedInv   struct {
		text string
		id   int64
	}
)

type parsedResearch struct {
	topic, finding, src string
	id                  int64
}
type parsedBug struct {
	date  string
	cause string
	fix   []string
	id    int64
}
type parsedTask struct {
	status string
	text   string
	cites  []string
	id     int64
}

type parsedSpec struct {
	goal        string
	constraints []string
	interfaces  []parsedIface
	research    []parsedResearch
	invariants  []parsedInv
	bugs        []parsedBug
	tasks       []parsedTask
}

// splitPipe splits a pipe-table row on unescaped `|`, unescaping `\|` to `|`.
func splitPipe(s string) []string {
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '|' {
			cur.WriteByte('|')
			i++
			continue
		}
		if s[i] == '|' {
			cells = append(cells, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	cells = append(cells, cur.String())
	return cells
}

func firstBacktick(s string) string {
	i := strings.Index(s, "`")
	if i < 0 {
		return s
	}
	j := strings.Index(s[i+1:], "`")
	if j < 0 {
		return s[i+1:]
	}
	return s[i+1 : i+1+j]
}

// deriveIfaceName extracts the cite key from an interface bullet: for a `cmd:`
// with an `sdd` prefix it is the subcommand (2nd token), else the first token.
func deriveIfaceName(kind, sig string) string {
	toks := strings.Fields(firstBacktick(sig))
	if len(toks) == 0 {
		return ""
	}
	if kind == "cmd" && toks[0] == "sdd" && len(toks) >= 2 {
		return toks[1]
	}
	return toks[0]
}

// parseSpec reads a cavekit-format SPEC.md into the parsed model. Unrecognized
// prose is skipped; integrity is enforced later at seed time (FK).
func parseSpec(content string) *parsedSpec {
	ps := &parsedSpec{}
	var goalLines []string
	section := ""

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimRight(raw, " \t")
		if strings.HasPrefix(line, "## ") {
			section = sectionOf(line)
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch section {
		case "G":
			goalLines = append(goalLines, trimmed)
		case "C":
			if b, ok := bullet(trimmed); ok {
				ps.constraints = append(ps.constraints, b)
			}
		case "I":
			if b, ok := bullet(trimmed); ok {
				if k, n, sig, ok := parseIface(b); ok {
					ps.interfaces = append(ps.interfaces, parsedIface{k, n, sig})
				}
			}
		case "R":
			if c := splitPipe(trimmed); len(c) == 4 && c[0] != "id" {
				if id, ok := rowID(c[0]); ok {
					ps.research = append(ps.research, parsedResearch{id: id, topic: c[1], finding: c[2], src: c[3]})
				}
			}
		case "V":
			if id, text, ok := parseInvariant(trimmed); ok {
				ps.invariants = append(ps.invariants, parsedInv{id: id, text: text})
			}
		case "B":
			if c := splitPipe(trimmed); len(c) == 4 && c[0] != "id" {
				if id, ok := rowID(c[0]); ok {
					ps.bugs = append(ps.bugs, parsedBug{id: id, date: c[1], cause: c[2], fix: splitRefs(c[3])})
				}
			}
		case "T":
			if c := splitPipe(trimmed); len(c) == 4 && c[0] != "id" {
				if id, ok := rowID(c[0]); ok {
					ps.tasks = append(ps.tasks, parsedTask{id: id, status: c[1], text: c[2], cites: splitRefs(c[3])})
				}
			}
		}
	}
	ps.goal = strings.Join(goalLines, " ")
	return ps
}

func sectionOf(header string) string {
	for _, s := range []string{"G", "C", "I", "R", "V", "B", "T"} {
		if strings.Contains(header, "§"+s) {
			return s
		}
	}
	return ""
}

func bullet(s string) (string, bool) {
	if strings.HasPrefix(s, "- ") {
		return strings.TrimSpace(s[2:]), true
	}
	return "", false
}

func parseIface(body string) (kind, name, sig string, ok bool) {
	colon := strings.Index(body, ":")
	if colon < 0 {
		return "", "", "", false
	}
	kind = strings.TrimSpace(body[:colon])
	sig = strings.TrimSpace(body[colon+1:])
	name = deriveIfaceName(kind, sig)
	if name == "" {
		return "", "", "", false
	}
	return kind, name, sig, true
}

func parseInvariant(line string) (int64, string, bool) {
	if !strings.HasPrefix(line, "V") {
		return 0, "", false
	}
	idx := strings.Index(line, ": ")
	if idx < 1 {
		return 0, "", false
	}
	n, err := strconv.ParseInt(line[1:idx], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return n, line[idx+2:], true
}

// rowID strips the section-letter prefix (B1, R3, T7) to its number.
func rowID(cell string) (int64, bool) {
	if len(cell) < 2 {
		return 0, false
	}
	n, err := strconv.ParseInt(cell[1:], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// --- seed ---

func dbEmpty(db *sql.DB, projectID int64) (bool, error) {
	n, err := dbq.New(db).ProjectRowCount(context.Background(), dbq.ProjectRowCountParams{
		ProjectID: projectID, ProjectID_2: projectID, ProjectID_3: projectID,
		ProjectID_4: projectID, ProjectID_5: projectID,
	})
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

// seedDB loads a parsed spec into one project in a single transaction, in
// dependency order so every cite resolves (V14). The parsed V<n>/T<n>/… numbers
// become per-project ordinals (V26); the global PKs are fresh, so importing the
// same spec into a different project never collides (B-1). If clear, the
// project's existing rows are removed first (--force). Any failure rolls back.
func seedDB(db *sql.DB, projectID int64, ps *parsedSpec, featureName string, clear bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := dbq.New(tx)
	ctx := context.Background()
	if clear {
		for _, del := range []func(context.Context, int64) error{
			q.DeleteProjectFeatures, q.DeleteProjectBugs, q.DeleteProjectInvariants,
			q.DeleteProjectInterfaces, q.DeleteProjectResearch,
		} {
			if err := del(ctx, projectID); err != nil {
				return err
			}
		}
	}

	for _, inv := range ps.invariants {
		if err := q.InsertInvariant(ctx, dbq.InsertInvariantParams{ProjectID: projectID, Ord: int64(inv.id), Text: inv.text}); err != nil {
			return fmt.Errorf("invariant V%d: %w", inv.id, err)
		}
	}
	ifaceID := map[string]int64{}
	for _, f := range ps.interfaces {
		id, err := q.InsertInterface(ctx, dbq.InsertInterfaceParams{ProjectID: projectID, Kind: f.kind, Name: f.name, Sig: f.sig})
		if err != nil {
			return fmt.Errorf("interface I.%s: %w", f.name, err)
		}
		ifaceID[f.name] = id
	}
	for _, r := range ps.research {
		if err := q.InsertResearch(ctx, dbq.InsertResearchParams{ProjectID: projectID, Ord: int64(r.id), Topic: r.topic, Finding: r.finding, Src: r.src}); err != nil {
			return fmt.Errorf("research R%d: %w", r.id, err)
		}
	}
	for _, bg := range ps.bugs {
		bugPK, err := q.InsertBug(ctx, dbq.InsertBugParams{ProjectID: projectID, Ord: int64(bg.id), Date: bg.date, Cause: bg.cause})
		if err != nil {
			return fmt.Errorf("bug B%d: %w", bg.id, err)
		}
		for _, ref := range bg.fix {
			n, err := strconv.Atoi(strings.TrimPrefix(ref, "V"))
			if err != nil {
				return fmt.Errorf("bug B%d bad fix %q", bg.id, ref)
			}
			invID, err := q.InvariantIDByOrd(ctx, dbq.InvariantIDByOrdParams{ProjectID: projectID, Ord: int64(n)})
			if err != nil {
				return fmt.Errorf("bug B%d fix %s: no such invariant", bg.id, ref)
			}
			if err := q.InsertBugFix(ctx, dbq.InsertBugFixParams{BugID: bugPK, InvID: invID}); err != nil {
				return fmt.Errorf("bug B%d fix %s: %w", bg.id, ref, err)
			}
		}
	}

	ford, err := q.NextFeatureOrd(ctx, projectID)
	if err != nil {
		return err
	}
	fid, err := q.InsertFeature(ctx, dbq.InsertFeatureParams{ProjectID: projectID, Ord: ford, Name: featureName})
	if err != nil {
		return err
	}
	if ps.goal != "" {
		if err := q.InsertGoal(ctx, dbq.InsertGoalParams{FeatureID: fid, Text: ps.goal}); err != nil {
			return err
		}
	}
	for _, c := range ps.constraints {
		if err := q.InsertConstraint(ctx, dbq.InsertConstraintParams{FeatureID: fid, Text: c}); err != nil {
			return err
		}
	}
	for _, tk := range ps.tasks {
		taskPK, err := q.InsertTaskFull(ctx, dbq.InsertTaskFullParams{FeatureID: fid, Ord: int64(tk.id), Text: tk.text, Status: tk.status})
		if err != nil {
			return fmt.Errorf("task T%d: %w", tk.id, err)
		}
		for _, cite := range tk.cites {
			if err := seedCite(tx, projectID, taskPK, cite, ifaceID); err != nil {
				return fmt.Errorf("task T%d: %w", tk.id, err)
			}
		}
	}

	return tx.Commit()
}

func seedCite(tx *sql.Tx, projectID, taskPK int64, cite string, ifaceID map[string]int64) error {
	ctx := context.Background()
	q := dbq.New(tx)
	switch {
	case strings.HasPrefix(cite, "V"):
		ord, err := strconv.Atoi(cite[1:])
		if err != nil {
			return fmt.Errorf("bad cite %q", cite)
		}
		invID, err := q.InvariantIDByOrd(ctx, dbq.InvariantIDByOrdParams{ProjectID: projectID, Ord: int64(ord)})
		if err != nil {
			return fmt.Errorf("cite %s: no such invariant", cite)
		}
		if err := q.InsertTaskCiteInv(ctx, dbq.InsertTaskCiteInvParams{TaskID: taskPK, InvID: invID}); err != nil {
			return fmt.Errorf("cite %s: %w", cite, err)
		}
	case strings.HasPrefix(cite, "I."):
		id, ok := ifaceID[cite[2:]]
		if !ok {
			return fmt.Errorf("cite %s: no such interface", cite)
		}
		if err := q.InsertTaskCiteIface(ctx, dbq.InsertTaskCiteIfaceParams{TaskID: taskPK, IfaceID: id}); err != nil {
			return fmt.Errorf("cite %s: %w", cite, err)
		}
	default:
		return fmt.Errorf("unrecognized cite %q", cite)
	}
	return nil
}

func newImportCmd() *cobra.Command {
	var force bool
	var name string
	c := &cobra.Command{
		Use:   "import <file>",
		Short: "bootstrap an empty spec.db from a cavekit-format SPEC.md",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			db, pid, specFile, err := openProjectContext()
			if err != nil {
				return err
			}
			defer db.Close()

			empty, err := dbEmpty(db, pid)
			if err != nil {
				return err
			}
			if !empty && !force {
				return fmt.Errorf("this project already has rows; pass --force to reseed")
			}

			featureName := name
			if featureName == "" {
				featureName = strings.TrimSuffix(filepath.Base(args[0]), filepath.Ext(args[0]))
			}
			if err := seedDB(db, pid, parseSpec(string(content)), featureName, !empty); err != nil {
				return err
			}
			return exportSpec(db, pid, specFile)
		},
	}
	c.Flags().BoolVar(&force, "force", false, "reseed even if spec.db is not empty")
	c.Flags().StringVar(&name, "name", "", "feature name (default: file stem)")
	return c
}
