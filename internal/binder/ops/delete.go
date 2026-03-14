package ops

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/eykd/prosemark-go/internal/binder"
)

// deleteParseBinderFn is the parse function used by Delete. It may be replaced
// in tests to simulate parse failures.
var deleteParseBinderFn = binder.Parse

// deleteInlineLinkRE matches a complete inline markdown link [text](url).
var deleteInlineLinkRE = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)

// Delete removes the node selected by params.Selector and its entire subtree
// from the binder source. Returns the modified bytes and diagnostics. Source
// bytes are unchanged on error (atomic abort semantics). Parse errors are
// surfaced as diagnostics, not as a returned error.
func Delete(ctx context.Context, src []byte, project *binder.Project, params binder.DeleteParams) ([]byte, []binder.Diagnostic) {
	// Require --yes confirmation (OPE011).
	if !params.Yes {
		return src, []binder.Diagnostic{{
			Severity: "error",
			Code:     binder.CodeMissingConfirmation,
			Message:  "delete requires --yes confirmation",
		}}
	}

	// Parse the source.
	result, parseDiags, err := deleteParseBinderFn(ctx, src, project)
	if err != nil {
		return src, append(parseDiags, binder.Diagnostic{
			Severity: "error",
			Code:     binder.CodeIOOrParseFailure,
			Message:  fmt.Sprintf("parse error: %v", err),
		})
	}

	// Evaluate selector: supports path navigation (colon), index qualifiers ([N]),
	// flat deep search (bare stem), and code-fence detection.
	nodes, selDiags := deleteEvalSelector(params.Selector, result.Root, result.Lines, project)
	if len(nodes) == 0 {
		// Fatal selector error (OPE001/OPE002/OPE006): return src unchanged.
		return src, append(parseDiags, selDiags...)
	}

	// Collect diagnostics: parse warnings + selector warnings (OPW001).
	var allDiags []binder.Diagnostic
	allDiags = append(allDiags, parseDiags...)
	allDiags = append(allDiags, selDiags...)

	// Emit OPW003 if any node's list-item line has non-structural content.
	for _, node := range nodes {
		if deleteNodeHasNonStructuralContent(node.RawLine) {
			allDiags = append(allDiags, binder.Diagnostic{
				Severity: "warning",
				Code:     binder.CodeNonStructuralDestroyed,
				Message:  "non-structural content in the deleted list item was destroyed",
			})
			break
		}
	}

	// Emit OPW004 if any node is the sole child of a non-root list item.
	for _, node := range nodes {
		parent := deleteFindParentNode(result.Root, node)
		if parent != nil && parent.Type != "root" && len(parent.Children) == 1 {
			allDiags = append(allDiags, binder.Diagnostic{
				Severity: "warning",
				Code:     binder.CodeEmptySublistPruned,
				Message:  "empty sublist was pruned after deleting sole child",
			})
			break
		}
	}

	// Emit OPW005 if any node has children (cascade delete).
	for _, node := range nodes {
		if len(node.Children) > 0 {
			allDiags = append(allDiags, binder.Diagnostic{
				Severity: "warning",
				Code:     binder.CodeCascadeDelete,
				Message:  fmt.Sprintf("deleting %q also removed its %d descendant(s)", node.Target, countDescendants(node)),
			})
		}
	}

	// Sort nodes by Line descending so deletions are applied bottom-to-top,
	// keeping earlier line numbers valid across iterations.
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Line > nodes[j].Line
	})

	// Collect ref labels used by deleted nodes (including subtrees) before
	// removing lines, so we can detect orphaned reference definitions.
	deletedLabels := make(map[string]bool)
	for _, node := range nodes {
		if label := deleteExtractRefLabel(node.RawLine); label != "" {
			deletedLabels[label] = true
		}
		deleteCollectRefLabels(node, deletedLabels)
	}

	for _, node := range nodes {
		startIdx := node.Line - 1
		endIdx := deleteComputeSubtreeEnd(node) - 1
		result.Lines = deleteRemoveRange(result.Lines, startIdx, endIdx)
		result.LineEnds = deleteRemoveRange(result.LineEnds, startIdx, endIdx)
	}

	// Remove orphaned reference definitions (OPW006).
	if len(deletedLabels) > 0 {
		// Build set of deleted node pointers for fast lookup.
		deletedSet := make(map[*binder.Node]bool, len(nodes))
		for _, node := range nodes {
			deletedSet[node] = true
		}
		// Collect labels still referenced by surviving nodes.
		survivingLabels := make(map[string]bool)
		deleteCollectRefLabelsExcluding(result.Root, deletedSet, survivingLabels)
		// Remove labels that are still in use from the orphan set.
		for label := range deletedLabels {
			if survivingLabels[label] {
				delete(deletedLabels, label)
			}
		}
		if len(deletedLabels) > 0 {
			result.Lines, result.LineEnds = deleteRemoveOrphanRefDefs(
				result.Lines, result.LineEnds, deletedLabels,
			)
			allDiags = append(allDiags, binder.Diagnostic{
				Severity: "warning",
				Code:     binder.CodeOrphanRefDefCleaned,
				Message:  "orphaned reference definition(s) removed after deletion",
			})
		}
	}

	// Collapse consecutive blank lines (no \n\n\n or more in output).
	result.Lines, result.LineEnds = deleteCollapseBlankLines(result.Lines, result.LineEnds)

	// Strip trailing blank lines at EOF.
	result.Lines, result.LineEnds = deleteStripTrailingBlanks(result.Lines, result.LineEnds)

	return binder.Serialize(result), allDiags
}

