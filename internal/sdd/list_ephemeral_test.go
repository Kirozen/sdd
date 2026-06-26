package sdd

import (
	"fmt"
	"reflect"
	"testing"
)

// T114 / V102: list goal renders `F<ord> <n> | text`, with n 1-based per feature
// (resets at each feature boundary, ORDER BY id within a feature).
func TestListGoalByPosition(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fa, _ := addFeature(db, pid, "fa")
	fpkA, _ := featurePK(db, pid, fa)
	addGoal(db, fpkA, "a1")
	addGoal(db, fpkA, "a2")
	fb, _ := addFeature(db, pid, "fb")
	fpkB, _ := featurePK(db, pid, fb)
	addGoal(db, fpkB, "b1")

	got, err := listKind(db, pid, "goal")
	if err != nil {
		t.Fatalf("listKind goal: %v", err)
	}
	want := []string{
		fmt.Sprintf("F%d 1 | a1", fa),
		fmt.Sprintf("F%d 2 | a2", fa),
		fmt.Sprintf("F%d 1 | b1", fb),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("list goal = %v, want %v", got, want)
	}
}

// T114 / V102: list constraint mirrors list goal.
func TestListConstraintByPosition(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	fo, _ := addFeature(db, pid, "f")
	fpk, _ := featurePK(db, pid, fo)
	addConstraint(db, fpk, "c1")
	addConstraint(db, fpk, "c2")

	got, err := listKind(db, pid, "constraint")
	if err != nil {
		t.Fatalf("listKind constraint: %v", err)
	}
	want := []string{
		fmt.Sprintf("F%d 1 | c1", fo),
		fmt.Sprintf("F%d 2 | c2", fo),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("list constraint = %v, want %v", got, want)
	}
}

// T114 / V17: a valid-but-empty kind returns no lines and no error.
func TestListGoalEmpty(t *testing.T) {
	db := openTestDB(t)
	pid := mustProject(t, db)
	addFeature(db, pid, "f") // feature with no goals

	got, err := listKind(db, pid, "goal")
	if err != nil {
		t.Fatalf("listKind goal: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("list goal of an empty project = %v, want []", got)
	}
}

// T114 / V20: list goal/constraint is scoped — project B's rows are invisible to A.
func TestListGoalConstraintScoped(t *testing.T) {
	db := openTestDB(t)
	a := mustProject(t, db)
	b := mustProject(t, db)
	fb, _ := addFeature(db, b, "fb")
	fpkB, _ := featurePK(db, b, fb)
	addGoal(db, fpkB, "B's goal")
	addConstraint(db, fpkB, "B's constraint")

	for _, kind := range []string{"goal", "constraint"} {
		got, err := listKind(db, a, kind)
		if err != nil {
			t.Fatalf("listKind %s: %v", kind, err)
		}
		if len(got) != 0 {
			t.Errorf("list %s from A leaked B's rows: %v", kind, got)
		}
	}
}
