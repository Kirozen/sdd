package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// --- parsed model ---

type parsedIface struct{ kind, name, sig string }
type parsedInv struct {
	id   int64
	text string
}
type parsedResearch struct {
	id                   int64
	topic, finding, src  string
}
type parsedBug struct {
	id        int64
	date      string
	cause     string
	fix       []string
}
type parsedTask struct {
	id     int64
	status string
	text   string
	cites  []string
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
					ps.research = append(ps.research, parsedResearch{id, c[1], c[2], c[3]})
				}
			}
		case "V":
			if id, text, ok := parseInvariant(trimmed); ok {
				ps.invariants = append(ps.invariants, parsedInv{id, text})
			}
		case "B":
			if c := splitPipe(trimmed); len(c) == 4 && c[0] != "id" {
				if id, ok := rowID(c[0]); ok {
					ps.bugs = append(ps.bugs, parsedBug{id, c[1], c[2], splitRefs(c[3])})
				}
			}
		case "T":
			if c := splitPipe(trimmed); len(c) == 4 && c[0] != "id" {
				if id, ok := rowID(c[0]); ok {
					ps.tasks = append(ps.tasks, parsedTask{id, c[1], c[2], splitRefs(c[3])})
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
	for _, t := range []string{"invariant", "interface", "bug", "research", "feature"} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM "`+t+`" WHERE project_id=?`, projectID).Scan(&n); err != nil {
			return false, err
		}
		if n > 0 {
			return false, nil
		}
	}
	return true, nil
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

	if clear {
		for _, t := range []string{"feature", "bug", "invariant", "interface", "research"} {
			if _, err := tx.Exec(`DELETE FROM "`+t+`" WHERE project_id=?`, projectID); err != nil {
				return err
			}
		}
	}

	for _, inv := range ps.invariants {
		if _, err := tx.Exec(`INSERT INTO invariant(project_id, ord, text) VALUES(?, ?, ?)`, projectID, inv.id, inv.text); err != nil {
			return fmt.Errorf("invariant V%d: %w", inv.id, err)
		}
	}
	ifaceID := map[string]int64{}
	for _, f := range ps.interfaces {
		res, err := tx.Exec(`INSERT INTO interface(project_id, kind, name, sig) VALUES(?, ?, ?, ?)`, projectID, f.kind, f.name, f.sig)
		if err != nil {
			return fmt.Errorf("interface I.%s: %w", f.name, err)
		}
		id, _ := res.LastInsertId()
		ifaceID[f.name] = id
	}
	for _, r := range ps.research {
		if _, err := tx.Exec(`INSERT INTO research(project_id, ord, topic, finding, src) VALUES(?, ?, ?, ?, ?)`, projectID, r.id, r.topic, r.finding, r.src); err != nil {
			return fmt.Errorf("research R%d: %w", r.id, err)
		}
	}
	for _, bg := range ps.bugs {
		res, err := tx.Exec(`INSERT INTO bug(project_id, ord, date, cause) VALUES(?, ?, ?, ?)`, projectID, bg.id, bg.date, bg.cause)
		if err != nil {
			return fmt.Errorf("bug B%d: %w", bg.id, err)
		}
		bugPK, _ := res.LastInsertId()
		for _, ref := range bg.fix {
			n, err := strconv.Atoi(strings.TrimPrefix(ref, "V"))
			if err != nil {
				return fmt.Errorf("bug B%d bad fix %q", bg.id, ref)
			}
			var invID int64
			if err := tx.QueryRow(`SELECT id FROM invariant WHERE project_id=? AND ord=?`, projectID, n).Scan(&invID); err != nil {
				return fmt.Errorf("bug B%d fix %s: no such invariant", bg.id, ref)
			}
			if _, err := tx.Exec(`INSERT INTO bug_fix(bug_id, inv_id) VALUES(?, ?)`, bugPK, invID); err != nil {
				return fmt.Errorf("bug B%d fix %s: %w", bg.id, ref, err)
			}
		}
	}

	var ford int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(ord),0)+1 FROM feature WHERE project_id=?`, projectID).Scan(&ford); err != nil {
		return err
	}
	res, err := tx.Exec(`INSERT INTO feature(project_id, ord, name) VALUES(?, ?, ?)`, projectID, ford, featureName)
	if err != nil {
		return err
	}
	fid, _ := res.LastInsertId()
	if ps.goal != "" {
		if _, err := tx.Exec(`INSERT INTO goal(feature_id, text) VALUES(?, ?)`, fid, ps.goal); err != nil {
			return err
		}
	}
	for _, c := range ps.constraints {
		if _, err := tx.Exec(`INSERT INTO "constraint"(feature_id, text) VALUES(?, ?)`, fid, c); err != nil {
			return err
		}
	}
	for _, tk := range ps.tasks {
		res, err := tx.Exec(`INSERT INTO task(feature_id, ord, text, status) VALUES(?, ?, ?, ?)`, fid, tk.id, tk.text, tk.status)
		if err != nil {
			return fmt.Errorf("task T%d: %w", tk.id, err)
		}
		taskPK, _ := res.LastInsertId()
		for _, cite := range tk.cites {
			if err := seedCite(tx, projectID, taskPK, cite, ifaceID); err != nil {
				return fmt.Errorf("task T%d: %w", tk.id, err)
			}
		}
	}

	return tx.Commit()
}

func seedCite(tx *sql.Tx, projectID, taskPK int64, cite string, ifaceID map[string]int64) error {
	switch {
	case strings.HasPrefix(cite, "V"):
		ord, err := strconv.Atoi(cite[1:])
		if err != nil {
			return fmt.Errorf("bad cite %q", cite)
		}
		var invID int64
		if err := tx.QueryRow(`SELECT id FROM invariant WHERE project_id=? AND ord=?`, projectID, ord).Scan(&invID); err != nil {
			return fmt.Errorf("cite %s: no such invariant", cite)
		}
		if _, err := tx.Exec(`INSERT INTO task_cites_inv(task_id, inv_id) VALUES(?, ?)`, taskPK, invID); err != nil {
			return fmt.Errorf("cite %s: %w", cite, err)
		}
	case strings.HasPrefix(cite, "I."):
		id, ok := ifaceID[cite[2:]]
		if !ok {
			return fmt.Errorf("cite %s: no such interface", cite)
		}
		if _, err := tx.Exec(`INSERT INTO task_cites_iface(task_id, iface_id) VALUES(?, ?)`, taskPK, id); err != nil {
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
