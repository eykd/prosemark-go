package ops

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/eykd/prosemark-go/internal/binder"
)

// opsInlineLinkRE matches inline markdown links anywhere in a line.
var opsInlineLinkRE = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s"]+)`)

// parseBinderFn is the parse function used by AddChild. It may be replaced in
// tests to simulate parse failures.
var parseBinderFn = binder.Parse

// AddChild inserts a new child node into the binder at the specified position under
// the parent selected by params.ParentSelector. Returns the modified binder bytes,
// diagnostics, and any fatal error. On validation or logical error the returned
// bytes are equal to src (no mutation).
func AddChild(ctx context.Context, src []byte, project *binder.Project, params binder.AddChildParams) ([]byte, []binder.Diagnostic, error) {
	result, parseDiags, err := parseBinderFn(ctx, src, project)
	if err != nil {
		return src, append(parseDiags, binder.Diagnostic{
			Severity: "error",
			Code:     binder.CodeIOOrParseFailure,
			Message:  fmt.Sprintf("parse error: %v", err),
		}), err
	}

	// Validate target path (OPE004, OPE005) before touching the selector.
	if diag := validateOpTarget(params.Target); diag != nil {
		return src, append(parseDiags, *diag), nil
	}

	// Evaluate the parent selector.
	selResult, errDiags := binder.EvalSelector(params.ParentSelector, result.Root)
	if errDiags != nil {
		// OPE001 may be upgraded to OPE006 when the node exists inside a code fence.
		for _, d := range errDiags {
			if d.Code == binder.CodeSelectorNoMatch {
				if isSelectorInCodeFence(result.Lines, params.ParentSelector) {
					return src, append(parseDiags, binder.Diagnostic{
						Severity: "error",
						Code:     binder.CodeNodeInCodeFence,
						Message:  fmt.Sprintf("selector %q matches a node inside a code fence", params.ParentSelector),
					}), nil
				}
			}
		}
		return src, append(parseDiags, errDiags...), nil
	}

	parent := selResult.Nodes[0]
	var allDiags []binder.Diagnostic
	allDiags = append(allDiags, parseDiags...)
	allDiags = append(allDiags, selResult.Warnings...)

	// Idempotency check (OPW002): skip if target already exists as a direct child.
	if !params.Force {
		for _, child := range parent.Children {
			if child.Target == params.Target {
				return src, append(allDiags, binder.Diagnostic{
					Severity: "warning",
					Code:     binder.CodeDuplicateSkipped,
					Message:  fmt.Sprintf("target %q already exists as a child; skipping (use --force to override)", params.Target),
				}), nil
			}
		}
	}

	// Resolve insertion index among parent's children.
	insertIdx, diagErr := resolveInsertionIndex(parent, params)
	if diagErr != nil {
		return src, append(allDiags, *diagErr), nil
	}

	// Derive title from stem when empty.
	title := params.Title
	if title == "" {
		title = opStemFromPath(params.Target)
	}
	title = escapeTitle(title)

	// Build the new list-item line.
	indentStr, marker := inferMarkerAndIndent(parent)
	newLine := indentStr + marker + " [" + title + "](" + params.Target + ")"

	// Determine line ending from the file's majority style.
	lineEnd := majorityLineEnding(result.LineEnds)

	// Find the 0-based position in result.Lines at which to insert.
	lineIdx := insertionLineIdx(parent, insertIdx, result)

	// Splice the new line into the ParseResult.
	result.Lines = sliceInsert(result.Lines, lineIdx, newLine)
	result.LineEnds = sliceInsert(result.LineEnds, lineIdx, lineEnd)

	return binder.Serialize(result), allDiags, nil
}

// validateOpTarget checks OPE004 (path escapes root) and OPE005 (target is binder).
func validateOpTarget(target string) *binder.Diagnostic {
	if opEscapesRoot(target) {
		return &binder.Diagnostic{
			Severity: "error",
			Code:     binder.CodeInvalidTargetPath,
			Message:  "target path escapes the project root",
		}
	}
	if target == "_binder.md" {
		return &binder.Diagnostic{
			Severity: "error",
			Code:     binder.CodeTargetIsBinder,
			Message:  "target is the binder file itself",
		}
	}
	return nil
}

