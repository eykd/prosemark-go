package binder_test

import (
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// makeTestNode creates a Node with Type "node", given target, title, and optional children.
func makeTestNode(target, title string, children ...*binder.Node) *binder.Node {
	c := make([]*binder.Node, len(children))
	copy(c, children)
	return &binder.Node{
		Type:     "node",
		Target:   target,
		Title:    title,
		Children: c,
	}
}

// makeTestRoot creates a synthetic root Node with the given children.
func makeTestRoot(children ...*binder.Node) *binder.Node {
	c := make([]*binder.Node, len(children))
	copy(c, children)
	return &binder.Node{
		Type:     "root",
		Children: c,
	}
}

// firstDiagCode returns the Code of the first Diagnostic matching the given severity,
// or "" if none is found.
func firstDiagCode(diags []binder.Diagnostic, severity string) string {
	for _, d := range diags {
		if d.Severity == severity {
			return d.Code
		}
	}
	return ""
}

// TestEvalSelector_Root verifies that the "." selector returns the synthetic root node.
func TestEvalSelector_Root(t *testing.T) {
	root := makeTestRoot(makeTestNode("intro.md", "Introduction"))

	result, diags := binder.EvalSelector(".", root)

	if firstDiagCode(diags, "error") != "" {
		t.Errorf("unexpected error diagnostic: %q", firstDiagCode(diags, "error"))
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(result.Nodes))
	}
	if result.Nodes[0].Type != "root" {
		t.Errorf("got node type %q, want %q", result.Nodes[0].Type, "root")
	}
}

// TestEvalSelector_BareStem_SingleMatch verifies that a bare stem matches a single node
// whose target basename (without .md extension) equals the stem.
func TestEvalSelector_BareStem_SingleMatch(t *testing.T) {
	root := makeTestRoot(
		makeTestNode("intro.md", "Introduction"),
		makeTestNode("conclusion.md", "Conclusion"),
	)

	result, diags := binder.EvalSelector("intro", root)

	if firstDiagCode(diags, "error") != "" {
		t.Errorf("unexpected error diagnostic: %q", firstDiagCode(diags, "error"))
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(result.Nodes))
	}
	if result.Nodes[0].Target != "intro.md" {
		t.Errorf("got target %q, want %q", result.Nodes[0].Target, "intro.md")
	}
}

// TestEvalSelector_BareStem_MatchesSubdirTarget verifies that a bare stem matches a node
// whose target is in a subdirectory (e.g., "chapter" matches "part1/chapter.md").
func TestEvalSelector_BareStem_MatchesSubdirTarget(t *testing.T) {
	root := makeTestRoot(
		makeTestNode("intro.md", "Introduction"),
		makeTestNode("part1/chapter.md", "Chapter 1"),
	)

	result, diags := binder.EvalSelector("chapter", root)

	if firstDiagCode(diags, "error") != "" {
		t.Errorf("unexpected error diagnostic: %q", firstDiagCode(diags, "error"))
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(result.Nodes))
	}
	if result.Nodes[0].Target != "part1/chapter.md" {
		t.Errorf("got target %q, want %q", result.Nodes[0].Target, "part1/chapter.md")
	}
}

// TestEvalSelector_RelativePath verifies that a relative path selector (containing "/")
// matches a node whose target equals the relative path with ".md" appended.
func TestEvalSelector_RelativePath(t *testing.T) {
	root := makeTestRoot(
		makeTestNode("part1/chapter.md", "Chapter 1"),
		makeTestNode("part2/chapter.md", "Chapter 2"),
	)

	result, diags := binder.EvalSelector("part1/chapter", root)

	if firstDiagCode(diags, "error") != "" {
		t.Errorf("unexpected error diagnostic: %q", firstDiagCode(diags, "error"))
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(result.Nodes))
	}
	if result.Nodes[0].Target != "part1/chapter.md" {
		t.Errorf("got target %q, want %q", result.Nodes[0].Target, "part1/chapter.md")
	}
}