// deleteEvalSelector evaluates a selector for the delete operation.
//
// For selectors containing ":" or "[", path navigation via binder.EvalSelector
// is used. Otherwise a deep-tree flat search is performed.
//
// When no match is found, the function checks for project-level ambiguity
// (OPE002) and code-fence presence (OPE006) before returning OPE001.
func deleteEvalSelector(selector string, root *binder.Node, lines []string, project *binder.Project) ([]*binder.Node, []binder.Diagnostic) {
	// "." refers to the root node, which cannot be deleted.
	if selector == "." {
		return nil, []binder.Diagnostic{{
			Severity: "error",
			Code:     binder.CodeSelectorNoMatch,
			Message:  `selector "." matches the root node which cannot be deleted`,
		}}
	}

	// Delegate to EvalSelector for path (colon) or index ([N]) selectors.
	if strings.Contains(selector, ":") || strings.Contains(selector, "[") {
		selResult, errDiags := binder.EvalSelector(selector, root)
		allDiags := append(selResult.Warnings, errDiags...)
		return selResult.Nodes, allDiags
	}

	// Flat deep-tree search for bare-stem selectors.
	var matches []*binder.Node
	deleteSearchTree(root, selector, &matches)

	if len(matches) == 0 {
		// Check for project-level stem ambiguity (OPE002).
		if project != nil {
			var stemMatches []string
			for _, f := range project.Files {
				if opStemFromPath(f) == selector {
					stemMatches = append(stemMatches, f)
				}
			}
			if len(stemMatches) > 1 {
				return nil, []binder.Diagnostic{{
					Severity: "error",
					Code:     binder.CodeAmbiguousBareStem,
					Message:  fmt.Sprintf("selector %q is ambiguous: matches multiple project files", selector),
				}}
			}
		}
		// Check for code-fence presence (OPE006).
		if isSelectorInCodeFence(lines, selector) {
			return nil, []binder.Diagnostic{{
				Severity: "error",
				Code:     binder.CodeNodeInCodeFence,
				Message:  fmt.Sprintf("selector %q matches a node inside a code fence", selector),
			}}
		}
		return nil, []binder.Diagnostic{{
			Severity: "error",
			Code:     binder.CodeSelectorNoMatch,
			Message:  fmt.Sprintf("selector %q matched no nodes", selector),
		}}
	}

	if len(matches) > 1 {
		return matches, []binder.Diagnostic{{
			Severity: "warning",
			Code:     binder.CodeMultiMatch,
			Message:  fmt.Sprintf("selector %q matched %d nodes; operation applied to all matches", selector, len(matches)),
		}}
	}

	return matches, nil
}

// deleteSearchTree appends to matches all nodes in the subtree rooted at n
// whose target or title matches selector.
func deleteSearchTree(n *binder.Node, selector string, matches *[]*binder.Node) {
	for _, child := range n.Children {
		if deleteNodeMatchesSelector(child, selector) {
			*matches = append(*matches, child)
		}
		deleteSearchTree(child, selector, matches)
	}
}

// deleteNodeMatchesSelector reports whether child matches selector by stem,
// direct path, stem+".md", or case-insensitive title. Path-containing
// selectors bypass stem and title matching.
func deleteNodeMatchesSelector(child *binder.Node, selector string) bool {
	if strings.Contains(selector, "/") {
		return child.Target == selector || child.Target == selector+".md"
	}
	return nodeMatchesSelector(child, selector)
}

// deleteComputeSubtreeEnd returns the 1-based line number of the last line in
// the subtree rooted at n (the parser does not populate SubtreeEnd).
func deleteComputeSubtreeEnd(n *binder.Node) int {
	if len(n.Children) == 0 {
		return n.Line
	}
	return deleteComputeSubtreeEnd(n.Children[len(n.Children)-1])
}

// deleteFindParentNode returns the parent of target in the subtree rooted at
// root, or nil if target is not found.
func deleteFindParentNode(root *binder.Node, target *binder.Node) *binder.Node {
	for _, child := range root.Children {
		if child == target {
			return root
		}
		if found := deleteFindParentNode(child, target); found != nil {
			return found
		}
	}
	return nil
}