// opEscapesRoot reports whether path escapes the project root via "..".
func opEscapesRoot(path string) bool {
	return path == ".." || strings.HasPrefix(path, "../")
}

// isSelectorInCodeFence reports whether any fenced code block in lines contains
// an inline link whose target stem matches the bare-stem selector.
func isSelectorInCodeFence(lines []string, selector string) bool {
	inFence := false
	fenceMarker := ""
	for _, line := range lines {
		if !inFence {
			if strings.HasPrefix(line, "```") {
				inFence, fenceMarker = true, "```"
			} else if strings.HasPrefix(line, "~~~") {
				inFence, fenceMarker = true, "~~~"
			}
		} else {
			if strings.HasPrefix(line, fenceMarker) {
				inFence, fenceMarker = false, ""
				continue
			}
			if fencedLineMatchesSelector(line, selector) {
				return true
			}
		}
	}
	return false
}

// fencedLineMatchesSelector checks whether a line inside a code fence contains an
// inline link whose target stem equals the bare-stem selector.
func fencedLineMatchesSelector(line, selector string) bool {
	m := opsInlineLinkRE.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	target := m[2]
	return opStemFromPath(target) == selector || target == selector || target == selector+".md"
}

// resolveInsertionIndex returns the 0-based index in parent.Children at which to
// insert the new node, or an error diagnostic if the position spec is invalid.
func resolveInsertionIndex(parent *binder.Node, params binder.AddChildParams) (int, *binder.Diagnostic) {
	n := len(parent.Children)

	if params.At != nil {
		idx := *params.At
		if idx < 0 || idx > n {
			return 0, &binder.Diagnostic{
				Severity: "error",
				Code:     binder.CodeIndexOutOfBounds,
				Message:  fmt.Sprintf("at index %d out of bounds: %d children", idx, n),
			}
		}
		return idx, nil
	}

	if params.Before != "" {
		i := findSiblingIndex(parent.Children, params.Before)
		if i < 0 {
			return 0, &binder.Diagnostic{
				Severity: "error",
				Code:     binder.CodeSiblingNotFound,
				Message:  fmt.Sprintf("before-sibling %q not found", params.Before),
			}
		}
		return i, nil
	}

	if params.After != "" {
		i := findSiblingIndex(parent.Children, params.After)
		if i < 0 {
			return 0, &binder.Diagnostic{
				Severity: "error",
				Code:     binder.CodeSiblingNotFound,
				Message:  fmt.Sprintf("after-sibling %q not found", params.After),
			}
		}
		return i + 1, nil
	}

	if params.Position == "first" {
		return 0, nil
	}

	return n, nil
}

// findSiblingIndex returns the 0-based index of the first child whose target
// matches selector, or -1 if no child matches.
func findSiblingIndex(children []*binder.Node, selector string) int {
	for i, child := range children {
		if siblingMatchesSelector(child, selector) {
			return i
		}
	}
	return -1
}

// siblingMatchesSelector reports whether a child node's target matches a bare-stem selector.
func siblingMatchesSelector(child *binder.Node, selector string) bool {
	return opStemFromPath(child.Target) == selector ||
		child.Target == selector ||
		child.Target == selector+".md"
}

// inferMarkerAndIndent returns the indentation string and list marker to use for a new
// child of parent, inherited from existing siblings or defaulted for first children.
func inferMarkerAndIndent(parent *binder.Node) (indentStr, marker string) {
	if len(parent.Children) > 0 {
		sibling := parent.Children[0]
		indentStr = rawIndent(sibling)
		marker = sibling.ListMarker
		if isOrderedMarker(marker) {
			style := orderedStyle(marker)
			marker = fmt.Sprintf("%d%s", maxOrdinal(parent.Children)+1, style)
		}
		return
	}

	if parent.Type == "root" {
		return "", "-"
	}

	// First child of a non-root node: one indent level deeper than parent.
	pIndent := rawIndent(parent)
	if len(pIndent) > 0 && pIndent[0] == '\t' {
		indentStr = pIndent + "\t"
	} else {
		indentStr = pIndent + "  "
	}
	return indentStr, "-"
}

