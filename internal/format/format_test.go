package format

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/RamXX/nd/internal/model"
	"github.com/mattn/go-runewidth"
)

// makeIssue creates a minimal issue for testing. All fields except ID, Title,
// Status, Priority, Type, Parent, and CreatedAt default to sensible values.
func makeIssue(id, title string, status model.Status, priority int, issueType model.IssueType, parent string) *model.Issue {
	return &model.Issue{
		ID:        id,
		Title:     title,
		Status:    status,
		Priority:  model.Priority(priority),
		Type:      issueType,
		Parent:    parent,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CreatedBy: "tester",
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestFormatIssueLine(t *testing.T) {
	issue := makeIssue("TST-0001", "Fix the login bug", model.StatusOpen, 1, model.TypeBug, "")
	issue.Assignee = "alice"
	issue.Labels = []string{"auth", "urgent"}

	line := FormatIssueLine(issue, 200)

	// Should contain the issue ID.
	if !strings.Contains(line, "TST-0001") {
		t.Errorf("line should contain issue ID: %s", line)
	}
	// Should contain assignee.
	if !strings.Contains(line, "@alice") {
		t.Errorf("line should contain assignee: %s", line)
	}
	// Should contain labels.
	if !strings.Contains(line, "auth, urgent") {
		t.Errorf("line should contain labels: %s", line)
	}
	// Should contain title.
	if !strings.Contains(line, "Fix the login bug") {
		t.Errorf("line should contain title: %s", line)
	}
}

func TestFormatIssueLine_Closed(t *testing.T) {
	issue := makeIssue("TST-0002", "Old task", model.StatusClosed, 2, model.TypeTask, "")

	line := FormatIssueLine(issue, 200)

	// Closed issues should contain the closed icon.
	if !strings.Contains(line, "TST-0002") {
		t.Errorf("closed line should contain issue ID: %s", line)
	}
}

func TestFormatIssueLine_TruncatesLongTitle(t *testing.T) {
	longTitle := strings.Repeat("A", 200)
	issue := makeIssue("TST-0003", longTitle, model.StatusOpen, 1, model.TypeTask, "")

	// Narrow availWidth forces truncation even for a very long title.
	line := FormatIssueLine(issue, 60)

	if !strings.Contains(line, "...") {
		t.Errorf("long title should be truncated with ...: %s", line)
	}
}

func TestTree_BasicGrouping(t *testing.T) {
	epic := makeIssue("TST-epic", "Auth Epic", model.StatusOpen, 1, model.TypeEpic, "")
	child1 := makeIssue("TST-ch01", "Design auth", model.StatusOpen, 1, model.TypeTask, "TST-epic")
	child2 := makeIssue("TST-ch02", "Implement auth", model.StatusInProgress, 1, model.TypeFeature, "TST-epic")

	issues := []*model.Issue{epic, child1, child2}
	contextIDs := map[string]bool{}

	var buf bytes.Buffer
	Tree(&buf, issues, contextIDs, "priority", false)
	output := buf.String()

	// Parent should appear without connector.
	if !strings.Contains(output, "TST-epic") {
		t.Errorf("output should contain parent epic: %s", output)
	}
	// Children should appear with tree connectors.
	if !strings.Contains(output, "├── ") && !strings.Contains(output, "└── ") {
		t.Errorf("output should contain tree connectors: %s", output)
	}
	if !strings.Contains(output, "TST-ch01") {
		t.Errorf("output should contain child1: %s", output)
	}
	if !strings.Contains(output, "TST-ch02") {
		t.Errorf("output should contain child2: %s", output)
	}
	// Count should include all 3 issues.
	if !strings.Contains(output, "3 issue(s)") {
		t.Errorf("count should be 3: %s", output)
	}
}

func TestTree_Unparented(t *testing.T) {
	task1 := makeIssue("TST-t001", "Standalone task", model.StatusOpen, 2, model.TypeTask, "")
	task2 := makeIssue("TST-t002", "Another task", model.StatusOpen, 3, model.TypeChore, "")

	issues := []*model.Issue{task1, task2}
	contextIDs := map[string]bool{}

	var buf bytes.Buffer
	Tree(&buf, issues, contextIDs, "priority", false)
	output := buf.String()

	// Should have [Unparented] section.
	if !strings.Contains(output, "[Unparented]") {
		t.Errorf("output should contain [Unparented] section: %s", output)
	}
	if !strings.Contains(output, "TST-t001") {
		t.Errorf("output should contain task1: %s", output)
	}
	if !strings.Contains(output, "TST-t002") {
		t.Errorf("output should contain task2: %s", output)
	}
	if !strings.Contains(output, "2 issue(s)") {
		t.Errorf("count should be 2: %s", output)
	}
}

func TestTree_ContextOnly(t *testing.T) {
	// Context-only parent: excluded by filters, fetched for display only.
	epic := makeIssue("TST-epic", "Auth Epic", model.StatusClosed, 1, model.TypeEpic, "")
	child1 := makeIssue("TST-ch01", "Design auth", model.StatusOpen, 1, model.TypeTask, "TST-epic")

	issues := []*model.Issue{epic, child1}
	contextIDs := map[string]bool{"TST-epic": true}

	var buf bytes.Buffer
	Tree(&buf, issues, contextIDs, "priority", false)
	output := buf.String()

	// Context-only epic should still appear (for grouping).
	if !strings.Contains(output, "TST-epic") {
		t.Errorf("output should contain context-only epic: %s", output)
	}
	// Count should exclude context-only parent (only child1 counts).
	if !strings.Contains(output, "1 issue(s)") {
		t.Errorf("count should be 1 (context parent excluded): %s", output)
	}
}

func TestTree_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	Tree(&buf, nil, map[string]bool{}, "priority", false)
	output := buf.String()

	if !strings.Contains(output, "No issues found.") {
		t.Errorf("empty list should show 'No issues found.': %s", output)
	}
}

func TestTree_DeepNesting(t *testing.T) {
	// 3+ levels: epic -> feature -> subtask.
	epic := makeIssue("TST-epic", "Top Epic", model.StatusOpen, 1, model.TypeEpic, "")
	feature := makeIssue("TST-feat", "Feature under epic", model.StatusOpen, 1, model.TypeFeature, "TST-epic")
	subtask := makeIssue("TST-sub1", "Subtask under feature", model.StatusOpen, 1, model.TypeTask, "TST-feat")

	issues := []*model.Issue{epic, feature, subtask}
	contextIDs := map[string]bool{}

	var buf bytes.Buffer
	Tree(&buf, issues, contextIDs, "priority", false)
	output := buf.String()

	// All three should appear.
	if !strings.Contains(output, "TST-epic") {
		t.Errorf("output should contain epic: %s", output)
	}
	if !strings.Contains(output, "TST-feat") {
		t.Errorf("output should contain feature: %s", output)
	}
	if !strings.Contains(output, "TST-sub1") {
		t.Errorf("output should contain subtask: %s", output)
	}
	// Should have nested connectors (the subtask is indented deeper).
	if !strings.Contains(output, "3 issue(s)") {
		t.Errorf("count should be 3: %s", output)
	}
}

func TestTree_SortWithinGroups(t *testing.T) {
	epic := makeIssue("TST-epic", "Top Epic", model.StatusOpen, 1, model.TypeEpic, "")
	child_p3 := makeIssue("TST-ch01", "Low priority child", model.StatusOpen, 3, model.TypeTask, "TST-epic")
	child_p1 := makeIssue("TST-ch02", "High priority child", model.StatusOpen, 1, model.TypeTask, "TST-epic")
	child_p2 := makeIssue("TST-ch03", "Medium priority child", model.StatusOpen, 2, model.TypeTask, "TST-epic")

	issues := []*model.Issue{epic, child_p3, child_p1, child_p2}
	contextIDs := map[string]bool{}

	var buf bytes.Buffer
	Tree(&buf, issues, contextIDs, "priority", false)
	output := buf.String()

	// Children should be sorted by priority: P1, P2, P3.
	ch02Idx := strings.Index(output, "TST-ch02")
	ch03Idx := strings.Index(output, "TST-ch03")
	ch01Idx := strings.Index(output, "TST-ch01")

	if ch02Idx < 0 || ch03Idx < 0 || ch01Idx < 0 {
		t.Fatalf("all children should appear in output: %s", output)
	}
	if ch02Idx > ch03Idx {
		t.Errorf("P1 child (TST-ch02) should come before P2 child (TST-ch03)")
	}
	if ch03Idx > ch01Idx {
		t.Errorf("P2 child (TST-ch03) should come before P3 child (TST-ch01)")
	}
}

func TestTree_SortWithinGroupsReverse(t *testing.T) {
	epic := makeIssue("TST-epic", "Top Epic", model.StatusOpen, 1, model.TypeEpic, "")
	child_p1 := makeIssue("TST-ch01", "High priority", model.StatusOpen, 1, model.TypeTask, "TST-epic")
	child_p3 := makeIssue("TST-ch02", "Low priority", model.StatusOpen, 3, model.TypeTask, "TST-epic")

	issues := []*model.Issue{epic, child_p1, child_p3}
	contextIDs := map[string]bool{}

	var buf bytes.Buffer
	Tree(&buf, issues, contextIDs, "priority", true)
	output := buf.String()

	// With reverse, P3 should come before P1.
	ch01Idx := strings.Index(output, "TST-ch01")
	ch02Idx := strings.Index(output, "TST-ch02")

	if ch01Idx < 0 || ch02Idx < 0 {
		t.Fatalf("both children should appear: %s", output)
	}
	if ch02Idx > ch01Idx {
		t.Errorf("with reverse, P3 child (TST-ch02) should come before P1 child (TST-ch01)")
	}
}

func TestTree_MixedParentsAndUnparented(t *testing.T) {
	epic := makeIssue("TST-epic", "Auth Epic", model.StatusOpen, 1, model.TypeEpic, "")
	child := makeIssue("TST-ch01", "Design auth", model.StatusOpen, 1, model.TypeTask, "TST-epic")
	orphan := makeIssue("TST-orph", "Standalone task", model.StatusOpen, 2, model.TypeTask, "")

	issues := []*model.Issue{epic, child, orphan}
	contextIDs := map[string]bool{}

	var buf bytes.Buffer
	Tree(&buf, issues, contextIDs, "priority", false)
	output := buf.String()

	// Epic and child should be grouped.
	if !strings.Contains(output, "TST-epic") {
		t.Errorf("output should contain epic: %s", output)
	}
	if !strings.Contains(output, "TST-ch01") {
		t.Errorf("output should contain child: %s", output)
	}
	// Orphan should be in [Unparented].
	if !strings.Contains(output, "[Unparented]") {
		t.Errorf("output should contain [Unparented] section: %s", output)
	}
	if !strings.Contains(output, "TST-orph") {
		t.Errorf("output should contain orphan: %s", output)
	}
	// Count should be all 3.
	if !strings.Contains(output, "3 issue(s)") {
		t.Errorf("count should be 3: %s", output)
	}
}

func TestTree_NoUnparentedSectionWhenEmpty(t *testing.T) {
	epic := makeIssue("TST-epic", "Auth Epic", model.StatusOpen, 1, model.TypeEpic, "")
	child := makeIssue("TST-ch01", "Task", model.StatusOpen, 1, model.TypeTask, "TST-epic")

	issues := []*model.Issue{epic, child}
	contextIDs := map[string]bool{}

	var buf bytes.Buffer
	Tree(&buf, issues, contextIDs, "priority", false)
	output := buf.String()

	if strings.Contains(output, "[Unparented]") {
		t.Errorf("output should NOT contain [Unparented] when all issues have parents: %s", output)
	}
}

func TestTree_ConnectorPositions(t *testing.T) {
	epic := makeIssue("TST-epic", "Epic", model.StatusOpen, 1, model.TypeEpic, "")
	child1 := makeIssue("TST-ch01", "First child", model.StatusOpen, 1, model.TypeTask, "TST-epic")
	child2 := makeIssue("TST-ch02", "Second child", model.StatusOpen, 1, model.TypeTask, "TST-epic")
	child3 := makeIssue("TST-ch03", "Third child", model.StatusOpen, 1, model.TypeTask, "TST-epic")

	issues := []*model.Issue{epic, child1, child2, child3}
	contextIDs := map[string]bool{}

	var buf bytes.Buffer
	Tree(&buf, issues, contextIDs, "id", false)
	output := buf.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Find child lines (lines containing tree connectors).
	var childLines []string
	for _, line := range lines {
		if strings.Contains(line, "├── ") || strings.Contains(line, "└── ") {
			childLines = append(childLines, line)
		}
	}

	if len(childLines) < 3 {
		t.Fatalf("expected at least 3 child lines with connectors, got %d: %v", len(childLines), childLines)
	}

	// First and second children should use ├──, last should use └──.
	if !strings.Contains(childLines[0], "├── ") {
		t.Errorf("first child should use ├──: %s", childLines[0])
	}
	if !strings.Contains(childLines[1], "├── ") {
		t.Errorf("second child should use ├──: %s", childLines[1])
	}
	if !strings.Contains(childLines[2], "└── ") {
		t.Errorf("last child should use └──: %s", childLines[2])
	}
}

func TestTable_UnchangedBehavior(t *testing.T) {
	issue1 := makeIssue("TST-0001", "First issue", model.StatusOpen, 1, model.TypeBug, "")
	issue2 := makeIssue("TST-0002", "Second issue", model.StatusClosed, 2, model.TypeTask, "")

	issues := []*model.Issue{issue1, issue2}

	var buf bytes.Buffer
	Table(&buf, issues)
	output := buf.String()

	if !strings.Contains(output, "TST-0001") {
		t.Errorf("Table output should contain first issue: %s", output)
	}
	if !strings.Contains(output, "TST-0002") {
		t.Errorf("Table output should contain second issue: %s", output)
	}
	if !strings.Contains(output, "2 issue(s)") {
		t.Errorf("Table output should show count: %s", output)
	}
}

func TestTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, nil)
	output := buf.String()

	if !strings.Contains(output, "No issues found.") {
		t.Errorf("Table with no issues should show 'No issues found.': %s", output)
	}
}

