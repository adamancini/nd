package format

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/RamXX/nd/internal/model"
	"github.com/RamXX/nd/internal/store"
	"github.com/RamXX/nd/internal/ui"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// defaultTerminalWidth is the fallback width used when stdout is not a tty
// (pipes, redirects, CI output capture) or when term.GetSize fails. It is
// also used by the tree renderer when no terminal width has been detected.
const defaultTerminalWidth = 120

// minTitleFloor is the minimum number of columns the title truncation logic
// will reserve, even when the terminal is narrower than the metadata prefix.
// Ensures that extremely narrow terminals still render something useful for
// the title (at least "..." plus a handful of characters) rather than a
// negative-length slice panic.
const minTitleFloor = 10

// ellipsisReserve is the number of columns reserved for the trailing "..."
// marker when a title has to be truncated.
const ellipsisReserve = 3

// ansiEscapeRE matches ANSI SGR escape sequences (CSI ... m) as emitted by
// lipgloss / termenv. Used to strip color codes before measuring the visual
// width of a rendered string.
var ansiEscapeRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// detectTerminalWidth returns the current terminal width in columns for
// stdout. When stdout is not a tty (pipe, redirect, CI) or when
// term.GetSize returns an error or non-positive width, it falls back to
// defaultTerminalWidth (120). The detection reads from os.Stdout.Fd() so
// callers get the real terminal width of the user's session without any
// plumbing.
func detectTerminalWidth() int {
	fd := int(os.Stdout.Fd())
	if !term.IsTerminal(fd) {
		return defaultTerminalWidth
	}
	w, _, err := term.GetSize(fd)
	if err != nil || w <= 0 {
		return defaultTerminalWidth
	}
	return w
}

// stripANSI removes ANSI SGR escape sequences from s. Used so the
// visible column width of a colorized string can be measured accurately.
func stripANSI(s string) string {
	return ansiEscapeRE.ReplaceAllString(s, "")
}

// visualWidth returns the visible column width of s with ANSI color
// escape sequences stripped out. Backed by runewidth so it correctly
// handles multi-byte runes and wide East-Asian characters.
func visualWidth(s string) int {
	return runewidth.StringWidth(stripANSI(s))
}

// truncateTitle truncates title so its visual width is at most budget
// columns. When truncation occurs, the trailing "..." marker is appended
// within the budget (budget includes the ellipsis). If budget falls below
// ellipsisReserve the function returns a bare "..." so callers never
// produce a negative-length slice.
func truncateTitle(title string, budget int) string {
	if budget < ellipsisReserve {
		return "..."
	}
	if runewidth.StringWidth(title) <= budget {
		return title
	}
	// runewidth.Truncate keeps the marker inside the target width.
	return runewidth.Truncate(title, budget, "...")
}

