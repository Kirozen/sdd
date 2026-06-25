package sdd

import (
	"context"
	"testing"

	dbq "github.com/kirozen/sdd/internal/db"
)

// T106 / V98: rm-goal deletes the n-th goal (1-based, ORDER BY id) and leaves the
// others in order.
func TestRmGoalByPosition(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	ford, _ := addFeature(db, pid, "f")
	fpk, _ := featurePK(db, pid, ford)
	for _, g := range []string{"g1", "g2", "g3"} {
		if err := addGoal(db, fpk, g); err != nil {
			t.Fatalf("addGoal: %v", err)
		}
	}

	// remove the 2nd goal
	if err := rmGoal(db, pid, ford, 2); err != nil {
		t.Fatalf("rmGoal: %v", err)
	}
	got, _ := dbq.New(db).GoalsByFeature(context.Background(), fpk)
	want := []string{"g1", "g3"}
	if len(got) != len(want) {
		t.Fatalf("goals = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("goal[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// T106 / V17: an out-of-range position errors and changes nothing.
func TestRmGoalOutOfRange(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	ford, _ := addFeature(db, pid, "f")
	fpk, _ := featurePK(db, pid, ford)
	addGoal(db, fpk, "only")

	if err := rmGoal(db, pid, ford, 5); err == nil {
		t.Error("rm-goal #5 of a 1-goal feature succeeded")
	}
	got, _ := dbq.New(db).GoalsByFeature(context.Background(), fpk)
	if len(got) != 1 {
		t.Errorf("goal removed despite out-of-range position")
	}
}

// T106 / V20: a feature ordinal from project B is invisible to A.
func TestRmGoalScoped(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)
	fordB, _ := addFeature(db, b, "fb")
	fpkB, _ := featurePK(db, b, fordB)
	addGoal(db, fpkB, "B's goal")

	if err := rmGoal(db, a, fordB, 1); err == nil {
		t.Error("rm-goal reached across projects (V20)")
	}
	got, _ := dbq.New(db).GoalsByFeature(context.Background(), fpkB)
	if len(got) != 1 {
		t.Errorf("B's goal deleted from A's scope")
	}
}

// T106 / V98: rm-constraint mirrors rm-goal — deletes the n-th constraint.
func TestRmConstraintByPosition(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	ford, _ := addFeature(db, pid, "f")
	fpk, _ := featurePK(db, pid, ford)
	for _, c := range []string{"c1", "c2"} {
		if err := addConstraint(db, fpk, c); err != nil {
			t.Fatalf("addConstraint: %v", err)
		}
	}

	if err := rmConstraint(db, pid, ford, 1); err != nil {
		t.Fatalf("rmConstraint: %v", err)
	}
	got, _ := dbq.New(db).ConstraintsByFeature(context.Background(), fpk)
	if len(got) != 1 || got[0] != "c2" {
		t.Errorf("constraints = %v, want [c2]", got)
	}
}