// TestEvalSelector_ZeroMatch_OPE001 verifies that a selector matching no nodes
// returns an OPE001 error diagnostic and an empty Nodes slice.
func TestEvalSelector_ZeroMatch_OPE001(t *testing.T) {
	root := makeTestRoot(
		makeTestNode("intro.md", "Introduction"),
	)

	result, diags := binder.EvalSelector("notexist", root)

	if len(result.Nodes) != 0 {
		t.Errorf("got %d nodes, want 0", len(result.Nodes))
	}
	got := firstDiagCode(diags, "error")
	if got != binder.CodeSelectorNoMatch {
		t.Errorf("got error code %q, want %q", got, binder.CodeSelectorNoMatch)
	}
}

// TestEvalSelector_MultiMatch_OPW001 verifies that when the same selector matches
// multiple nodes in the tree, an OPW001 warning is emitted and all matched nodes are returned.
func TestEvalSelector_MultiMatch_OPW001(t *testing.T) {
	// Same target referenced at two sibling positions.
	root := makeTestRoot(
		makeTestNode("chapter.md", "Chapter (first ref)"),
		makeTestNode("chapter.md", "Chapter (second ref)"),
		makeTestNode("conclusion.md", "Conclusion"),
	)

	result, diags := binder.EvalSelector("chapter", root)

	// No fatal error diagnostic.
	if firstDiagCode(diags, "error") != "" {
		t.Errorf("unexpected error diagnostic: %q", firstDiagCode(diags, "error"))
	}
	// Multi-match warning must be emitted.
	got := firstDiagCode(result.Warnings, "warning")
	if got != binder.CodeMultiMatch {
		t.Errorf("got warning code %q, want %q", got, binder.CodeMultiMatch)
	}
	// All matched nodes must be returned.
	if len(result.Nodes) < 2 {
		t.Errorf("got %d nodes, want at least 2", len(result.Nodes))
	}
}

// TestEvalSelector_AmbiguousBareStem_OPE002 verifies that a bare stem matching nodes
// with different targets that share the same basename returns OPE002 (ambiguous).
func TestEvalSelector_AmbiguousBareStem_OPE002(t *testing.T) {
	// Two nodes with same basename "chapter" but different directory paths.
	root := makeTestRoot(
		makeTestNode("part1/chapter.md", "Chapter 1"),
		makeTestNode("part2/chapter.md", "Chapter 2"),
	)

	result, diags := binder.EvalSelector("chapter", root)

	if len(result.Nodes) != 0 {
		t.Errorf("got %d nodes, want 0", len(result.Nodes))
	}
	got := firstDiagCode(diags, "error")
	if got != binder.CodeAmbiguousBareStem {
		t.Errorf("got error code %q, want %q", got, binder.CodeAmbiguousBareStem)
	}
}

// TestEvalSelector_IndexQualified verifies that a "[N]" index qualifier selects
// the Nth (0-based) match from the set of nodes matching the FileRef.
func TestEvalSelector_IndexQualified(t *testing.T) {
	// Same target referenced twice; index selects among them.
	root := makeTestRoot(
		makeTestNode("chapter.md", "Chapter (first)"),
		makeTestNode("chapter.md", "Chapter (second)"),
		makeTestNode("intro.md", "Introduction"),
	)

	tests := []struct {
		name        string
		selector    string
		wantTitle   string
		wantErrCode string
	}{
		{
			name:      "index[0] selects first match",
			selector:  "chapter[0]",
			wantTitle: "Chapter (first)",
		},
		{
			name:      "index[1] selects second match",
			selector:  "chapter[1]",
			wantTitle: "Chapter (second)",
		},
		{
			name:        "index out of range returns OPE001",
			selector:    "chapter[5]",
			wantErrCode: binder.CodeSelectorNoMatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags := binder.EvalSelector(tt.selector, root)

			errCode := firstDiagCode(diags, "error")
			if tt.wantErrCode != "" {
				if errCode != tt.wantErrCode {
					t.Errorf("got error code %q, want %q", errCode, tt.wantErrCode)
				}
				return
			}

			if errCode != "" {
				t.Errorf("unexpected error diagnostic: %q", errCode)
			}
			if len(result.Nodes) != 1 {
				t.Fatalf("got %d nodes, want 1", len(result.Nodes))
			}
			if result.Nodes[0].Title != tt.wantTitle {
				t.Errorf("got title %q, want %q", result.Nodes[0].Title, tt.wantTitle)
			}
		})
	}
}