// deleteNodeHasNonStructuralContent reports whether rawLine contains content
// beyond the structural inline link (OPW003 trigger). Checks both prefix
// (e.g. GFM checkbox) and suffix.
func deleteNodeHasNonStructuralContent(rawLine string) bool {
	loc := deleteInlineLinkRE.FindStringIndex(rawLine)
	if loc == nil {
		return false
	}
	// Check suffix (content after the link).
	if strings.TrimSpace(rawLine[loc[1]:]) != "" {
		return true
	}
	// Check prefix (between list marker and link) — catches GFM checkboxes.
	prefix := moveListMarkerRE.ReplaceAllString(rawLine[:loc[0]], "")
	return strings.TrimSpace(prefix) != ""
}

// deleteRemoveRange returns a copy of s with elements [startIdx, endIdx] removed
// (0-based, inclusive).
func deleteRemoveRange(s []string, startIdx, endIdx int) []string {
	out := make([]string, 0, len(s)-(endIdx-startIdx+1))
	out = append(out, s[:startIdx]...)
	out = append(out, s[endIdx+1:]...)
	return out
}

// deleteCollapseBlankLines removes duplicate consecutive blank lines, ensuring
// no more than one blank line appears in a row.
func deleteCollapseBlankLines(lines, lineEnds []string) ([]string, []string) {
	newLines := make([]string, 0, len(lines))
	newEnds := make([]string, 0, len(lineEnds))
	prevBlank := false
	for i, line := range lines {
		isBlank := line == ""
		if isBlank && prevBlank {
			continue
		}
		newLines = append(newLines, line)
		newEnds = append(newEnds, lineEnds[i])
		prevBlank = isBlank
	}
	return newLines, newEnds
}

// countDescendants returns the total number of descendants (children + their children, etc.)
// in the subtree rooted at n.
func countDescendants(n *binder.Node) int {
	count := 0
	for _, child := range n.Children {
		count += 1 + countDescendants(child)
	}
	return count
}

// deleteStripTrailingBlanks removes blank lines from the end of lines/lineEnds.
func deleteStripTrailingBlanks(lines, lineEnds []string) ([]string, []string) {
	n := len(lines)
	for n > 0 && lines[n-1] == "" {
		n--
	}
	return lines[:n], lineEnds[:n]
}

// deleteRefLabelRE matches a full reference-style link [text][label] and
// captures the label in group 1.
var deleteRefLabelRE = regexp.MustCompile(`\[[^\]]*\]\[([^\]]+)\]`)

// deleteCollectRefLabels recursively collects lowercase reference link labels
// used in the subtree rooted at n.
func deleteCollectRefLabels(n *binder.Node, labels map[string]bool) {
	for _, child := range n.Children {
		if label := deleteExtractRefLabel(child.RawLine); label != "" {
			labels[label] = true
		}
		deleteCollectRefLabels(child, labels)
	}
}

// deleteCollectRefLabelsExcluding recursively collects lowercase reference link
// labels, skipping nodes in the excluded set and their subtrees.
func deleteCollectRefLabelsExcluding(n *binder.Node, excluded map[*binder.Node]bool, labels map[string]bool) {
	for _, child := range n.Children {
		if excluded[child] {
			continue
		}
		if label := deleteExtractRefLabel(child.RawLine); label != "" {
			labels[label] = true
		}
		deleteCollectRefLabelsExcluding(child, excluded, labels)
	}
}

// deleteExtractRefLabel returns the lowercase reference label from a raw line
// containing a reference-style link, or "" if none found.
func deleteExtractRefLabel(rawLine string) string {
	m := deleteRefLabelRE.FindStringSubmatch(rawLine)
	if m == nil {
		return ""
	}
	return strings.ToLower(m[1])
}

// deleteRemoveOrphanRefDefs removes lines that are reference definitions for
// any of the given orphaned labels.
func deleteRemoveOrphanRefDefs(lines, lineEnds []string, orphanLabels map[string]bool) ([]string, []string) {
	newLines := make([]string, 0, len(lines))
	newEnds := make([]string, 0, len(lineEnds))
	for i, line := range lines {
		if deleteIsOrphanRefDefLine(line, orphanLabels) {
			continue
		}
		newLines = append(newLines, line)
		newEnds = append(newEnds, lineEnds[i])
	}
	return newLines, newEnds
}

// deleteIsOrphanRefDefLine reports whether line is a reference definition whose
// label is in orphanLabels.
func deleteIsOrphanRefDefLine(line string, orphanLabels map[string]bool) bool {
	m := refDefLineRE.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	return orphanLabels[strings.ToLower(m[1])]
}

// refDefLineRE matches a markdown reference definition line: [label]: url
var refDefLineRE = regexp.MustCompile(`^\[([^\]]+)\]:\s+`)
