package sdd

import (
	"bytes"
	"testing"
)

// ordArg accepts both the bare number and the prefixed form.
func TestOrdArgAcceptsBareAndPrefixed(t *testing.T) {
	cases := []struct {
		in, prefix string
		want       int
		ok         bool
	}{
		{"98", "T", 98, true},
		{"T98", "T", 98, true},
		{"3", "U", 3, true},
		{"U3", "U", 3, true},
		{"17", "F", 17, true},
		{"F17", "F", 17, true},
		{"", "T", 0, false},
		{"Tx", "T", 0, false},
	}
	for _, c := range cases {
		got, err := ordArg(c.in, c.prefix)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("ordArg(%q,%q) = (%d,%v), want (%d,nil)", c.in, c.prefix, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("ordArg(%q,%q) accepted a bad input", c.in, c.prefix)
		}
	}
}

// set-task and resolve-unknown accept both "3" and the prefixed "T3"/"U3" via
// the root command (harmonized with rm-task/retract-* through ordArg).
func TestSetTaskAndResolveUnknownAcceptPrefix(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := gitRepo(t)
	t.Chdir(dir)

	run := func(args ...string) error {
		root := newRootCmd()
		root.SetArgs(args)
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		return root.Execute()
	}

	if err := run("init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	run("new-feature", "f")
	run("add-task", "t1", "--feature", "1")
	run("add-task", "t2", "--feature", "1")
	run("add-unknown", "q1", "--feature", "1")
	run("add-unknown", "q2", "--feature", "1")

	// prefixed form
	if err := run("set-task", "T1", "--status", "x"); err != nil {
		t.Errorf("set-task T1: %v", err)
	}
	if err := run("resolve-unknown", "U1"); err != nil {
		t.Errorf("resolve-unknown U1: %v", err)
	}
	// bare form still works
	if err := run("set-task", "2", "--status", "x"); err != nil {
		t.Errorf("set-task 2: %v", err)
	}
	if err := run("resolve-unknown", "2"); err != nil {
		t.Errorf("resolve-unknown 2: %v", err)
	}
}