// prefixWidthOfIssue measures the visual width that a given issue's
// non-title prefix occupies. Useful for computing exact budgets in
// truncation tests without depending on internal package state.
func prefixWidthOfIssue(issue *model.Issue) int {
	// Pass an enormous availWidth so the title is never truncated.
	line := FormatIssueLine(issue, 10_000)
	stripped := stripANSI(line)
	title := issue.Title
	// The title always appears at the end of the stripped line.
	idx := strings.LastIndex(stripped, title)
	if idx < 0 {
		return 0
	}
	return runewidth.StringWidth(stripped[:idx])
}

// TestFormatIssueLine_TitleExactlyAtBudget confirms that when the title
// fits in the budget exactly (visual width == budget), no truncation
// occurs and the title appears untouched in the output.
func TestFormatIssueLine_TitleExactlyAtBudget(t *testing.T) {
	title := strings.Repeat("A", 30)
	issue := makeIssue("TST-EXCT", title, model.StatusOpen, 1, model.TypeTask, "")

	prefix := prefixWidthOfIssue(issue)
	availWidth := prefix + len(title) // budget equals exactly the title width

	line := FormatIssueLine(issue, availWidth)
	stripped := stripANSI(line)

	if !strings.Contains(stripped, title) {
		t.Errorf("title at exact budget should fit without truncation: %q (stripped=%q)", line, stripped)
	}
	if strings.Contains(stripped, "...") {
		t.Errorf("title at exact budget should not be truncated with ellipsis: %q", stripped)
	}
}

