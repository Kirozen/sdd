package sdd

import (
	"context"
	"testing"

	dbq "github.com/kirozen/sdd/internal/db"
)

// T109 / V100: edit goal addresses the n-th goal (1-based, ORDER BY id) by
// position and leaves the others untouched.
func TestEditGoalByPosition(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	ford, _ := addFeature(db, pid, "f")
	fpk, _ := featurePK(db, pid, ford)
	for _, g := range []string{"g1", "g2", "g3"} {
		if err := addGoal(db, fpk, g); err != nil {
			t.Fatalf("addGoal: %v", err)
		}
	}

	if err := editGoalByPos(db, pid, ford, 2, "g2-edited"); err != nil {
		t.Fatalf("editGoalByPos: %v", err)
	}
	got, _ := dbq.New(db).GoalsByFeature(context.Background(), fpk)
	want := []string{"g1", "g2-edited", "g3"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("goal[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// T109 / V100: edit constraint mirrors edit goal.
func TestEditConstraintByPosition(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	ford, _ := addFeature(db, pid, "f")
	fpk, _ := featurePK(db, pid, ford)
	for _, c := range []string{"c1", "c2"} {
		if err := addConstraint(db, fpk, c); err != nil {
			t.Fatalf("addConstraint: %v", err)
		}
	}

	if err := editConstraintByPos(db, pid, ford, 1, "c1-edited"); err != nil {
		t.Fatalf("editConstraintByPos: %v", err)
	}
	got, _ := dbq.New(db).ConstraintsByFeature(context.Background(), fpk)
	if got[0] != "c1-edited" || got[1] != "c2" {
		t.Errorf("constraints = %v, want [c1-edited c2]", got)
	}
}

// T109 / V17: an out-of-range position errors and changes nothing.
func TestEditGoalOutOfRange(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	ford, _ := addFeature(db, pid, "f")
	fpk, _ := featurePK(db, pid, ford)
	addGoal(db, fpk, "only")

	if err := editGoalByPos(db, pid, ford, 5, "nope"); err == nil {
		t.Error("edit goal #5 of a 1-goal feature succeeded")
	}
	got, _ := dbq.New(db).GoalsByFeature(context.Background(), fpk)
	if len(got) != 1 || got[0] != "only" {
		t.Errorf("goal mutated despite out-of-range position: %v", got)
	}
}

// T109 / V20: the B6 regression. A feature ordinal from project B is invisible to
// A — edit goal AND edit constraint from A can never touch B's rows.
func TestEditGoalConstraintScoped(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)
	fordB, _ := addFeature(db, b, "fb")
	fpkB, _ := featurePK(db, b, fordB)
	addGoal(db, fpkB, "B's goal")
	addConstraint(db, fpkB, "B's constraint")

	if err := editGoalByPos(db, a, fordB, 1, "hijack"); err == nil {
		t.Error("edit goal reached across projects (V20/B6)")
	}
	if err := editConstraintByPos(db, a, fordB, 1, "hijack"); err == nil {
		t.Error("edit constraint reached across projects (V20/B6)")
	}
	gs, _ := dbq.New(db).GoalsByFeature(context.Background(), fpkB)
	cs, _ := dbq.New(db).ConstraintsByFeature(context.Background(), fpkB)
	if len(gs) != 1 || gs[0] != "B's goal" {
		t.Errorf("B's goal mutated from A's scope: %v", gs)
	}
	if len(cs) != 1 || cs[0] != "B's constraint" {
		t.Errorf("B's constraint mutated from A's scope: %v", cs)
	}
}
