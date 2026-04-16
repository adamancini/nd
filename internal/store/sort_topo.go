package store

import "github.com/RamXX/nd/internal/model"

// TopoSortIssues returns issues sorted in topological dependency order using
// Kahn's algorithm. Issues that block others appear before the issues they block.
// Within the same topological level, issues are sorted by priority.
// Issues in cycles are appended at the end sorted by priority. No panic on cycles.
// Only edges where BOTH endpoints are in the input slice are considered.
func TopoSortIssues(issues []*model.Issue) []*model.Issue {
	if len(issues) == 0 {
		return issues
	}

	inSlice := make(map[string]*model.Issue, len(issues))
	for _, issue := range issues {
		inSlice[issue.ID] = issue
	}

	// in-degree = number of blockers that are also in the slice.
	inDegree := make(map[string]int, len(issues))
	for _, issue := range issues {
		inDegree[issue.ID] = 0
	}
	for _, issue := range issues {
		for _, blockedID := range issue.Blocks {
			if _, ok := inSlice[blockedID]; ok {
				inDegree[blockedID]++
			}
		}
	}

	// Seed queue: zero in-degree, sorted by priority.
	var queue []*model.Issue
	for _, issue := range issues {
		if inDegree[issue.ID] == 0 {
			queue = append(queue, issue)
		}
	}
	sortByPriority(queue)

	result := make([]*model.Issue, 0, len(issues))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		var newlyReady []*model.Issue
		for _, blockedID := range node.Blocks {
			if _, ok := inSlice[blockedID]; !ok {
				continue
			}
			inDegree[blockedID]--
			if inDegree[blockedID] == 0 {
				newlyReady = append(newlyReady, inSlice[blockedID])
			}
		}
		sortByPriority(newlyReady)
		queue = append(queue, newlyReady...)
		sortByPriority(queue)
	}

	// Append remaining (cycles) sorted by priority.
	seen := make(map[string]bool, len(result))
	for _, issue := range result {
		seen[issue.ID] = true
	}
	var remaining []*model.Issue
	for _, issue := range issues {
		if !seen[issue.ID] {
			remaining = append(remaining, issue)
		}
	}
	sortByPriority(remaining)
	return append(result, remaining...)
}

func sortByPriority(issues []*model.Issue) {
	sortByFunc(issues, func(a, b *model.Issue) bool {
		return a.Priority < b.Priority
	})
}