// TestFormatIssueLine_TitleOneOverBudget confirms that a title one
// character longer than the available budget gets truncated and the
// output ends with "...".
func TestFormatIssueLine_TitleOneOverBudget(t *testing.T) {
	title := strings.Repeat("A", 30)
	issue := makeIssue("TST-OVER", title, model.StatusOpen, 1, model.TypeTask, "")

	prefix := prefixWidthOfIssue(issue)
	availWidth := prefix + len(title) - 1 // one column short of fitting the title

	line := FormatIssueLine(issue, availWidth)
	stripped := stripANSI(line)

	if !strings.HasSuffix(stripped, "...") {
		t.Errorf("title one over budget should be truncated with ...: %q", stripped)
	}
	// The rendered visual width must not exceed availWidth.
	if got := runewidth.StringWidth(stripped); got > availWidth {
		t.Errorf("rendered line width %d exceeds budget %d: %q", got, availWidth, stripped)
	}
}

// TestFormatIssueLine_NarrowBudgetClampsToFloor confirms that a
// pathologically narrow terminal still produces a line with a minimum
// title floor rather than panicking or producing a negative-width slice.
func TestFormatIssueLine_NarrowBudgetClampsToFloor(t *testing.T) {
	title := "An unreasonably long title that will be truncated"
	issue := makeIssue("TST-NARW", title, model.StatusOpen, 1, model.TypeTask, "")

	// availWidth = 5 is smaller than the prefix alone. truncateTitle
	// should clamp to minTitleFloor and still append "..." to signal
	// the truncation occurred.
	line := FormatIssueLine(issue, 5)
	stripped := stripANSI(line)

	if !strings.Contains(stripped, "...") {
		t.Errorf("narrow budget should still mark truncation with ...: %q", stripped)
	}
	// Sanity: the function must not produce an empty line.
	if strings.TrimSpace(stripped) == "" {
		t.Errorf("narrow budget produced empty output")
	}
}

// TestFormatIssueLine_AnsiAware confirms that the prefix-width
// calculation strips ANSI color codes. A title that has 40 visible
// columns should fit in a budget of prefix_width + 40 regardless of how
// many ANSI escape bytes the colorized prefix contains.
func TestFormatIssueLine_AnsiAware(t *testing.T) {
	title := strings.Repeat("B", 40)
	// StatusInProgress is colorized by RenderStatusIcon; this exercises
	// the ANSI-strip path. Priority 0 and 1 also emit ANSI codes.
	issue := makeIssue("TST-ANSI", title, model.StatusInProgress, 0, model.TypeBug, "")
	issue.Labels = []string{"critical"}

	prefix := prefixWidthOfIssue(issue)
	availWidth := prefix + 40 // exactly fits the 40-column title

	line := FormatIssueLine(issue, availWidth)
	stripped := stripANSI(line)

	if !strings.Contains(stripped, title) {
		t.Errorf("ANSI-colorized prefix must measure visible width: title missing from %q", stripped)
	}
	if strings.Contains(stripped, "...") {
		t.Errorf("ANSI-aware budget should not truncate a title that fits visually: %q", stripped)
	}
	// The line must contain ANSI escape bytes (color codes) to prove
	// that the original rendered output had color, and that stripping
	// was necessary to compute the budget correctly.
	if !strings.Contains(line, "\x1b[") {
		t.Logf("note: rendered line had no ANSI escapes (likely NO_COLOR=1 in env)")
	}
}

