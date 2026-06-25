package sdd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rootRun executes one root command in dir, feeding stdin, returning its error.
func rootRun(t *testing.T, dir, stdin string, args ...string) error {
	t.Helper()
	t.Chdir(dir)
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetIn(strings.NewReader(stdin))
	return root.Execute()
}

func rootMustRun(t *testing.T, dir, stdin string, args ...string) {
	t.Helper()
	if err := rootRun(t, dir, stdin, args...); err != nil {
		t.Fatalf("cmd %v: %v", args, err)
	}
}

func tab(fields ...string) string { return strings.Join(fields, "\t") }

// V61/V62/V63/V64/V67: a batch applied in one shot yields a byte-identical
// SPEC.md to replaying the same commands unitarily — same cores, same ordinals
// (the task cites V1, created earlier in the same tx), apostrophe intact, the
// implicit feature matching an explicit --feature 1.
func TestApplyByteIdenticalToUnitary(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repoA := gitRepo(t)
	repoB := gitRepo(t)

	batch := strings.Join([]string{
		tab("new-feature", "batch-f"),
		tab("add-invariant", "inv one"),
		tab("add-goal", "goal with l'apostrophe et   espaces"),
		tab("add-constraint", "c one"),
		tab("add-task", "--cites", "V1", "task citing V1"),
	}, "\n") + "\n"

	rootMustRun(t, repoA, "", "init")
	rootMustRun(t, repoA, batch, "apply")

	rootMustRun(t, repoB, "", "init")
	rootMustRun(t, repoB, "", "new-feature", "batch-f")
	rootMustRun(t, repoB, "", "add-invariant", "inv one")
	rootMustRun(t, repoB, "", "add-goal", "goal with l'apostrophe et   espaces", "--feature", "1")
	rootMustRun(t, repoB, "", "add-constraint", "c one", "--feature", "1")
	rootMustRun(t, repoB, "", "add-task", "task citing V1", "--feature", "1", "--cites", "V1")

	a, err := os.ReadFile(filepath.Join(repoA, "SPEC.md"))
	if err != nil {
		t.Fatalf("read A: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(repoB, "SPEC.md"))
	if err != nil {
		t.Fatalf("read B: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("apply != unitary replay\n--- apply ---\n%s\n--- unitary ---\n%s", a, b)
	}
}

// V65/V66/V72: a bad line rolls the whole batch back, names the physical 1-based
// line (counting the skipped blank), and leaves SPEC.md + db untouched.
func TestApplyRollsBackOnBadLine(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repo := gitRepo(t)
	rootMustRun(t, repo, "", "init")
	before, _ := os.ReadFile(filepath.Join(repo, "SPEC.md"))

	batch := strings.Join([]string{
		tab("new-feature", "f"), // line 1
		"",                      // line 2 (blank, skipped)
		tab("add-goal", "g"),    // line 3
		tab("add-task", "--cites", "V99", "bad cite"), // line 4: orphan
	}, "\n") + "\n"

	err := rootRun(t, repo, batch, "apply")
	if err == nil {
		t.Fatal("apply accepted a batch with an orphan cite")
	}
	if !strings.Contains(err.Error(), "line 4") {
		t.Errorf("error must name the failing physical line, got: %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(repo, "SPEC.md"))
	if !bytes.Equal(before, after) {
		t.Error("failed batch re-exported SPEC.md (V66: no partial export)")
	}
	// db must be unchanged: check passes only if db still matches the frozen file.
	rootMustRun(t, repo, "", "check")
}

// V72: blank lines are skipped, not dispatched as an empty verb.
func TestApplySkipsBlankLines(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repo := gitRepo(t)
	rootMustRun(t, repo, "", "init")

	batch := "\n" + tab("new-feature", "f") + "\n\n\n" + tab("add-goal", "g") + "\n\n"
	rootMustRun(t, repo, batch, "apply")
	rootMustRun(t, repo, "", "check")

	rootMustRun(t, repo, "", "export")
	spec, _ := os.ReadFile(filepath.Join(repo, "SPEC.md"))
	if !strings.Contains(string(spec), "FEATURE 1: f") {
		t.Errorf("blank-line batch did not apply the real lines:\n%s", spec)
	}
}

// boundary: only the create-only add-* family is accepted.
func TestApplyRejectsOutOfScopeVerb(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repo := gitRepo(t)
	rootMustRun(t, repo, "", "init")

	err := rootRun(t, repo, tab("set-task", "1", "--status", "x")+"\n", "apply")
	if err == nil || !strings.Contains(err.Error(), "not an apply verb") {
		t.Errorf("apply should reject a non-add-* verb, got: %v", err)
	}
}

// V64: a feature-bearing verb with no new-feature before it and no --feature is
// an error, never a silent attach.
func TestApplyImplicitFeatureRequired(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repo := gitRepo(t)
	rootMustRun(t, repo, "", "init")

	err := rootRun(t, repo, tab("add-goal", "orphan goal")+"\n", "apply")
	if err == nil || !strings.Contains(err.Error(), "needs a feature") {
		t.Errorf("add-goal without a feature should error, got: %v", err)
	}
}
