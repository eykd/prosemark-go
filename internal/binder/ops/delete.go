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
// from the binder source. Returns the modified bytes, diagnostics, and any
// fatal error. Source bytes are unchanged on error (atomic abort semantics).
func Delete(ctx context.Context, src []byte, project *binder.Project, params binder.DeleteParams) ([]byte, []binder.Diagnostic, error) {
	// Require --yes confirmation (OPE009).
	if !params.Yes {
		return src, []binder.Diagnostic{{
			Severity: "error",
			Code:     binder.CodeIOOrParseFailure,
			Message:  "delete requires --yes confirmation",
		}}, nil
	}

	// Parse the source.
	result, parseDiags, err := deleteParseBinderFn(ctx, src, project)
	if err != nil {
		return src, append(parseDiags, binder.Diagnostic{
			Severity: "error",
			Code:     binder.CodeIOOrParseFailure,
			Message:  fmt.Sprintf("parse error: %v", err),
		}), err
	}

	// Evaluate selector: supports path navigation (colon), index qualifiers ([N]),
	// flat deep search (bare stem), and code-fence detection.
	nodes, selDiags := deleteEvalSelector(params.Selector, result.Root, result.Lines, project)
	if len(nodes) == 0 {
		// Fatal selector error (OPE001/OPE002/OPE006): return src unchanged.
		return src, append(parseDiags, selDiags...), nil
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

	// Sort nodes by Line descending so deletions are applied bottom-to-top,
	// keeping earlier line numbers valid across iterations.
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Line > nodes[j].Line
	})

	for _, node := range nodes {
		startIdx := node.Line - 1
		endIdx := deleteComputeSubtreeEnd(node) - 1
		result.Lines = deleteRemoveRange(result.Lines, startIdx, endIdx)
		result.LineEnds = deleteRemoveRange(result.LineEnds, startIdx, endIdx)
	}

	// Collapse consecutive blank lines (no \n\n\n or more in output).
	result.Lines, result.LineEnds = deleteCollapseBlankLines(result.Lines, result.LineEnds)

	// Strip trailing blank lines at EOF.
	result.Lines, result.LineEnds = deleteStripTrailingBlanks(result.Lines, result.LineEnds)

	return binder.Serialize(result), allDiags, nil
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
// direct path, stem+".md", or case-insensitive title.
func deleteNodeMatchesSelector(child *binder.Node, selector string) bool {
	if strings.Contains(selector, "/") {
		return child.Target == selector || child.Target == selector+".md"
	}
	return opStemFromPath(child.Target) == selector ||
		child.Target == selector ||
		strings.EqualFold(child.Title, selector)
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

// deleteStripTrailingBlanks removes blank lines from the end of lines/lineEnds.
func deleteStripTrailingBlanks(lines, lineEnds []string) ([]string, []string) {
	n := len(lines)
	for n > 0 && lines[n-1] == "" {
		n--
	}
	return lines[:n], lineEnds[:n]
}