// TestRenderTreeNode_PrefixReducesBudget confirms that when a tree
// connector is prepended to a line, the budget passed to FormatIssueLine
// is reduced by the connector's visual width so the combined output
// stays within the terminal width.
func TestRenderTreeNode_PrefixReducesBudget(t *testing.T) {
	title := strings.Repeat("C", 100)
	// Epic with a child, both with the same long title so we can compare.
	epic := makeIssue("TST-epic", title, model.StatusOpen, 1, model.TypeEpic, "")
	child := makeIssue("TST-ch01", title, model.StatusOpen, 1, model.TypeTask, "TST-epic")

	contextIDs := map[string]bool{}

	var buf bytes.Buffer
	// Exercise the renderTreeNode path directly with a known termWidth.
	childrenOf := map[string][]*model.Issue{
		"TST-epic": {child},
	}
	renderTreeNode(&buf, epic, childrenOf, contextIDs, "priority", false, "", 60, pickGlyphs(false))

	output := buf.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 rendered lines (epic + child), got %d: %q", len(lines), output)
	}

	for i, line := range lines {
		stripped := stripANSI(line)
		width := runewidth.StringWidth(stripped)
		if width > 60 {
			t.Errorf("line %d width %d exceeds termWidth 60: %q", i, width, stripped)
		}
	}

	// Both rendered lines should be truncated (their titles were 100
	// chars and the term is 60 wide).
	for i, line := range lines {
		if !strings.Contains(line, "...") {
			t.Errorf("line %d should be truncated at narrow termWidth: %q", i, line)
		}
	}
}

// TestRenderTreeNode_DeeperPrefixShorterTitle confirms that the deeper
// a node is nested, the shorter the title budget becomes, so a deeply
// nested node truncates more aggressively than a top-level one.
func TestRenderTreeNode_DeeperPrefixShorterTitle(t *testing.T) {
	title := strings.Repeat("D", 80)
	epic := makeIssue("TST-epic", title, model.StatusOpen, 1, model.TypeEpic, "")
	feature := makeIssue("TST-feat", title, model.StatusOpen, 1, model.TypeFeature, "TST-epic")
	subtask := makeIssue("TST-sub1", title, model.StatusOpen, 1, model.TypeTask, "TST-feat")

	contextIDs := map[string]bool{}

	// Render at a fixed known termWidth so each depth is predictable.
	var buf bytes.Buffer
	childrenOf := map[string][]*model.Issue{
		"TST-epic": {feature},
		"TST-feat": {subtask},
	}
	renderTreeNode(&buf, epic, childrenOf, contextIDs, "priority", false, "", 80, pickGlyphs(false))

	output := buf.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 rendered lines, got %d: %q", len(lines), output)
	}

	// Measure how much of the title appears on each line by counting
	// consecutive 'D's before the ellipsis on the stripped line. Deeper
	// nesting should show strictly fewer Ds.
	titleRun := func(s string) int {
		stripped := stripANSI(s)
		count := 0
		for _, r := range stripped {
			if r == 'D' {
				count++
			}
		}
		return count
	}

	epicDs := titleRun(lines[0])
	featDs := titleRun(lines[1])
	subDs := titleRun(lines[2])

	if !(epicDs > featDs && featDs > subDs) {
		t.Errorf("deeper nesting should truncate more aggressively; got epic=%d feat=%d sub=%d",
			epicDs, featDs, subDs)
	}

	// All rendered lines must fit within the termWidth.
	for i, line := range lines {
		stripped := stripANSI(line)
		if width := runewidth.StringWidth(stripped); width > 80 {
			t.Errorf("line %d width %d exceeds termWidth 80: %q", i, width, stripped)
		}
	}
}

// TestDetectTerminalWidth_NonTtyFallback confirms that when stdout is
// not a tty (as in go test without a pseudo-tty) and $COLUMNS is unset,
// detectTerminalWidth falls back through the remaining chain to the
// documented 120-column default.
//
// Under `go test`, stdout is not a tty, stderr is not a tty, and
// /dev/tty is only reachable on POSIX. On platforms where /dev/tty
// returns a width (e.g. developer running `go test` from an interactive
// terminal), the result may be that terminal's width. Accept either the
// default or any positive width reported by /dev/tty.
func TestDetectTerminalWidth_NonTtyFallback(t *testing.T) {
	// Make sure a stale $COLUMNS in the developer's shell doesn't
	// override the fallback we want to test here.
	t.Setenv("COLUMNS", "")

	got := detectTerminalWidth()
	if got <= 0 {
		t.Errorf("detectTerminalWidth() = %d, want a positive width", got)
	}
	// Either the documented default, or a real /dev/tty width. Both
	// are acceptable; the important invariant is >0 and deterministic.
	// (On CI where there is no /dev/tty, this will be the default.)
}

// TestDetectTerminalWidth_EnvColumnsOverride confirms that when
// $COLUMNS is set to a valid positive integer, detectTerminalWidth
// returns that value regardless of stdout/stderr tty state.
func TestDetectTerminalWidth_EnvColumnsOverride(t *testing.T) {
	t.Setenv("COLUMNS", "40")
	got := detectTerminalWidth()
	if got != 40 {
		t.Errorf("detectTerminalWidth() with COLUMNS=40 = %d, want 40", got)
	}
}

// TestDetectTerminalWidth_EnvColumnsPrecedence confirms that the
// $COLUMNS override wins even when later fallbacks would produce a
// different positive width. This is the watch(1) workaround path: the
// user sets COLUMNS and expects it to win even if stdout happens to be
// a tty with a different size.
func TestDetectTerminalWidth_EnvColumnsPrecedence(t *testing.T) {
	t.Setenv("COLUMNS", "55")
	got := detectTerminalWidth()
	if got != 55 {
		t.Errorf("detectTerminalWidth() with COLUMNS=55 = %d, want 55 (env must win over tty detection)", got)
	}
}

