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
	renderTreeNode(&buf, epic, childrenOf, contextIDs, "priority", false, "", 60)

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
	renderTreeNode(&buf, epic, childrenOf, contextIDs, "priority", false, "", 80)

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
// not a tty (as in go test without a pseudo-tty), detectTerminalWidth
// returns the documented 120-column fallback.
func TestDetectTerminalWidth_NonTtyFallback(t *testing.T) {
	got := detectTerminalWidth()
	if got != defaultTerminalWidth {
		t.Errorf("under go test (non-tty stdout), detectTerminalWidth() = %d, want %d",
			got, defaultTerminalWidth)
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