// FormatIssueLine renders a single issue as a one-line string suitable for
// Table or Tree output. Closed issues are rendered with RenderClosedLine.
//
// availWidth is the number of terminal columns available for the entire
// rendered line (including status icon, ID, priority/type tags, labels,
// separator and title). Callers are expected to subtract any tree-prefix
// width from the detected terminal width before passing availWidth so
// nested tree nodes do not wrap.
//
// The function computes the visual width of the non-title prefix (with
// ANSI color codes stripped) and truncates the title to fit. When the
// terminal is narrower than the prefix itself, a minimum title floor of
// minTitleFloor columns is preserved so the output remains scannable.
func FormatIssueLine(issue *model.Issue, availWidth int) string {
	status := string(issue.Status)
	isClosed := issue.Status == model.StatusClosed

	if isClosed {
		// Build prefix so we can budget the title. The prefix mirrors the
		// literal format string used below for the final line.
		prefix := fmt.Sprintf("%s %s [P%d] [%s] - ",
			ui.StatusIconClosed, issue.ID, issue.Priority, issue.Type)
		prefixWidth := visualWidth(prefix)
		budget := availWidth - prefixWidth
		if budget < minTitleFloor {
			budget = minTitleFloor
		}
		title := truncateTitle(issue.Title, budget)
		line := fmt.Sprintf("%s %s [P%d] [%s] - %s",
			ui.StatusIconClosed, issue.ID, issue.Priority, issue.Type, title)
		return ui.RenderClosedLine(line)
	}

	var parts []string
	parts = append(parts, ui.RenderStatusIcon(status))
	parts = append(parts, ui.RenderID(issue.ID))
	parts = append(parts, fmt.Sprintf("[%s]", ui.RenderPriority(int(issue.Priority))))
	parts = append(parts, fmt.Sprintf("[%s]", ui.RenderType(string(issue.Type))))
	if issue.Assignee != "" {
		parts = append(parts, fmt.Sprintf("@%s", issue.Assignee))
	}
	if len(issue.Labels) > 0 {
		parts = append(parts, fmt.Sprintf("[%s]", strings.Join(issue.Labels, ", ")))
	}

	// Compute the visual width of the non-title prefix. The prefix is:
	//   <joined parts> <space> "- "
	// because parts are joined with a single space and the separator
	// itself ("- ") starts the title segment.
	joinedPrefix := strings.Join(parts, " ")
	// Account for the trailing " - " separator before the title.
	prefixWidth := visualWidth(joinedPrefix) + len(" - ")
	budget := availWidth - prefixWidth
	if budget < minTitleFloor {
		budget = minTitleFloor
	}
	title := truncateTitle(issue.Title, budget)

	parts = append(parts, fmt.Sprintf("- %s", title))

	return strings.Join(parts, " ")
}

// Table renders a compact issue list with status icons, colors, and bd-style formatting.
// Format: STATUS_ICON ID [PRIORITY] [TYPE] @ASSIGNEE [LABELS] - TITLE
//
// Terminal width is detected once up-front via detectTerminalWidth and
// passed as availWidth to FormatIssueLine so titles truncate to fit the
// current terminal size (falling back to defaultTerminalWidth when stdout
// is not a tty).
func Table(w io.Writer, issues []*model.Issue) {
	if len(issues) == 0 {
		fmt.Fprintln(w, "No issues found.")
		return
	}

	termWidth := detectTerminalWidth()
	for _, issue := range issues {
		fmt.Fprintln(w, FormatIssueLine(issue, termWidth))
	}
	fmt.Fprintf(w, "\n%d issue(s)\n", len(issues))
}

// Tree renders issues grouped by parent with tree connectors (├──/└──).
// contextIDs marks parents that were fetched only for display context (not in
// the original filter result); they are rendered muted and excluded from count.
// sortBy and reverse control ordering of issues within each group.
func Tree(w io.Writer, issues []*model.Issue, contextIDs map[string]bool, sortBy string, reverse bool) {
	if len(issues) == 0 {
		fmt.Fprintln(w, "No issues found.")
		return
	}

	// Build a lookup and a parent->children map.
	issueMap := make(map[string]*model.Issue, len(issues))
	childrenOf := make(map[string][]*model.Issue) // parentID -> children
	var unparented []*model.Issue
	var topLevel []*model.Issue

	for _, issue := range issues {
		issueMap[issue.ID] = issue
	}

	for _, issue := range issues {
		if issue.Parent == "" {
			// No parent at all.
			unparented = append(unparented, issue)
		} else if _, parentInSlice := issueMap[issue.Parent]; parentInSlice {
			// Parent is in the slice -- this issue is a child.
			childrenOf[issue.Parent] = append(childrenOf[issue.Parent], issue)
		} else {
			// Parent not in the slice (and not fetched) -- treat as unparented.
			unparented = append(unparented, issue)
		}
	}

	// Identify top-level parents: issues that have children (or are context-only
	// parents) and are not themselves children of another issue in the slice.
	for _, issue := range issues {
		if len(childrenOf[issue.ID]) > 0 || contextIDs[issue.ID] {
			// Check this issue is not a child of another issue in the slice.
			if issue.Parent == "" || issueMap[issue.Parent] == nil {
				topLevel = append(topLevel, issue)
			}
			// If it IS a child, it will be rendered under its parent already.
		}
	}

	// Remove top-level parents from unparented (they are rendered separately).
	topLevelSet := make(map[string]bool, len(topLevel))
	for _, issue := range topLevel {
		topLevelSet[issue.ID] = true
	}
	filtered := unparented[:0]
	for _, issue := range unparented {
		if !topLevelSet[issue.ID] {
			filtered = append(filtered, issue)
		}
	}
	unparented = filtered

	// Sort groups.
	store.SortIssues(topLevel, sortBy, reverse)
	store.SortIssues(unparented, sortBy, reverse)

	// Count only non-context issues.
	issueCount := 0
	for _, issue := range issues {
		if !contextIDs[issue.ID] {
			issueCount++
		}
	}

	// Detect terminal width once for the entire render pass. Each node
	// passes this down and subtracts the visual width of its own tree
	// connector/prefix before budgeting its title.
	termWidth := detectTerminalWidth()

	// Render top-level parents and their children.
	for _, parent := range topLevel {
		renderTreeNode(w, parent, childrenOf, contextIDs, sortBy, reverse, "", termWidth)
	}

	// Render [Unparented] section if there are unparented issues.
	if len(unparented) > 0 {
		fmt.Fprintln(w, ui.RenderMuted("[Unparented]"))
		for i, issue := range unparented {
			connector := "├── "
			if i == len(unparented)-1 {
				connector = "└── "
			}
			avail := termWidth - visualWidth(connector)
			fmt.Fprintln(w, connector+FormatIssueLine(issue, avail))
		}
	}

	fmt.Fprintf(w, "\n%d issue(s)\n", issueCount)
}