// rawIndent extracts the leading whitespace characters from a node's source line.
func rawIndent(n *binder.Node) string {
	if n.Indent == 0 || len(n.RawLine) == 0 {
		return ""
	}
	if n.Indent <= len(n.RawLine) {
		return n.RawLine[:n.Indent]
	}
	return strings.Repeat(" ", n.Indent)
}

// isOrderedMarker reports whether marker is an ordered list marker (e.g., "1.", "2)").
func isOrderedMarker(marker string) bool {
	if len(marker) < 2 {
		return false
	}
	last := marker[len(marker)-1]
	if last != '.' && last != ')' {
		return false
	}
	for i := 0; i < len(marker)-1; i++ {
		if marker[i] < '0' || marker[i] > '9' {
			return false
		}
	}
	return true
}

// orderedStyle returns the trailing style character ("." or ")") of an ordered marker.
func orderedStyle(marker string) string {
	return string(marker[len(marker)-1])
}

// maxOrdinal returns the maximum numeric ordinal found among nodes' list markers.
func maxOrdinal(nodes []*binder.Node) int {
	maxVal := 0
	for _, n := range nodes {
		if v := ordinalValue(n.ListMarker); v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

// ordinalValue extracts the integer ordinal from an ordered marker like "2." or "3)".
func ordinalValue(marker string) int {
	if len(marker) < 2 {
		return 0
	}
	n := 0
	for i := 0; i < len(marker)-1; i++ {
		c := marker[i]
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// insertionLineIdx returns the 0-based index into result.Lines at which to insert
// the new node's line.
func insertionLineIdx(parent *binder.Node, insertIdx int, result *binder.ParseResult) int {
	if insertIdx < len(parent.Children) {
		// Insert before the child at insertIdx.
		return parent.Children[insertIdx].Line - 1
	}
	// Insert after all children.
	if len(parent.Children) == 0 {
		if parent.Type == "root" {
			return len(result.Lines)
		}
		// After the parent's own line (Line is 1-based; insert at that 0-indexed position).
		return parent.Line
	}
	// After the last child's entire subtree.
	return subtreeLastLine(parent.Children[len(parent.Children)-1])
}

// subtreeLastLine returns the maximum 1-based line number across all nodes in
// the subtree rooted at n. Used as the 0-based insert position (= insert after line n).
func subtreeLastLine(n *binder.Node) int {
	last := n.Line
	for _, c := range n.Children {
		if l := subtreeLastLine(c); l > last {
			last = l
		}
	}
	return last
}

// majorityLineEnding returns the most common line ending found in ends, defaulting to "\n".
func majorityLineEnding(ends []string) string {
	crlf := 0
	lf := 0
	for _, e := range ends {
		switch e {
		case "\r\n":
			crlf++
		case "\n":
			lf++
		}
	}
	if crlf > lf {
		return "\r\n"
	}
	return "\n"
}

// escapeTitle backslash-escapes '[' and ']' in a title string.
func escapeTitle(title string) string {
	title = strings.ReplaceAll(title, "[", `\[`)
	title = strings.ReplaceAll(title, "]", `\]`)
	return title
}

// opStemFromPath extracts the filename stem (basename without last extension).
func opStemFromPath(p string) string {
	if idx := strings.LastIndex(p, "/"); idx >= 0 {
		p = p[idx+1:]
	}
	if idx := strings.LastIndex(p, "."); idx >= 0 {
		p = p[:idx]
	}
	return p
}

// sliceInsert returns a new slice with v inserted at position idx.
func sliceInsert(s []string, idx int, v string) []string {
	out := make([]string, len(s)+1)
	copy(out, s[:idx])
	out[idx] = v
	copy(out[idx+1:], s[idx:])
	return out
}