// TestEvalSelector_MultiSegment_ColonDescent verifies that a colon-separated selector
// evaluates the second segment among the children of the first segment's match.
func TestEvalSelector_MultiSegment_ColonDescent(t *testing.T) {
	section := makeTestNode("part1/section.md", "Section")
	chapter := makeTestNode("part1/chapter.md", "Chapter 1", section)
	root := makeTestRoot(
		makeTestNode("intro.md", "Introduction"),
		chapter,
		makeTestNode("conclusion.md", "Conclusion"),
	)

	result, diags := binder.EvalSelector("chapter:section", root)

	if firstDiagCode(diags, "error") != "" {
		t.Errorf("unexpected error diagnostic: %q", firstDiagCode(diags, "error"))
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(result.Nodes))
	}
	if result.Nodes[0].Target != "part1/section.md" {
		t.Errorf("got target %q, want %q", result.Nodes[0].Target, "part1/section.md")
	}
}

// TestEvalSelector_MultiSegment_ZeroMatchMidPath verifies that OPE001 is returned
// when a colon-chained second segment matches no children of the first result.
func TestEvalSelector_MultiSegment_ZeroMatchMidPath(t *testing.T) {
	chapter := makeTestNode("part1/chapter.md", "Chapter 1",
		makeTestNode("part1/section.md", "Section"),
	)
	root := makeTestRoot(chapter)

	result, diags := binder.EvalSelector("chapter:notexist", root)

	if len(result.Nodes) != 0 {
		t.Errorf("got %d nodes, want 0", len(result.Nodes))
	}
	got := firstDiagCode(diags, "error")
	if got != binder.CodeSelectorNoMatch {
		t.Errorf("got error code %q, want %q", got, binder.CodeSelectorNoMatch)
	}
}

// TestEvalSelector_MultiSegment_ThreeLevels verifies three-level colon descent works.
func TestEvalSelector_MultiSegment_ThreeLevels(t *testing.T) {
	subsection := makeTestNode("ch/sub/subsection.md", "Subsection")
	section := makeTestNode("ch/section.md", "Section", subsection)
	chapter := makeTestNode("ch/chapter.md", "Chapter", section)
	root := makeTestRoot(chapter)

	result, diags := binder.EvalSelector("chapter:section:subsection", root)

	if firstDiagCode(diags, "error") != "" {
		t.Errorf("unexpected error diagnostic: %q", firstDiagCode(diags, "error"))
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(result.Nodes))
	}
	if result.Nodes[0].Target != "ch/sub/subsection.md" {
		t.Errorf("got target %q, want %q", result.Nodes[0].Target, "ch/sub/subsection.md")
	}
}

// TestEvalSelector_Root_IsNotAmbiguousOnBareStemConflict verifies that when a bare
// stem is used with the root selector as a prefix, the root segment resolves correctly.
func TestEvalSelector_DotRootPrefix_DescendsToChild(t *testing.T) {
	chapter := makeTestNode("chapter.md", "Chapter")
	root := makeTestRoot(chapter)

	// ".:chapter" means: from root, find child with stem "chapter"
	result, diags := binder.EvalSelector(".:chapter", root)

	if firstDiagCode(diags, "error") != "" {
		t.Errorf("unexpected error diagnostic: %q", firstDiagCode(diags, "error"))
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(result.Nodes))
	}
	if result.Nodes[0].Target != "chapter.md" {
		t.Errorf("got target %q, want %q", result.Nodes[0].Target, "chapter.md")
	}
}

// TestEvalSelector_IndexQualified_WithColonDescent verifies index qualification combined
// with colon descent works correctly.
func TestEvalSelector_IndexQualified_WithColonDescent(t *testing.T) {
	section1 := makeTestNode("p1/section.md", "Section")
	chapter1 := makeTestNode("chapter.md", "Chapter 1 (first)", section1)
	chapter2 := makeTestNode("chapter.md", "Chapter 2 (second)")
	root := makeTestRoot(chapter1, chapter2)

	// "chapter[0]:section" → children of first "chapter" match → find "section" there
	result, diags := binder.EvalSelector("chapter[0]:section", root)

	if firstDiagCode(diags, "error") != "" {
		t.Errorf("unexpected error diagnostic: %q", firstDiagCode(diags, "error"))
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(result.Nodes))
	}
	if result.Nodes[0].Target != "p1/section.md" {
		t.Errorf("got target %q, want %q", result.Nodes[0].Target, "p1/section.md")
	}
}
