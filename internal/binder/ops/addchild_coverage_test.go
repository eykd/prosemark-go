package ops

// Additional tests to achieve 100% coverage of addchild.go.
// These tests exercise helper functions directly and edge-case paths not
// reached by the primary acceptance-style tests in addchild_test.go.

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// ──────────────────────────────────────────────────────────────────────────────
// Parse error path (OPE009) via parseBinderFn mock
// ──────────────────────────────────────────────────────────────────────────────

// TestAddChild_ParseError_OPE009 verifies that when the underlying parser
// returns an error, AddChild propagates it and emits an OPE009 diagnostic
// without mutating the source bytes.
func TestAddChild_ParseError_OPE009(t *testing.T) {
	orig := parseBinderFn
	t.Cleanup(func() { parseBinderFn = orig })

	testErr := errors.New("mock parse failure")
	parseBinderFn = func(_ context.Context, _ []byte, _ *binder.Project) (*binder.ParseResult, []binder.Diagnostic, error) {
		return nil, nil, testErr
	}

	src := []byte("<!-- prosemark-binder:v1 -->\n\n- [Chapter One](chapter-one.md)\n")
	out, diags, err := AddChild(context.Background(), src, nil, binder.AddChildParams{
		ParentSelector: ".",
		Target:         "chapter-two.md",
		Title:          "Chapter Two",
		Position:       "last",
	})

	if err != testErr {
		t.Errorf("expected testErr, got %v", err)
	}
	if !hasDiagCode(diags, binder.CodeIOOrParseFailure) {
		t.Errorf("expected OPE009 diagnostic, got: %v", diags)
	}
	if !bytes.Equal(out, src) {
		t.Error("expected src unchanged on parse error")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// isSelectorInCodeFence: tilde fences and fence-close detection
// ──────────────────────────────────────────────────────────────────────────────

// TestIsSelectorInCodeFence_TildeAndBacktick verifies that tilde-delimited code
// fences are recognised and that the closing-fence detection allows the scanner
// to continue past a closed fence into a subsequent one.
// This covers:
//   - the "~~~" opening branch
//   - the closing-fence branch (inFence → prefix matches fenceMarker)
//   - fencedLineMatchesSelector returning false (non-link line inside fence)
func TestIsSelectorInCodeFence_TildeAndBacktick(t *testing.T) {
	lines := []string{
		"~~~",
		"not a link line",
		"~~~",
		"```",
		"- [selector](selector.md)",
		"```",
	}
	if !isSelectorInCodeFence(lines, "selector") {
		t.Error("expected true: selector is inside the backtick fence after the tilde fence")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tab indent for first child of a non-root node
// ──────────────────────────────────────────────────────────────────────────────

// TestAddChild_TabIndentFirstChild verifies that when a non-root node uses tab
// indentation and has no existing children, the new first child receives an
// additional level of tab indentation.
func TestAddChild_TabIndentFirstChild(t *testing.T) {
	// tab-leaf is a tab-indented leaf node with no children.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n- [Root](root.md)\n\t- [TabLeaf](tab-leaf.md)\n")
	params := binder.AddChildParams{
		ParentSelector: "root:tab-leaf",
		Target:         "sub-child.md",
		Title:          "Sub Child",
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	if !bytes.Contains(out, []byte("\t\t- [Sub Child](sub-child.md)")) {
		t.Errorf("expected double-tab-indented child in output:\n%q", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// rawIndent: Indent larger than RawLine length
// ──────────────────────────────────────────────────────────────────────────────

// TestRawIndent_IndentExceedsRawLine verifies the fallback path in rawIndent
// where the stored Indent count exceeds the actual RawLine length.
func TestRawIndent_IndentExceedsRawLine(t *testing.T) {
	n := &binder.Node{Indent: 5, RawLine: "ab"} // Indent > len(RawLine)
	got := rawIndent(n)
	want := "     " // 5 spaces
	if got != want {
		t.Errorf("rawIndent: got %q, want %q", got, want)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// isOrderedMarker: non-period/paren terminal and non-digit prefix
// ──────────────────────────────────────────────────────────────────────────────

// TestIsOrderedMarker_NonTerminal verifies that a marker whose last character
// is neither '.' nor ')' is rejected, and that a marker with a non-digit prefix
// followed by '.' is also rejected.
func TestIsOrderedMarker_NonTerminal(t *testing.T) {
	if isOrderedMarker("1a") {
		t.Error("expected false: '1a' does not end in '.' or ')'")
	}
	if isOrderedMarker("a.") {
		t.Error("expected false: 'a.' has a non-digit prefix")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ordinalValue: short marker and non-digit prefix
// ──────────────────────────────────────────────────────────────────────────────

// TestOrdinalValue_EdgeCases verifies that ordinalValue returns 0 for a
// single-character marker and for a marker with a non-digit prefix.
func TestOrdinalValue_EdgeCases(t *testing.T) {
	if v := ordinalValue("1"); v != 0 {
		t.Errorf("ordinalValue(\"1\"): got %d, want 0 (len < 2)", v)
	}
	if v := ordinalValue("a."); v != 0 {
		t.Errorf("ordinalValue(\"a.\"): got %d, want 0 (non-digit prefix)", v)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// subtreeLastLine: recursive case with grandchildren
// ──────────────────────────────────────────────────────────────────────────────

// TestSubtreeLastLine_WithGrandchildren verifies that subtreeLastLine recurses
// into grandchild nodes and returns the correct deepest line number.
func TestSubtreeLastLine_WithGrandchildren(t *testing.T) {
	n := &binder.Node{
		Line: 3,
		Children: []*binder.Node{
			{Line: 5, Children: []*binder.Node{
				{Line: 6},
				{Line: 7},
			}},
		},
	}
	got := subtreeLastLine(n)
	if got != 7 {
		t.Errorf("subtreeLastLine: got %d, want 7", got)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// majorityLineEnding: CRLF majority
// ──────────────────────────────────────────────────────────────────────────────

// TestMajorityLineEnding_CRLF verifies that when CRLF endings outnumber LF
// endings, majorityLineEnding returns "\r\n".
func TestMajorityLineEnding_CRLF(t *testing.T) {
	ends := []string{"\r\n", "\r\n", "\n"}
	got := majorityLineEnding(ends)
	if got != "\r\n" {
		t.Errorf("majorityLineEnding: got %q, want CRLF", got)
	}
}

// TestAddChild_CRLFLineEnding verifies that AddChild inserts new nodes using
// CRLF line endings when the source file uses CRLF majority endings.
func TestAddChild_CRLFLineEnding(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\r\n\r\n- [Chapter One](chapter-one.md)\r\n")
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "chapter-two.md",
		Title:          "Chapter Two",
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	idx := bytes.Index(out, []byte("chapter-two.md"))
	if idx < 0 {
		t.Fatalf("chapter-two.md missing from output:\n%q", out)
	}
	lineEnd := bytes.IndexByte(out[idx:], '\n')
	if lineEnd < 0 {
		t.Fatal("new node line has no LF terminator")
	}
	if lineEnd == 0 || out[idx+lineEnd-1] != '\r' {
		t.Errorf("expected CRLF for new node in CRLF-majority file:\n%q", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// opStemFromPath: path containing a directory separator
// ──────────────────────────────────────────────────────────────────────────────

// TestOpStemFromPath_Directory verifies that opStemFromPath strips the directory
// component when the path contains a slash.
func TestOpStemFromPath_Directory(t *testing.T) {
	got := opStemFromPath("sub/chapter.md")
	if got != "chapter" {
		t.Errorf("opStemFromPath: got %q, want \"chapter\"", got)
	}
}

// TestAddChild_EmptyTitle_DirectoryTarget verifies that when the target path
// contains a directory prefix and no title is supplied, the title is correctly
// derived from the file stem (without the directory component).
func TestAddChild_EmptyTitle_DirectoryTarget(t *testing.T) {
	src := binderSrc()
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "sub/chapter.md",
		Title:          "",
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	if !bytes.Contains(out, []byte("[chapter]")) {
		t.Errorf("expected stem-derived title 'chapter' in output:\n%q", out)
	}
}
