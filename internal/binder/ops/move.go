package ops

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/eykd/prosemark-go/internal/binder"
)

// moveListMarkerRE matches the leading whitespace + list marker + space of a list item.
var moveListMarkerRE = regexp.MustCompile(`^[\t ]*(?:[-*+]|\d+[.)]) `)

// moveFirstLineMarkerRE matches only the list marker + space (no leading whitespace).
var moveFirstLineMarkerRE = regexp.MustCompile(`^(?:[-*+]|\d+[.)]) `)

// moveOpsCheckboxRE matches a GFM task-list checkbox at the start of content.
var moveOpsCheckboxRE = regexp.MustCompile(`^\[[xX ]\]\s+`)

// moveParseBinderFn is the parse function used by Move. It may be replaced
// in tests to simulate parse failures.
var moveParseBinderFn = binder.Parse

// Move relocates the source node (and its subtree) under the destination parent.
// Returns the modified bytes, diagnostics, and any fatal error. Source bytes are
// unchanged on error (atomic abort semantics).
func Move(ctx context.Context, src []byte, project *binder.Project, params binder.MoveParams) ([]byte, []binder.Diagnostic, error) {
	// Require --yes confirmation (OPE009).
	if !params.Yes {
		return src, []binder.Diagnostic{{
			Severity: "error",
			Code:     binder.CodeIOOrParseFailure,
			Message:  "move requires --yes confirmation",
		}}, nil
	}

	// Parse the source.
	result, parseDiags, err := moveParseBinderFn(ctx, src, project)
	if err != nil {
		return src, append(parseDiags, binder.Diagnostic{
			Severity: "error",
			Code:     binder.CodeIOOrParseFailure,
			Message:  fmt.Sprintf("parse error: %v", err),
		}), err
	}

	// Find source nodes.
	sourceNodes, selDiags := moveEvalSourceSelector(params.SourceSelector, result.Root, result.Lines)
	if len(sourceNodes) == 0 {
		return src, append(parseDiags, selDiags...), nil
	}

	var allDiags []binder.Diagnostic
	allDiags = append(allDiags, parseDiags...)
	allDiags = append(allDiags, selDiags...)

	// Find destination parent.
	destNode, destDiags := moveEvalDestSelector(params.DestinationParentSelector, result.Root)
	if destNode == nil {
		return src, append(allDiags, destDiags...), nil
	}
	allDiags = append(allDiags, destDiags...)

	// Cycle detection: destination must not be a descendant of any source node.
	for _, srcNode := range sourceNodes {
		if srcNode == destNode || moveIsDescendant(srcNode, destNode) {
			return src, append(allDiags, binder.Diagnostic{
				Severity: "error",
				Code:     binder.CodeCycleDetected,
				Message:  "destination is a descendant of source: cycle detected",
			}), nil
		}
	}

	// OPW003: warn if any source node has non-structural content.
	for _, srcNode := range sourceNodes {
		if moveNodeHasNonStructuralContent(srcNode.RawLine) {
			allDiags = append(allDiags, binder.Diagnostic{
				Severity: "warning",
				Code:     binder.CodeNonStructuralDestroyed,
				Message:  "non-structural content in source list item will be destroyed",
			})
			break
		}
	}

	// OPW004: warn if any non-root parent loses its sole child.
	if moveAnyParentLosesAllChildren(result.Root, sourceNodes) {
		allDiags = append(allDiags, binder.Diagnostic{
			Severity: "warning",
			Code:     binder.CodeEmptySublistPruned,
			Message:  "empty sublist was pruned after moving sole child",
		})
	}

	var moveInsertIdx int
	if params.Position == "first" {
		moveInsertIdx = 0
	} else {
		moveInsertIdx = len(destNode.Children)
	}
	targetIndentStr, targetMarker := inferMarkerAndIndent(destNode, moveInsertIdx)
	return moveRebuildDocument(result, sourceNodes, destNode, params.Position, targetIndentStr, targetMarker), allDiags, nil
}

