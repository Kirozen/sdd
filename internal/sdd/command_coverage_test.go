package sdd

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// allowlistedCmds: cobra auto-generates `help` and `completion`; they are tool
// scaffolding, not part of the spec command surface. This list is closed and
// named — adding an exemption is a visible diff in review (V107).
var allowlistedCmds = map[string]bool{"help": true, "completion": true}

// cmdReferenced reports whether content cites cmd AS A COMMAND, not as a bare
// prose word. The char immediately before the name must be a backtick (`cmd` /
// `sdd cmd`) or the literal "sdd " invocation prefix, and a word boundary must
// follow. Case-sensitive: commands are lowercase, so prose "Show"/"Status"
// never matches. Without this strict predicate, common-word commands
// (next/list/status/show/gate) would count as covered by lexical coincidence
// and the V107 oracle would be a placebo (review BLOCK-1).
func cmdReferenced(content, cmd string) bool {
	re := regexp.MustCompile("(`|sdd )" + regexp.QuoteMeta(cmd) + `\b`)
	return re.MatchString(content)
}

// specSurfaceCommands = every command on the cobra root minus the named
// allowlist, sorted for stable diffs.
func specSurfaceCommands() []string {
	var cmds []string
	for _, c := range newRootCmd().Commands() {
		if allowlistedCmds[c.Name()] {
			continue
		}
		cmds = append(cmds, c.Name())
	}
	sort.Strings(cmds)
	return cmds
}

func readDoc(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// skillCorpus reads the VERSIONED skills (../../skills, the single source per
// V86), NOT ../../.claude/skills which is a local, uncommitted dogfood install
// — scanning that would make the oracle non-hermetic on a clean clone/CI
// (review BLOCK-2).
func skillCorpus(t *testing.T) string {
	t.Helper()
	paths, err := filepath.Glob("../../skills/*/SKILL.md")
	if err != nil {
		t.Fatalf("glob skills: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no SKILL.md under ../../skills — the coverage oracle scans the versioned source (V86), not .claude/skills")
	}
	var all string
	for _, p := range paths {
		all += readDoc(t, p) + "\n"
	}
	return all
}

// V107: every spec-surface command must be referenced in a versioned skill or a
// README. Anti-drift: a command shipped without a reference fails here.
func TestEveryCommandIsReferenced(t *testing.T) {
	corpus := skillCorpus(t) + "\n" +
		readDoc(t, "../../README.fr.md") + "\n" +
		readDoc(t, "../../README.md")

	var uncovered []string
	for _, cmd := range specSurfaceCommands() {
		if !cmdReferenced(corpus, cmd) {
			uncovered = append(uncovered, cmd)
		}
	}
	if len(uncovered) > 0 {
		t.Fatalf("commands with no skill/doc reference (V107): %v — add `sdd <cmd>` to a SKILL.md or a README", uncovered)
	}
}

// V109: README.md (EN) and README.fr.md cite the SAME set of spec-surface
// commands. The goal requires EN/FR parity, which the V107 disjunction (fr OR
// en) cannot catch — a command documented in one README only slips through V107
// but fails here, naming the symmetric difference.
func TestReadmeCommandParity(t *testing.T) {
	fr := readDoc(t, "../../README.fr.md")
	en := readDoc(t, "../../README.md")

	var frOnly, enOnly []string
	for _, cmd := range specSurfaceCommands() {
		inFR := cmdReferenced(fr, cmd)
		inEN := cmdReferenced(en, cmd)
		switch {
		case inFR && !inEN:
			frOnly = append(frOnly, cmd)
		case inEN && !inFR:
			enOnly = append(enOnly, cmd)
		}
	}
	if len(frOnly) > 0 || len(enOnly) > 0 {
		t.Fatalf("README EN/FR command parity broken (V109): FR-only=%v EN-only=%v", frOnly, enOnly)
	}
}