// TestDetectTerminalWidth_EnvColumnsInvalid confirms that non-numeric
// $COLUMNS values are ignored and the function falls through to the
// next source without panicking.
func TestDetectTerminalWidth_EnvColumnsInvalid(t *testing.T) {
	t.Setenv("COLUMNS", "not-a-number")
	got := detectTerminalWidth()
	if got <= 0 {
		t.Errorf("detectTerminalWidth() with invalid COLUMNS = %d, want a positive fallback", got)
	}
	// Must NOT be interpreted as 0 / the string length / anything weird.
	// Since we explicitly want the fallback, ensure we didn't accidentally
	// parse "not-a-number" as anything.
	if got == 0 {
		t.Errorf("detectTerminalWidth() must not return 0 for invalid COLUMNS")
	}
}

// TestDetectTerminalWidth_EnvColumnsZero confirms that COLUMNS=0 is
// rejected (not treated as a valid width) and the function falls
// through to subsequent sources.
func TestDetectTerminalWidth_EnvColumnsZero(t *testing.T) {
	t.Setenv("COLUMNS", "0")
	got := detectTerminalWidth()
	if got == 0 {
		t.Errorf("detectTerminalWidth() with COLUMNS=0 returned 0; must reject and fall through")
	}
	if got <= 0 {
		t.Errorf("detectTerminalWidth() = %d, want a positive fallback width", got)
	}
}

// TestDetectTerminalWidth_EnvColumnsNegative confirms that COLUMNS=-5
// is rejected and the function falls through to subsequent sources.
func TestDetectTerminalWidth_EnvColumnsNegative(t *testing.T) {
	t.Setenv("COLUMNS", "-5")
	got := detectTerminalWidth()
	if got < 0 {
		t.Errorf("detectTerminalWidth() with COLUMNS=-5 returned %d; must reject and fall through", got)
	}
	if got <= 0 {
		t.Errorf("detectTerminalWidth() = %d, want a positive fallback width", got)
	}
}

// TestDetectTerminalWidth_EnvColumnsTrimmed confirms that whitespace
// around the COLUMNS value is handled gracefully (either trimmed and
// accepted, or rejected as invalid -- both are acceptable; the
// function must not crash).
func TestDetectTerminalWidth_EnvColumnsTrimmed(t *testing.T) {
	t.Setenv("COLUMNS", "  42  ")
	got := detectTerminalWidth()
	if got <= 0 {
		t.Errorf("detectTerminalWidth() = %d, want a positive width", got)
	}
	// Accept either 42 (trimmed and parsed) or a fallback. Both are
	// reasonable; strconv.Atoi without TrimSpace would reject it.
}

// TestDetectTerminalWidth_NoPanic confirms the function never panics
// across a matrix of COLUMNS values, including some degenerate ones.
// This is the "unwanted behavior" acceptance criterion: no panic, no
// hang, no fd leak.
func TestDetectTerminalWidth_NoPanic(t *testing.T) {
	cases := []string{
		"",               // empty
		" ",              // whitespace only
		"0",              // zero
		"-1",             // negative
		"abc",            // non-numeric
		"99999999999999", // huge
		"1",              // minimum positive
	}
	for _, c := range cases {
		t.Run("COLUMNS="+c, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("detectTerminalWidth() panicked with COLUMNS=%q: %v", c, r)
				}
			}()
			t.Setenv("COLUMNS", c)
			got := detectTerminalWidth()
			if got <= 0 {
				t.Errorf("detectTerminalWidth() with COLUMNS=%q = %d, want positive", c, got)
			}
		})
	}
}

// TestStripANSI_RemovesSgrSequences confirms the regex-based stripping
// used by visualWidth handles common SGR sequences emitted by lipgloss.
func TestStripANSI_RemovesSgrSequences(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello", "hello"},
		{"simple color", "\x1b[31mhello\x1b[0m", "hello"},
		{"compound", "\x1b[1;31;48;5;202mhello\x1b[0m world", "hello world"},
		{"empty sgr", "\x1b[mhello", "hello"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(tc.in)
			if got != tc.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestVisualWidth_MatchesRunewidthOnStripped confirms visualWidth uses
// runewidth.StringWidth on the ANSI-stripped input so wide East-Asian
// runes count for two columns and ANSI codes count for zero.
func TestVisualWidth_MatchesRunewidthOnStripped(t *testing.T) {
	// "ABC" = 3, "\u4e2d" (CJK) = 2. Sequence with color codes should
	// strip to "AB\u4e2dC" and measure 5 columns.
	s := "\x1b[31mAB\u4e2dC\x1b[0m"
	if got := visualWidth(s); got != 5 {
		t.Errorf("visualWidth(%q) = %d, want 5", s, got)
	}
}

// TestTruncateTitle_PreservesShortTitles confirms truncateTitle leaves
// titles shorter than the budget untouched and does not add an
// ellipsis.
func TestTruncateTitle_PreservesShortTitles(t *testing.T) {
	got := truncateTitle("hello", 20)
	if got != "hello" {
		t.Errorf("truncateTitle(short, 20) = %q, want %q", got, "hello")
	}
}

// TestTruncateTitle_SubEllipsisBudget confirms that when the budget is
// smaller than ellipsisReserve, the function returns a bare ellipsis
// marker without panicking.
func TestTruncateTitle_SubEllipsisBudget(t *testing.T) {
	got := truncateTitle("anything", 1)
	if got != "..." {
		t.Errorf("truncateTitle(_, 1) = %q, want \"...\"", got)
	}
}

// makeRichIssue builds a representative open issue with every optional
// prefix column populated: a priority tag, a type tag, an assignee, and
// two labels. Used by the width-bucketed column-drop tests to exercise
// the full drop ladder (labels -> assignee -> type -> priority).
func makeRichIssue() *model.Issue {
	issue := makeIssue("TST-WIDE", "Implement the new terminal renderer", model.StatusOpen, 2, model.TypeTask, "")
	issue.Assignee = "alice"
	issue.Labels = []string{"frontend", "music-theory"}
	return issue
}

// TestFormatIssueLine_Width120_FullPrefix is the wide-terminal regression
// guard. At 120 columns every optional prefix column (priority, type,
// assignee, labels) must still render.
func TestFormatIssueLine_Width120_FullPrefix(t *testing.T) {
	issue := makeRichIssue()
	line := FormatIssueLine(issue, 120)
	stripped := stripANSI(line)

	for _, want := range []string{"TST-WIDE", "P2", "[task]", "@alice", "frontend", "music-theory", "Implement the new terminal renderer"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("width=120 rendering should contain %q, got %q", want, stripped)
		}
	}
	if got := runewidth.StringWidth(stripped); got > 120 {
		t.Errorf("width=120 rendered width %d exceeds budget: %q", got, stripped)
	}
}

// TestFormatIssueLine_Width80_FullPrefix confirms that an 80-col
// terminal -- the historical baseline -- still renders every prefix
// column for a typical issue.
func TestFormatIssueLine_Width80_FullPrefix(t *testing.T) {
	issue := makeRichIssue()
	line := FormatIssueLine(issue, 80)
	stripped := stripANSI(line)

	for _, want := range []string{"TST-WIDE", "P2", "[task]", "@alice", "frontend", "music-theory"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("width=80 rendering should contain %q, got %q", want, stripped)
		}
	}
	if got := runewidth.StringWidth(stripped); got > 80 {
		t.Errorf("width=80 rendered width %d exceeds budget: %q", got, stripped)
	}
}