// moveRebuildDocument removes sourceNodes from their current positions,
// re-indents them to match targetIndentStr (and replaces their list marker
// with targetMarker), and inserts them under destNode at the given position
// ("first" or "last"). Returns the serialized result.
func moveRebuildDocument(result *binder.ParseResult, sourceNodes []*binder.Node, destNode *binder.Node, position, targetIndentStr, targetMarker string) []byte {
	// Collect re-indented lines and mark source indices for removal.
	var movedLines []string
	var movedLineEnds []string
	skipSet := make(map[int]bool)

	for _, srcNode := range sourceNodes {
		startIdx := srcNode.Line - 1
		endIdx := deleteComputeSubtreeEnd(srcNode) - 1
		srcIndentLen := srcNode.Indent

		for i := startIdx; i <= endIdx; i++ {
			var reindented string
			if i == startIdx {
				// Replace marker and strip any GFM checkbox on the first line.
				reindented = moveReindentFirstLine(result.Lines[i], srcIndentLen, targetIndentStr, targetMarker)
			} else {
				reindented = moveReindentLine(result.Lines[i], srcIndentLen, targetIndentStr)
			}
			movedLines = append(movedLines, reindented)
			movedLineEnds = append(movedLineEnds, result.LineEnds[i])
			skipSet[i] = true
		}
	}

	// Determine raw insertion index in the original document.
	var insertIdx int
	if position == "first" {
		insertIdx = insertionLineIdx(destNode, 0, result)
	} else {
		insertIdx = insertionLineIdx(destNode, len(destNode.Children), result)
	}

	// Count source lines before insertIdx to compute the adjusted insert position.
	removedBefore := 0
	for idx := range skipSet {
		if idx < insertIdx {
			removedBefore++
		}
	}
	adjustedInsertIdx := insertIdx - removedBefore

	// Build the new document.
	newLines := make([]string, 0, len(result.Lines)+len(movedLines))
	newLineEnds := make([]string, 0, len(result.LineEnds)+len(movedLineEnds))
	inserted := false
	pos := 0

	for i, line := range result.Lines {
		if skipSet[i] {
			continue
		}
		if !inserted && pos == adjustedInsertIdx {
			newLines = append(newLines, movedLines...)
			newLineEnds = append(newLineEnds, movedLineEnds...)
			inserted = true
		}
		newLines = append(newLines, line)
		newLineEnds = append(newLineEnds, result.LineEnds[i])
		pos++
	}
	if !inserted {
		newLines = append(newLines, movedLines...)
		newLineEnds = append(newLineEnds, movedLineEnds...)
	}

	result.Lines = newLines
	result.LineEnds = newLineEnds

	// Collapse consecutive blank lines and strip trailing blanks.
	result.Lines, result.LineEnds = deleteCollapseBlankLines(result.Lines, result.LineEnds)
	result.Lines, result.LineEnds = deleteStripTrailingBlanks(result.Lines, result.LineEnds)

	return binder.Serialize(result)
}

