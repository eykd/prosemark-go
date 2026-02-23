package ops

import (
	"context"
	"fmt"
	"regexp"
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

	// Evaluate selector with deep-tree search.
	nodes, selDiags := deleteEvalSelector(params.Selector, result.Root)
	if selDiags != nil {
		// Fatal selector error (OPE001): return src unchanged.
		hasError := false
		for _, d := range selDiags {
			if d.Severity == "error" {
				hasError = true
				break
			}
		}
		if hasError {
			return src, append(parseDiags, selDiags...), nil
		}
	}

	node := nodes[0]

	// Collect diagnostics: parse warnings + selector warnings (OPW001).
	var allDiags []binder.Diagnostic
	allDiags = append(allDiags, parseDiags...)
	allDiags = append(allDiags, selDiags...)

	// Emit OPW003 if the node's list-item line has non-structural content.
	if deleteNodeHasNonStructuralContent(node.RawLine) {
		allDiags = append(allDiags, binder.Diagnostic{
			Severity: "warning",
			Code:     binder.CodeNonStructuralDestroyed,
			Message:  "non-structural content in the deleted list item was destroyed",
		})
	}

	// Find parent to determine whether its sublist will become empty.
	parent := deleteFindParentNode(result.Root, node)
	lastChildOfNonRoot := parent != nil && parent.Type != "root" && len(parent.Children) == 1

	// Compute the subtree end line (parser does not populate SubtreeEnd).
	subtreeEndLine := deleteComputeSubtreeEnd(node)

	// Delete lines node.Line..subtreeEndLine (1-based, inclusive).
	startIdx := node.Line - 1
	endIdx := subtreeEndLine - 1
	newLines := make([]string, 0, len(result.Lines)-(endIdx-startIdx+1))
	newLines = append(newLines, result.Lines[:startIdx]...)
	newLines = append(newLines, result.Lines[endIdx+1:]...)
	newLineEnds := make([]string, 0, len(result.LineEnds)-(endIdx-startIdx+1))
	newLineEnds = append(newLineEnds, result.LineEnds[:startIdx]...)
	newLineEnds = append(newLineEnds, result.LineEnds[endIdx+1:]...)
	result.Lines = newLines
	result.LineEnds = newLineEnds

	// Emit OPW004 if the parent's sublist is now empty.
	if lastChildOfNonRoot {
		allDiags = append(allDiags, binder.Diagnostic{
			Severity: "warning",
			Code:     binder.CodeEmptySublistPruned,
			Message:  "empty sublist was pruned after deleting sole child",
		})
	}

	// Collapse consecutive blank lines (no \n\n\n or more in output).
	result.Lines, result.LineEnds = deleteCollapseBlankLines(result.Lines, result.LineEnds)

	// Strip trailing blank lines at EOF.
	result.Lines, result.LineEnds = deleteStripTrailingBlanks(result.Lines, result.LineEnds)

	return binder.Serialize(result), allDiags, nil
}

// deleteEvalSelector performs a deep tree search for nodes matching the bare-stem
// selector. Returns OPE001 if no match. Returns OPW001 warning and first match if
// multiple nodes match (regardless of whether their targets differ).
func deleteEvalSelector(selector string, root *binder.Node) ([]*binder.Node, []binder.Diagnostic) {
	// "." refers to the root node, which cannot be deleted.
	if selector == "." {
		return nil, []binder.Diagnostic{{
			Severity: "error",
			Code:     binder.CodeSelectorNoMatch,
			Message:  `selector "." matches the root node which cannot be deleted`,
		}}
	}

	var matches []*binder.Node
	deleteSearchTree(root, selector, &matches)

	if len(matches) == 0 {
		return nil, []binder.Diagnostic{{
			Severity: "error",
			Code:     binder.CodeSelectorNoMatch,
			Message:  fmt.Sprintf("selector %q matched no nodes", selector),
		}}
	}

	if len(matches) > 1 {
		return []*binder.Node{matches[0]}, []binder.Diagnostic{{
			Severity: "warning",
			Code:     binder.CodeMultiMatch,
			Message:  fmt.Sprintf("selector %q matched %d nodes; targeting first match", selector, len(matches)),
		}}
	}

	return matches, nil
}

// deleteSearchTree appends to matches all nodes in the subtree rooted at n
// whose target stem matches selector.
func deleteSearchTree(n *binder.Node, selector string, matches *[]*binder.Node) {
	for _, child := range n.Children {
		if opStemFromPath(child.Target) == selector {
			*matches = append(*matches, child)
		}
		deleteSearchTree(child, selector, matches)
	}
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
// beyond the structural inline link (OPW003 trigger).
func deleteNodeHasNonStructuralContent(rawLine string) bool {
	loc := deleteInlineLinkRE.FindStringIndex(rawLine)
	if loc == nil {
		return false
	}
	rest := strings.TrimSpace(rawLine[loc[1]:])
	return rest != ""
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
