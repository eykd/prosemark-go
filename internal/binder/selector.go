package binder

import (
	"fmt"
	"regexp"
	"strings"
)

// selectorIndexRE matches a selector segment with an optional [N] index qualifier.
var selectorIndexRE = regexp.MustCompile(`^(.*)\[(\d+)\]$`)

// EvalSelector evaluates a selector expression against the given root node.
// Fatal errors (OPE001, OPE002) are returned in the []Diagnostic return value.
// Warnings (OPW001) are returned in SelectorResult.Warnings.
func EvalSelector(selector string, root *Node) (SelectorResult, []Diagnostic) {
	segments := strings.Split(selector, ":")

	var result SelectorResult
	currentNodes := []*Node{root}

	for _, seg := range segments {
		if seg == "." {
			continue
		}

		var children []*Node
		for _, n := range currentNodes {
			children = append(children, n.Children...)
		}

		fileRef, idx := parseSegment(seg)
		matches, warnings, errDiags := selectorMatchNodes(fileRef, children)
		if errDiags != nil {
			return SelectorResult{}, errDiags
		}
		result.Warnings = append(result.Warnings, warnings...)

		nodes, errDiags := applyIndex(fileRef, matches, idx)
		if errDiags != nil {
			return SelectorResult{}, errDiags
		}
		currentNodes = nodes
	}

	result.Nodes = currentNodes
	return result, nil
}

// parseSegment splits a selector segment into its file-ref and optional 0-based index.
// Returns idx == -1 if no [N] qualifier is present.
func parseSegment(seg string) (string, int) {
	m := selectorIndexRE.FindStringSubmatch(seg)
	if m == nil {
		return seg, -1
	}
	idx := 0
	for i := 0; i < len(m[2]); i++ {
		idx = idx*10 + int(m[2][i]-'0')
	}
	return m[1], idx
}

// applyIndex returns the Nth match (0-based) when idx >= 0, or all matches when idx == -1.
// Returns OPE001 if the index is out of range.
func applyIndex(fileRef string, matches []*Node, idx int) ([]*Node, []Diagnostic) {
	if idx < 0 {
		return matches, nil
	}
	if idx >= len(matches) {
		return nil, []Diagnostic{newSelectorDiag("error", CodeSelectorNoMatch,
			fmt.Sprintf("selector %q index [%d] out of range: %d matches found", fileRef, idx, len(matches)))}
	}
	return []*Node{matches[idx]}, nil
}

// selectorMatchNodes finds all nodes in the flat list whose target matches fileRef.
// Returns matching nodes, OPW001 warnings, and any fatal OPE001/OPE002 diagnostics.
func selectorMatchNodes(fileRef string, nodes []*Node) ([]*Node, []Diagnostic, []Diagnostic) {
	var matches []*Node
	for _, n := range nodes {
		if nodeMatchesSelector(fileRef, n) {
			matches = append(matches, n)
		}
	}

	if len(matches) == 0 {
		return nil, nil, []Diagnostic{newSelectorDiag("error", CodeSelectorNoMatch,
			fmt.Sprintf("selector %q matched no nodes", fileRef))}
	}

	firstTarget := matches[0].Target
	for _, m := range matches[1:] {
		if m.Target != firstTarget {
			return nil, nil, []Diagnostic{newSelectorDiag("error", CodeAmbiguousBareStem,
				fmt.Sprintf("selector %q is ambiguous: matches nodes with different targets", fileRef))}
		}
	}

	if len(matches) > 1 {
		w := newSelectorDiag("warning", CodeMultiMatch,
			fmt.Sprintf("selector %q matched %d nodes with the same target", fileRef, len(matches)))
		return matches, []Diagnostic{w}, nil
	}

	return matches, nil, nil
}

// nodeMatchesSelector reports whether n's target matches fileRef.
// A fileRef containing "/" is matched as a relative path (with or without .md extension).
// Otherwise matched as a bare stem, a direct target name, or a case-insensitive title match.
func nodeMatchesSelector(fileRef string, n *Node) bool {
	if strings.Contains(fileRef, "/") {
		return n.Target == fileRef || n.Target == fileRef+".md"
	}
	return stemFromPath(n.Target) == fileRef ||
		n.Target == fileRef ||
		strings.EqualFold(n.Title, fileRef)
}

// newSelectorDiag constructs a Diagnostic with no source location.
func newSelectorDiag(severity, code, message string) Diagnostic {
	return Diagnostic{Severity: severity, Code: code, Message: message}
}