// moveEvalSourceSelector finds source nodes matching selector.
// For selectors containing ":", path navigation via binder.EvalSelector is used.
// Otherwise a deep-tree search is performed with OPE006 code-fence detection.
// The root node is never a valid source: returns OPE001 with an explicit message.
func moveEvalSourceSelector(selector string, root *binder.Node, lines []string) ([]*binder.Node, []binder.Diagnostic) {
	const rootGuardMsg = "root node is not a valid target for this operation"
	if selector == "." {
		return nil, []binder.Diagnostic{{
			Severity: "error",
			Code:     binder.CodeSelectorNoMatch,
			Message:  rootGuardMsg,
		}}
	}
	if strings.Contains(selector, ":") {
		selResult, errDiags := binder.EvalSelector(selector, root)
		allDiags := append(selResult.Warnings, errDiags...)
		for _, n := range selResult.Nodes {
			if n.Type == "root" {
				return nil, append(allDiags, binder.Diagnostic{
					Severity: "error",
					Code:     binder.CodeSelectorNoMatch,
					Message:  rootGuardMsg,
				})
			}
		}
		return selResult.Nodes, allDiags
	}
	var matches []*binder.Node
	deleteSearchTree(root, selector, &matches)
	if len(matches) == 0 {
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
	var diags []binder.Diagnostic
	if len(matches) > 1 {
		diags = []binder.Diagnostic{{
			Severity: "warning",
			Code:     binder.CodeMultiMatch,
			Message:  fmt.Sprintf("selector %q matched %d nodes; operation applied to all matches", selector, len(matches)),
		}}
	}
	return matches, diags
}

// moveEvalDestSelector finds the destination parent node for a move operation.
// "." always returns the root node. Selectors containing ":" use path navigation.
func moveEvalDestSelector(selector string, root *binder.Node) (*binder.Node, []binder.Diagnostic) {
	if selector == "." {
		return root, nil
	}
	if strings.Contains(selector, ":") {
		selResult, errDiags := binder.EvalSelector(selector, root)
		var node *binder.Node
		if len(selResult.Nodes) > 0 {
			node = selResult.Nodes[0]
		}
		return node, append(selResult.Warnings, errDiags...)
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
	return matches[0], nil
}

// moveIsDescendant reports whether target is a descendant of ancestor.
func moveIsDescendant(ancestor, target *binder.Node) bool {
	for _, child := range ancestor.Children {
		if child == target || moveIsDescendant(child, target) {
			return true
		}
	}
	return false
}

// moveAnyParentLosesAllChildren reports whether any non-root parent would lose
// its only child due to the move.
func moveAnyParentLosesAllChildren(root *binder.Node, sourceNodes []*binder.Node) bool {
	for _, srcNode := range sourceNodes {
		parent := deleteFindParentNode(root, srcNode)
		if parent != nil && parent.Type != "root" && len(parent.Children) == 1 {
			return true
		}
	}
	return false
}

// moveNodeHasNonStructuralContent reports whether rawLine contains content
// before the structural inline link (after the list marker).
func moveNodeHasNonStructuralContent(rawLine string) bool {
	loc := deleteInlineLinkRE.FindStringIndex(rawLine)
	if loc == nil {
		return false
	}
	prefix := moveListMarkerRE.ReplaceAllString(rawLine[:loc[0]], "")
	return strings.TrimSpace(prefix) != ""
}

// moveReindentFirstLine adjusts the first line of a moved node: strips the
// original leading whitespace, original list marker, and any GFM checkbox,
// then prepends the target indent and target marker.
func moveReindentFirstLine(line string, srcIndentLen int, targetIndentStr, targetMarker string) string {
	// Strip leading whitespace.
	w := 0
	for w < len(line) && (line[w] == ' ' || line[w] == '\t') {
		w++
	}
	rest := line[w:]

	// Strip the original list marker (marker + space).
	if loc := moveFirstLineMarkerRE.FindStringIndex(rest); loc != nil {
		rest = rest[loc[1]:]
	}

	// Strip GFM task-list checkbox if present.
	if m := moveOpsCheckboxRE.FindString(rest); m != "" {
		rest = rest[len(m):]
	}

	return targetIndentStr + targetMarker + " " + rest
}

// moveReindentLine adjusts the leading whitespace of line so the moved node
// is indented at targetIndentStr plus any extra indent beyond srcIndentLen.
func moveReindentLine(line string, srcIndentLen int, targetIndentStr string) string {
	w := 0
	for w < len(line) && (line[w] == ' ' || line[w] == '\t') {
		w++
	}
	extra := max(w-srcIndentLen, 0)
	return targetIndentStr + strings.Repeat(" ", extra) + line[w:]
}
