package store

import (
	"testing"

	"github.com/RamXX/nd/internal/model"
)

func makeIssue(id string, priority int, blocks []string) *model.Issue {
	return &model.Issue{
		ID:       id,
		Title:    id,
		Priority: model.Priority(priority),
		Blocks:   blocks,
	}
}

func issueIDs(issues []*model.Issue) []string {
	ids := make([]string, len(issues))
	for i, issue := range issues {
		ids[i] = issue.ID
	}
	return ids
}

// TestTopoSortIssues_Linear: A blocks B blocks C -> order A,B,C
func TestTopoSortIssues_Linear(t *testing.T) {
	a := makeIssue("A", 2, []string{"B"})
	b := makeIssue("B", 2, []string{"C"})
	c := makeIssue("C", 2, nil)

	// Feed in reverse order to prove sorting, not input order.
	result := TopoSortIssues([]*model.Issue{c, b, a})
	ids := issueIDs(result)

	if len(ids) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(ids))
	}
	if ids[0] != "A" || ids[1] != "B" || ids[2] != "C" {
		t.Errorf("expected [A B C], got %v", ids)
	}
}

// TestTopoSortIssues_Diamond: A blocks B,C; B,C both block D -> A first, then B,C by priority, then D
func TestTopoSortIssues_Diamond(t *testing.T) {
	a := makeIssue("A", 2, []string{"B", "C"})
	b := makeIssue("B", 1, []string{"D"}) // higher priority (lower number)
	c := makeIssue("C", 3, []string{"D"}) // lower priority
	d := makeIssue("D", 2, nil)

	result := TopoSortIssues([]*model.Issue{d, c, b, a})
	ids := issueIDs(result)

	if ids[0] != "A" {
		t.Errorf("A should be first, got %v", ids)
	}
	if ids[1] != "B" {
		t.Errorf("B (P1) should come before C (P3), got %v", ids)
	}
	if ids[2] != "C" {
		t.Errorf("C should be third, got %v", ids)
	}
	if ids[3] != "D" {
		t.Errorf("D should be last, got %v", ids)
	}
}

// TestTopoSortIssues_NoDeps: no edges -> sorted by priority
func TestTopoSortIssues_NoDeps(t *testing.T) {
	a := makeIssue("A", 3, nil)
	b := makeIssue("B", 1, nil)
	c := makeIssue("C", 2, nil)

	result := TopoSortIssues([]*model.Issue{a, b, c})
	ids := issueIDs(result)

	if ids[0] != "B" || ids[1] != "C" || ids[2] != "A" {
		t.Errorf("expected [B C A] (by priority), got %v", ids)
	}
}

// TestTopoSortIssues_Cycle: A blocks B, B blocks A -> both appended, no panic
func TestTopoSortIssues_Cycle(t *testing.T) {
	a := makeIssue("A", 1, []string{"B"})
	b := makeIssue("B", 2, []string{"A"})

	result := TopoSortIssues([]*model.Issue{a, b})
	ids := issueIDs(result)

	if len(ids) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(ids))
	}
	// Both should be present (appended as cycle residue sorted by priority).
	if ids[0] != "A" || ids[1] != "B" {
		t.Errorf("expected [A B] (by priority), got %v", ids)
	}
}

// TestTopoSortIssues_MixedLocalExternal: A blocks Z (not in slice) -> edge ignored
func TestTopoSortIssues_MixedLocalExternal(t *testing.T) {
	a := makeIssue("A", 2, []string{"Z"}) // Z not in the slice
	b := makeIssue("B", 1, nil)

	result := TopoSortIssues([]*model.Issue{a, b})
	ids := issueIDs(result)

	// B has higher priority (1 < 2), so B first.
	if ids[0] != "B" || ids[1] != "A" {
		t.Errorf("expected [B A] (Z ignored, sorted by priority), got %v", ids)
	}
}

// TestTopoSortIssues_Reverse: linear chain, reverse=true via SortIssues -> reversed
func TestTopoSortIssues_Reverse(t *testing.T) {
	a := makeIssue("A", 2, []string{"B"})
	b := makeIssue("B", 2, []string{"C"})
	c := makeIssue("C", 2, nil)

	issues := []*model.Issue{c, b, a}
	SortIssues(issues, "deps", true)
	ids := issueIDs(issues)

	if ids[0] != "C" || ids[1] != "B" || ids[2] != "A" {
		t.Errorf("expected [C B A] (reversed topo order), got %v", ids)
	}
}

// TestSortIssues_Deps: end-to-end SortIssues("deps") in-place correctness
func TestSortIssues_Deps(t *testing.T) {
	a := makeIssue("A", 2, []string{"B"})
	b := makeIssue("B", 2, []string{"C"})
	c := makeIssue("C", 2, nil)

	issues := []*model.Issue{c, a, b}
	SortIssues(issues, "deps", false)
	ids := issueIDs(issues)

	if ids[0] != "A" || ids[1] != "B" || ids[2] != "C" {
		t.Errorf("expected [A B C], got %v", ids)
	}
}

// TestSortIssues_PriorityUnchanged: verify --sort=priority still works correctly
func TestSortIssues_PriorityUnchanged(t *testing.T) {
	a := makeIssue("A", 3, nil)
	b := makeIssue("B", 1, nil)
	c := makeIssue("C", 2, nil)

	issues := []*model.Issue{a, b, c}
	SortIssues(issues, "priority", false)
	ids := issueIDs(issues)

	if ids[0] != "B" || ids[1] != "C" || ids[2] != "A" {
		t.Errorf("expected [B C A] (by priority), got %v", ids)
	}
}

// TestTopoSortIssues_Empty: empty input returns empty.
func TestTopoSortIssues_Empty(t *testing.T) {
	result := TopoSortIssues(nil)
	if len(result) != 0 {
		t.Errorf("expected empty, got %d", len(result))
	}

	result2 := TopoSortIssues([]*model.Issue{})
	if len(result2) != 0 {
		t.Errorf("expected empty, got %d", len(result2))
	}
}

// TestTopoSortIssues_CycleWithNonCycle: mix of cycle and non-cycle nodes
func TestTopoSortIssues_CycleWithNonCycle(t *testing.T) {
	// A blocks B, B blocks A (cycle). C is independent.
	a := makeIssue("A", 1, []string{"B"})
	b := makeIssue("B", 2, []string{"A"})
	c := makeIssue("C", 0, nil) // highest priority, no deps

	result := TopoSortIssues([]*model.Issue{b, a, c})
	ids := issueIDs(result)

	// C should come first (no deps, highest priority).
	if ids[0] != "C" {
		t.Errorf("C should be first (no deps), got %v", ids)
	}
	// A and B are in a cycle, appended by priority.
	if ids[1] != "A" || ids[2] != "B" {
		t.Errorf("cycle nodes should be [A B] by priority, got %v", ids)
	}
}