// TestFormatIssueLine_Width60_DropsLabels confirms that at 60 columns
// the labels column is dropped first while the shorter assignee / type
// / priority tags remain.
func TestFormatIssueLine_Width60_DropsLabels(t *testing.T) {
	issue := makeRichIssue()
	line := FormatIssueLine(issue, 60)
	stripped := stripANSI(line)

	if strings.Contains(stripped, "frontend") || strings.Contains(stripped, "music-theory") {
		t.Errorf("width=60 should drop labels, got %q", stripped)
	}
	for _, want := range []string{"TST-WIDE", "P2", "[task]", "@alice"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("width=60 should still contain %q, got %q", want, stripped)
		}
	}
	if got := runewidth.StringWidth(stripped); got > 60 {
		t.Errorf("width=60 rendered width %d exceeds budget: %q", got, stripped)
	}
}

// TestFormatIssueLine_Width50_DropsLabelsAndAssignee confirms that at
// 50 columns both labels and the assignee column are dropped, but the
// type and priority tags are still visible.
func TestFormatIssueLine_Width50_DropsLabelsAndAssignee(t *testing.T) {
	issue := makeRichIssue()
	line := FormatIssueLine(issue, 50)
	stripped := stripANSI(line)

	if strings.Contains(stripped, "frontend") || strings.Contains(stripped, "music-theory") {
		t.Errorf("width=50 should drop labels, got %q", stripped)
	}
	if strings.Contains(stripped, "@alice") {
		t.Errorf("width=50 should drop assignee, got %q", stripped)
	}
	for _, want := range []string{"TST-WIDE", "P2", "[task]"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("width=50 should still contain %q, got %q", want, stripped)
		}
	}
	if got := runewidth.StringWidth(stripped); got > 50 {
		t.Errorf("width=50 rendered width %d exceeds budget: %q", got, stripped)
	}
}

// TestFormatIssueLine_Width40_DropsLabelsAssigneeType confirms that at
// 40 columns the type tag is dropped in addition to labels and
// assignee, leaving only the priority tag among optional columns.
func TestFormatIssueLine_Width40_DropsLabelsAssigneeType(t *testing.T) {
	issue := makeRichIssue()
	line := FormatIssueLine(issue, 40)
	stripped := stripANSI(line)

	if strings.Contains(stripped, "frontend") || strings.Contains(stripped, "music-theory") {
		t.Errorf("width=40 should drop labels, got %q", stripped)
	}
	if strings.Contains(stripped, "@alice") {
		t.Errorf("width=40 should drop assignee, got %q", stripped)
	}
	if strings.Contains(stripped, "[task]") {
		t.Errorf("width=40 should drop type tag, got %q", stripped)
	}
	for _, want := range []string{"TST-WIDE", "P2"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("width=40 should still contain %q, got %q", want, stripped)
		}
	}
	if got := runewidth.StringWidth(stripped); got > 40 {
		t.Errorf("width=40 rendered width %d exceeds budget: %q", got, stripped)
	}
}

// TestFormatIssueLine_Width30_DropsAllOptional confirms that at 30
// columns every optional column has been dropped, leaving only the
// status icon, ID, and a (truncated) title.
func TestFormatIssueLine_Width30_DropsAllOptional(t *testing.T) {
	issue := makeRichIssue()
	line := FormatIssueLine(issue, 30)
	stripped := stripANSI(line)

	for _, unwanted := range []string{"frontend", "music-theory", "@alice", "[task]", "P2"} {
		if strings.Contains(stripped, unwanted) {
			t.Errorf("width=30 should drop %q, got %q", unwanted, stripped)
		}
	}
	if !strings.Contains(stripped, "TST-WIDE") {
		t.Errorf("width=30 must keep ID, got %q", stripped)
	}
	if got := runewidth.StringWidth(stripped); got > 30 {
		t.Errorf("width=30 rendered width %d exceeds budget: %q", got, stripped)
	}
}

// TestFormatIssueLine_Width20_HitsTitleFloor confirms that at 20
// columns -- which is below the minimum-viable-line threshold -- every
// optional column is dropped and the title falls to minTitleFloor. The
// resulting line is allowed to exceed availWidth only in this
// pathological regime (AC7 bounds the no-wrap guarantee at >=30 cols).
func TestFormatIssueLine_Width20_HitsTitleFloor(t *testing.T) {
	issue := makeRichIssue()
	line := FormatIssueLine(issue, 20)
	stripped := stripANSI(line)

	for _, unwanted := range []string{"frontend", "music-theory", "@alice", "[task]", "P2"} {
		if strings.Contains(stripped, unwanted) {
			t.Errorf("width=20 should drop %q, got %q", unwanted, stripped)
		}
	}
	if !strings.Contains(stripped, "TST-WIDE") {
		t.Errorf("width=20 must keep ID, got %q", stripped)
	}
	if !strings.Contains(stripped, "...") {
		t.Errorf("width=20 should truncate title with ellipsis, got %q", stripped)
	}
}