// renderTreeNode renders a parent issue and its children recursively.
// termWidth is the total terminal width. The tree-prefix visual width
// (for example `├── ` or `│   ├── `) is subtracted from termWidth before
// passing the remaining budget to FormatIssueLine so nested nodes do not
// wrap on narrow terminals.
func renderTreeNode(w io.Writer, issue *model.Issue, childrenOf map[string][]*model.Issue, contextIDs map[string]bool, sortBy string, reverse bool, prefix string, termWidth int) {
	// Render the issue itself. The budget is the full terminal width
	// minus whatever prefix we are printing in front of the issue line.
	avail := termWidth - visualWidth(prefix)
	line := FormatIssueLine(issue, avail)
	if contextIDs[issue.ID] {
		line = ui.RenderClosedLine(line)
	}
	fmt.Fprintln(w, prefix+line)

	children := childrenOf[issue.ID]
	if len(children) == 0 {
		return
	}

	// Sort children within group.
	store.SortIssues(children, sortBy, reverse)

	for i, child := range children {
		connector := "├── "
		childPrefix := prefix + "│   "
		if i == len(children)-1 {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		childAvail := termWidth - visualWidth(prefix+connector)
		childLine := FormatIssueLine(child, childAvail)
		if contextIDs[child.ID] {
			childLine = ui.RenderClosedLine(childLine)
		}
		fmt.Fprintln(w, prefix+connector+childLine)

		// Recurse for deeper nesting.
		if len(childrenOf[child.ID]) > 0 {
			renderTreeNode_children(w, child.ID, childrenOf, contextIDs, sortBy, reverse, childPrefix, termWidth)
		}
	}
}

// renderTreeNode_children renders grandchildren+ at the correct indent level.
// termWidth is threaded down from the top-level Tree call so each nested
// line subtracts its own connector/prefix width from the same base budget.
func renderTreeNode_children(w io.Writer, parentID string, childrenOf map[string][]*model.Issue, contextIDs map[string]bool, sortBy string, reverse bool, prefix string, termWidth int) {
	children := childrenOf[parentID]
	store.SortIssues(children, sortBy, reverse)

	for i, child := range children {
		connector := "├── "
		childPrefix := prefix + "│   "
		if i == len(children)-1 {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		childAvail := termWidth - visualWidth(prefix+connector)
		childLine := FormatIssueLine(child, childAvail)
		if contextIDs[child.ID] {
			childLine = ui.RenderClosedLine(childLine)
		}
		fmt.Fprintln(w, prefix+connector+childLine)

		if len(childrenOf[child.ID]) > 0 {
			renderTreeNode_children(w, child.ID, childrenOf, contextIDs, sortBy, reverse, childPrefix, termWidth)
		}
	}
}

// Detail renders a single issue with colored output and markdown body.
func Detail(w io.Writer, issue *model.Issue) {
	status := string(issue.Status)

	// Header: STATUS_ICON ID . TITLE [PRIORITY . STATUS]
	fmt.Fprintf(w, "%s %s %s %s [%s %s %s]\n",
		ui.RenderStatusIcon(status),
		ui.RenderID(issue.ID),
		ui.RenderMuted("."),
		ui.RenderBold(issue.Title),
		ui.RenderPriority(int(issue.Priority)),
		ui.RenderMuted("."),
		ui.RenderStatus(status),
	)

	// Metadata line 1: Owner . Type
	var meta1 []string
	if issue.Assignee != "" {
		meta1 = append(meta1, fmt.Sprintf("%s %s", ui.RenderAccent("Owner:"), issue.Assignee))
	}
	meta1 = append(meta1, fmt.Sprintf("%s %s", ui.RenderAccent("Type:"), ui.RenderType(string(issue.Type))))
	fmt.Fprintln(w, strings.Join(meta1, fmt.Sprintf(" %s ", ui.RenderMuted("."))))

	// Metadata line 2: Created . Updated
	fmt.Fprintf(w, "%s %s %s %s %s\n",
		ui.RenderAccent("Created:"),
		issue.CreatedAt.Format("2006-01-02 15:04"),
		ui.RenderMuted("."),
		ui.RenderAccent("Updated:"),
		issue.UpdatedAt.Format("2006-01-02 15:04"),
	)

	if issue.CreatedBy != "" {
		fmt.Fprintf(w, "%s %s\n", ui.RenderAccent("Author:"), issue.CreatedBy)
	}
	if len(issue.Labels) > 0 {
		fmt.Fprintf(w, "%s %s\n", ui.RenderAccent("Labels:"), strings.Join(issue.Labels, ", "))
	}
	if issue.Parent != "" {
		fmt.Fprintf(w, "%s %s\n", ui.RenderAccent("Parent:"), issue.Parent)
	}
	if len(issue.Blocks) > 0 {
		fmt.Fprintf(w, "%s %s\n", ui.RenderAccent("Blocks:"), strings.Join(issue.Blocks, ", "))
	}
	if len(issue.BlockedBy) > 0 {
		fmt.Fprintf(w, "%s %s\n", ui.RenderAccent("Blocked by:"), strings.Join(issue.BlockedBy, ", "))
	}
	if len(issue.Related) > 0 {
		fmt.Fprintf(w, "%s %s\n", ui.RenderAccent("Related:"), strings.Join(issue.Related, ", "))
	}
	if issue.ClosedAt != "" {
		fmt.Fprintf(w, "%s %s\n", ui.RenderAccent("Closed:"), issue.ClosedAt)
	}
	if issue.CloseReason != "" {
		fmt.Fprintf(w, "%s %s\n", ui.RenderAccent("Reason:"), issue.CloseReason)
	}

	if issue.Body != "" {
		fmt.Fprintln(w)
		fmt.Fprint(w, ui.RenderMarkdown(issue.Body))
	}
}

// JSON outputs issues as JSON.
func JSON(w io.Writer, issues []*model.Issue) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(issues)
}

// JSONSingle outputs a single issue as JSON.
func JSONSingle(w io.Writer, issue *model.Issue) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(issue)
}

// Short renders a one-line summary of an issue.
func Short(w io.Writer, issue *model.Issue) {
	fmt.Fprintf(w, "%s %s [%s] %s (%s)\n",
		ui.RenderStatusIcon(string(issue.Status)),
		issue.ID,
		ui.RenderStatus(string(issue.Status)),
		issue.Title,
		ui.RenderPriority(int(issue.Priority)),
	)
}