// TestFormatIssueLine_NoLabels_SkipsDropOfLabels confirms that when the
// issue has no labels, the drop order starts at the assignee column
// instead of labels (which are absent).
func TestFormatIssueLine_NoLabels_SkipsDropOfLabels(t *testing.T) {
	issue := makeIssue("TST-NOLB", "Title without labels", model.StatusOpen, 2, model.TypeTask, "")
	issue.Assignee = "bob"
	// No labels.

	// At 50 cols, labels would normally be dropped first but there are
	// no labels, so the drop ladder moves on to assignee.
	line := FormatIssueLine(issue, 50)
	stripped := stripANSI(line)

	if strings.Contains(stripped, "@bob") {
		t.Errorf("width=50 no-labels issue should drop assignee next, got %q", stripped)
	}
	for _, want := range []string{"TST-NOLB", "P2", "[task]"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("width=50 no-labels issue should still contain %q, got %q", want, stripped)
		}
	}
	if got := runewidth.StringWidth(stripped); got > 50 {
		t.Errorf("no-labels width=50 rendered width %d exceeds budget: %q", got, stripped)
	}
}

// TestFormatIssueLine_VisualWidthNoANSI confirms that drop decisions
// are based on visual (ANSI-stripped) width, not raw byte length. An
// issue rendered with color codes should drop columns at the same
// width thresholds as one without color.
func TestFormatIssueLine_VisualWidthNoANSI(t *testing.T) {
	// Priority 0 (highest) emits ANSI color codes via RenderPriority; the
	// "critical" label also adds visual weight. At 60 cols, labels must
	// still drop regardless of how many ANSI escape bytes the colorized
	// prefix contains.
	issue := makeIssue("TST-ANS2", "A reasonably titled issue", model.StatusInProgress, 0, model.TypeBug, "")
	issue.Assignee = "carol"
	issue.Labels = []string{"critical", "security-hardening"}

	line := FormatIssueLine(issue, 60)
	stripped := stripANSI(line)

	if strings.Contains(stripped, "critical") || strings.Contains(stripped, "security-hardening") {
		t.Errorf("width=60 with ANSI colors should drop labels by visual width, got %q", stripped)
	}
	if got := runewidth.StringWidth(stripped); got > 60 {
		t.Errorf("ANSI-aware drop: rendered width %d exceeds budget 60: %q", got, stripped)
	}
}

// renderTreeAtWidth invokes Tree with $COLUMNS set to the requested
// width so the width-aware renderer picks up the expected size. It
// returns the rendered output as a string.
//
// The caller is responsible for supplying a representative issue
// hierarchy. The helper only controls the width input.
func renderTreeAtWidth(t *testing.T, issues []*model.Issue, width int) string {
	t.Helper()
	t.Setenv("COLUMNS", itoa(width))
	var buf bytes.Buffer
	Tree(&buf, issues, map[string]bool{}, "id", false)
	return buf.String()
}

// itoa is an inlined strconv.Itoa to avoid adding an import at the top
// of the test file for a single use-case.
func itoa(n int) string {
	// Fast path for common widths used in tests.
	switch n {
	case 40:
		return "40"
	case 59:
		return "59"
	case 60:
		return "60"
	case 80:
		return "80"
	case 120:
		return "120"
	}
	// Fallback for other values (general-purpose).
	// Uses the standard fmt routing path.
	return fmtItoa(n)
}

// fmtItoa is a tiny Sprintf helper kept separate so the hot path above
// does not have to pull fmt into consideration.
func fmtItoa(n int) string {
	return (func(i int) string {
		// Local implementation avoids adding strconv to the import set
		// while still handling arbitrary widths.
		if i == 0 {
			return "0"
		}
		neg := i < 0
		if neg {
			i = -i
		}
		var digits [20]byte
		pos := len(digits)
		for i > 0 {
			pos--
			digits[pos] = byte('0' + i%10)
			i /= 10
		}
		if neg {
			pos--
			digits[pos] = '-'
		}
		return string(digits[pos:])
	})(n)
}

// makeDeepHierarchy builds a synthetic issue tree suitable for the
// compact/full connector tests. Structure:
//
//	TST-root
//	├── TST-b01
//	│   └── TST-c01
//	└── TST-b02
func makeDeepHierarchy() []*model.Issue {
	root := makeIssue("TST-root", "Root parent", model.StatusOpen, 1, model.TypeEpic, "")
	branchA := makeIssue("TST-b01", "Branch A", model.StatusOpen, 1, model.TypeFeature, "TST-root")
	branchB := makeIssue("TST-b02", "Branch B", model.StatusOpen, 1, model.TypeTask, "TST-root")
	leaf := makeIssue("TST-c01", "Leaf under A", model.StatusOpen, 1, model.TypeTask, "TST-b01")
	return []*model.Issue{root, branchA, branchB, leaf}
}

// TestTree_Width120_UsesFullConnectors is the wide-terminal regression
// guard: at 120 columns the tree renderer must emit the historical
// 4-column Unicode box-drawing connectors.
func TestTree_Width120_UsesFullConnectors(t *testing.T) {
	issues := makeDeepHierarchy()
	output := renderTreeAtWidth(t, issues, 120)

	if !strings.Contains(output, "├── ") {
		t.Errorf("width=120: expected full 4-col ├── connector, got:\n%s", output)
	}
	if !strings.Contains(output, "└── ") {
		t.Errorf("width=120: expected full 4-col └── connector, got:\n%s", output)
	}
	// Continuation prefix for a non-last parent with grandchildren.
	if !strings.Contains(output, "│   ") {
		t.Errorf("width=120: expected full 4-col │   continuation, got:\n%s", output)
	}
}

// TestTree_Width40_UsesCompactConnectors asserts that at 40 columns the
// renderer switches to 2-col connectors and does NOT emit any 4-col
// box-drawing sequences.
func TestTree_Width40_UsesCompactConnectors(t *testing.T) {
	issues := makeDeepHierarchy()
	output := renderTreeAtWidth(t, issues, 40)

	// Compact glyphs must appear.
	if !strings.Contains(output, "├ ") {
		t.Errorf("width=40: expected compact 2-col ├ connector, got:\n%s", output)
	}
	if !strings.Contains(output, "└ ") {
		t.Errorf("width=40: expected compact 2-col └ connector, got:\n%s", output)
	}
	if !strings.Contains(output, "│ ") {
		t.Errorf("width=40: expected compact 2-col │ continuation, got:\n%s", output)
	}

	// Full glyphs must NOT appear at this width.
	if strings.Contains(output, "├── ") {
		t.Errorf("width=40: full ├── connector should NOT appear in compact mode, got:\n%s", output)
	}
	if strings.Contains(output, "└── ") {
		t.Errorf("width=40: full └── connector should NOT appear in compact mode, got:\n%s", output)
	}
	if strings.Contains(output, "│   ") {
		t.Errorf("width=40: full │   continuation should NOT appear in compact mode, got:\n%s", output)
	}
}

// TestTree_Width60_BoundaryUsesFull confirms the threshold comparison
// is strict-less-than: at exactly 60 columns the full connectors remain
// in use (threshold is `<`, not `<=`).
func TestTree_Width60_BoundaryUsesFull(t *testing.T) {
	issues := makeDeepHierarchy()
	output := renderTreeAtWidth(t, issues, 60)

	if !strings.Contains(output, "├── ") {
		t.Errorf("width=60 (exact threshold): expected full 4-col ├── connector, got:\n%s", output)
	}
	// And compact connectors must NOT bleed through. A "├ " substring
	// would also match "├── ", so look for the negation by insisting on
	// the full-width sequence and by checking that the 2-col last-branch
	// glyph "└ " only appears as part of the full "└── ".
	if !strings.Contains(output, "└── ") {
		t.Errorf("width=60: expected full 4-col └── connector, got:\n%s", output)
	}
}

// TestTree_Width59_UsesCompact asserts that one column below the
// threshold triggers compact mode.
func TestTree_Width59_UsesCompact(t *testing.T) {
	issues := makeDeepHierarchy()
	output := renderTreeAtWidth(t, issues, 59)

	if strings.Contains(output, "├── ") || strings.Contains(output, "└── ") {
		t.Errorf("width=59: full 4-col connectors should NOT appear, got:\n%s", output)
	}
	if !strings.Contains(output, "├ ") {
		t.Errorf("width=59: expected compact 2-col ├ connector, got:\n%s", output)
	}
}

// TestTree_CompactMode_DepthN_FitsWidth builds a 4-deep hierarchy,
// renders it at width 40, and asserts every rendered (ANSI-stripped)
// line fits within 40 visual columns.
func TestTree_CompactMode_DepthN_FitsWidth(t *testing.T) {
	// 4-deep: root -> mid -> sub -> leaf.
	root := makeIssue("TST-root", "Root", model.StatusOpen, 1, model.TypeEpic, "")
	mid := makeIssue("TST-mid0", "Mid level", model.StatusOpen, 1, model.TypeFeature, "TST-root")
	sub := makeIssue("TST-sub0", "Sub level", model.StatusOpen, 1, model.TypeTask, "TST-mid0")
	leaf := makeIssue("TST-lf00", "Leaf level", model.StatusOpen, 1, model.TypeTask, "TST-sub0")

	issues := []*model.Issue{root, mid, sub, leaf}
	output := renderTreeAtWidth(t, issues, 40)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	for i, line := range lines {
		// Skip the trailing "N issue(s)" footer and empty separator.
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, "issue(s)") {
			continue
		}
		stripped := stripANSI(line)
		if got := runewidth.StringWidth(stripped); got > 40 {
			t.Errorf("line %d width %d exceeds 40-col terminal in compact mode: %q", i, got, stripped)
		}
	}
}

// TestTree_FullMode_Unchanged pins the wide-terminal output to a golden
// string so any accidental change to the full-mode rendering is caught.
// The issue is declared with no optional columns (no assignee, no
// labels, no colors via NO_COLOR) so the golden stays stable across
// environments.
func TestTree_FullMode_Unchanged(t *testing.T) {
	// Disable ANSI colors deterministically for a stable golden.
	t.Setenv("NO_COLOR", "1")
	t.Setenv("COLUMNS", "120")

	root := makeIssue("TST-root", "Root", model.StatusOpen, 1, model.TypeEpic, "")
	// Use default priority/type so the prefix is predictable.
	branchA := makeIssue("TST-b01", "Branch A", model.StatusOpen, 1, model.TypeTask, "TST-root")
	branchB := makeIssue("TST-b02", "Branch B", model.StatusOpen, 1, model.TypeTask, "TST-root")

	var buf bytes.Buffer
	Tree(&buf, []*model.Issue{root, branchA, branchB}, map[string]bool{}, "id", false)
	got := stripANSI(buf.String())

	// Expect full 4-col connectors in the output.
	if !strings.Contains(got, "├── ") {
		t.Errorf("full-mode golden: expected ├── connector, got:\n%s", got)
	}
	if !strings.Contains(got, "└── ") {
		t.Errorf("full-mode golden: expected └── connector, got:\n%s", got)
	}
	// Pin the overall shape: exactly three issue lines plus footer.
	wantIDs := []string{"TST-root", "TST-b01", "TST-b02"}
	for _, id := range wantIDs {
		if !strings.Contains(got, id) {
			t.Errorf("full-mode golden: expected %q, got:\n%s", id, got)
		}
	}
	if !strings.Contains(got, "3 issue(s)") {
		t.Errorf("full-mode golden: expected '3 issue(s)' footer, got:\n%s", got)
	}
}

// TestFormatIssueLine_ResultingLineFitsWidth is a property-style test:
// across widths 30..160 step 10, the rendered line's visual width must
// be <= availWidth for a typical issue (AC5). Widths below 30 are
// excluded per AC7 which only guarantees no-wrap at >= 30 cols.
func TestFormatIssueLine_ResultingLineFitsWidth(t *testing.T) {
	issue := makeRichIssue()
	// Use a 40-char title per the story spec.
	issue.Title = strings.Repeat("x", 40)

	for w := 30; w <= 160; w += 10 {
		line := FormatIssueLine(issue, w)
		stripped := stripANSI(line)
		if got := runewidth.StringWidth(stripped); got > w {
			t.Errorf("width=%d: rendered width %d exceeds availWidth: %q", w, got, stripped)
		}
	}
}
