package binder_test

import (
	"context"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestParse_BOM_StrippedAndDiagnostic tests that a UTF-8 BOM is stripped from input
// and a BNDW010 warning is emitted.
func TestParse_BOM_StrippedAndDiagnostic(t *testing.T) {
	// UTF-8 BOM = EF BB BF
	bom := "\xef\xbb\xbf"
	src := []byte(bom + "<!-- prosemark-binder:v1 -->\n- [Chapter](chapter.md)\n")

	result, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasBOM {
		t.Error("ParseResult.HasBOM must be true when input has UTF-8 BOM")
	}
	// The content lines must not start with BOM bytes
	if len(result.Lines) > 0 && len(result.Lines[0]) > 0 && result.Lines[0][0] == '\xef' {
		t.Error("first line must not contain BOM bytes after stripping")
	}
	// BNDW010 warning must be emitted
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeBOMPresence {
			found = true
			if d.Severity != "warning" {
				t.Errorf("BNDW010 severity = %q, want %q", d.Severity, "warning")
			}
		}
	}
	if !found {
		t.Error("expected BNDW010 diagnostic when BOM present, got none")
	}
}

// TestParse_NoBOM_NoBOMDiagnostic tests that no BNDW010 is emitted when BOM is absent.
func TestParse_NoBOM_NoBOMDiagnostic(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Chapter](chapter.md)\n")

	result, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasBOM {
		t.Error("ParseResult.HasBOM must be false when no BOM present")
	}
	for _, d := range diags {
		if d.Code == binder.CodeBOMPresence {
			t.Errorf("unexpected BNDW010 diagnostic when no BOM: %+v", d)
		}
	}
}

// TestParse_LineEndings tests that LF, CRLF, and bare-CR line endings are detected per line.
func TestParse_LineEndings(t *testing.T) {
	tests := []struct {
		name      string
		src       []byte
		wantEnd   string
		lineCount int
	}{
		{
			name:      "LF line endings",
			src:       []byte("<!-- prosemark-binder:v1 -->\n- [A](a.md)\n- [B](b.md)\n"),
			wantEnd:   "\n",
			lineCount: 3,
		},
		{
			name:      "CRLF line endings",
			src:       []byte("<!-- prosemark-binder:v1 -->\r\n- [A](a.md)\r\n- [B](b.md)\r\n"),
			wantEnd:   "\r\n",
			lineCount: 3,
		},
		{
			name:      "bare-CR line endings",
			src:       []byte("<!-- prosemark-binder:v1 -->\r- [A](a.md)\r- [B](b.md)\r"),
			wantEnd:   "\r",
			lineCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := binder.Parse(context.Background(), tt.src, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.LineEnds) != tt.lineCount {
				t.Errorf("len(LineEnds) = %d, want %d", len(result.LineEnds), tt.lineCount)
				return
			}
			for i, end := range result.LineEnds {
				if end != tt.wantEnd {
					t.Errorf("LineEnds[%d] = %q, want %q", i, end, tt.wantEnd)
				}
			}
		})
	}
}

// TestParse_LineEndings_LastLineNoEnding tests that a file with no trailing newline
// has an empty string as the last line ending.
func TestParse_LineEndings_LastLineNoEnding(t *testing.T) {
	// Last line has no line ending
	src := []byte("<!-- prosemark-binder:v1 -->\n- [A](a.md)")

	result, _, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.LineEnds) < 2 {
		t.Fatalf("len(LineEnds) = %d, want >= 2", len(result.LineEnds))
	}
	if result.LineEnds[0] != "\n" {
		t.Errorf("LineEnds[0] = %q, want %q", result.LineEnds[0], "\n")
	}
	if result.LineEnds[1] != "" {
		t.Errorf("LineEnds[1] (no trailing newline) = %q, want %q", result.LineEnds[1], "")
	}
}

// TestParse_MissingPragma_EmitsBNDW001 tests that BNDW001 is emitted when pragma is absent.
func TestParse_MissingPragma_EmitsBNDW001(t *testing.T) {
	src := []byte("- [Chapter](chapter.md)\n")

	result, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasPragma {
		t.Error("ParseResult.HasPragma must be false when pragma absent")
	}
	if result.PragmaLine != 0 {
		t.Errorf("ParseResult.PragmaLine = %d, want 0 when absent", result.PragmaLine)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeMissingPragma {
			found = true
			if d.Severity != "warning" {
				t.Errorf("BNDW001 severity = %q, want %q", d.Severity, "warning")
			}
		}
	}
	if !found {
		t.Error("expected BNDW001 diagnostic when pragma absent, got none")
	}
}

// TestParse_PragmaPresent_RecordedCorrectly tests that pragma is detected and recorded.
func TestParse_PragmaPresent_RecordedCorrectly(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Chapter](chapter.md)\n")

	result, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasPragma {
		t.Error("ParseResult.HasPragma must be true when pragma present")
	}
	if result.PragmaLine != 1 {
		t.Errorf("ParseResult.PragmaLine = %d, want 1", result.PragmaLine)
	}
	for _, d := range diags {
		if d.Code == binder.CodeMissingPragma {
			t.Errorf("unexpected BNDW001 diagnostic when pragma present: %+v", d)
		}
	}
}

// TestParse_PragmaInsideFence_NotCounted tests that pragma inside a fenced code block
// is not counted as the pragma (M5 edge case from plan.md).
func TestParse_PragmaInsideFence_NotCounted(t *testing.T) {
	tests := []struct {
		name string
		src  []byte
	}{
		{
			name: "pragma inside backtick fence",
			src:  []byte("```\n<!-- prosemark-binder:v1 -->\n```\n- [Chapter](chapter.md)\n"),
		},
		{
			name: "pragma inside tilde fence",
			src:  []byte("~~~\n<!-- prosemark-binder:v1 -->\n~~~\n- [Chapter](chapter.md)\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags, err := binder.Parse(context.Background(), tt.src, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.HasPragma {
				t.Error("ParseResult.HasPragma must be false when pragma is inside a code fence")
			}
			// BNDW001 must be emitted because the effective pragma is absent
			found := false
			for _, d := range diags {
				if d.Code == binder.CodeMissingPragma {
					found = true
				}
			}
			if !found {
				t.Error("expected BNDW001 diagnostic when pragma is inside fence, got none")
			}
		})
	}
}

// TestParse_BacktickFence_LinksExcluded tests that links inside backtick fences emit BNDW005.
func TestParse_BacktickFence_LinksExcluded(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n```\n- [Fenced](fenced.md)\n```\n- [Real](real.md)\n")

	_, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeLinkInCodeFence {
			found = true
			if d.Severity != "warning" {
				t.Errorf("BNDW005 severity = %q, want %q", d.Severity, "warning")
			}
		}
	}
	if !found {
		t.Error("expected BNDW005 diagnostic for link inside backtick fence, got none")
	}
}

// TestParse_TildeFence_LinksExcluded tests that links inside tilde fences emit BNDW005.
func TestParse_TildeFence_LinksExcluded(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n~~~\n- [Fenced](fenced.md)\n~~~\n- [Real](real.md)\n")

	_, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeLinkInCodeFence {
			found = true
			if d.Severity != "warning" {
				t.Errorf("BNDW005 severity = %q, want %q", d.Severity, "warning")
			}
		}
	}
	if !found {
		t.Error("expected BNDW005 diagnostic for link inside tilde fence, got none")
	}
}

// TestParse_FenceOpenClose_LinkAfterFenceAllowed tests that links outside a closed fence
// do NOT produce BNDW005.
func TestParse_FenceOpenClose_LinkAfterFenceAllowed(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n```\nsome code\n```\n- [Real](real.md)\n")

	_, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, d := range diags {
		if d.Code == binder.CodeLinkInCodeFence {
			t.Errorf("unexpected BNDW005 for link outside fence: %+v", d)
		}
	}
}

// TestParse_EmptyInput tests that a zero-byte input is handled gracefully (M1 edge case).
func TestParse_EmptyInput(t *testing.T) {
	result, diags, err := binder.Parse(context.Background(), []byte{}, nil)
	if err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
	if result == nil {
		t.Fatal("ParseResult must not be nil on empty input")
	}
	if result.Root == nil {
		t.Error("ParseResult.Root must not be nil on empty input")
	}
	if result.Root.Type != "root" {
		t.Errorf("Root.Type = %q, want %q", result.Root.Type, "root")
	}
	if result.Root.Children == nil {
		t.Error("Root.Children must not be nil (empty slice)")
	}
	if result.Version != "1" {
		t.Errorf("ParseResult.Version = %q, want %q", result.Version, "1")
	}
	// BNDW001 must be emitted (missing pragma on empty input)
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeMissingPragma {
			found = true
		}
	}
	if !found {
		t.Error("expected BNDW001 for empty input (no pragma)")
	}
}

// TestParse_BOMOnly tests that a file containing only a BOM is handled gracefully.
func TestParse_BOMOnly(t *testing.T) {
	bom := []byte{0xef, 0xbb, 0xbf}
	result, diags, err := binder.Parse(context.Background(), bom, nil)
	if err != nil {
		t.Fatalf("unexpected error on BOM-only input: %v", err)
	}
	if !result.HasBOM {
		t.Error("ParseResult.HasBOM must be true")
	}
	hasBOMDiag := false
	hasMissingPragma := false
	for _, d := range diags {
		if d.Code == binder.CodeBOMPresence {
			hasBOMDiag = true
		}
		if d.Code == binder.CodeMissingPragma {
			hasMissingPragma = true
		}
	}
	if !hasBOMDiag {
		t.Error("expected BNDW010 for BOM-only input")
	}
	if !hasMissingPragma {
		t.Error("expected BNDW001 for BOM-only input (no pragma)")
	}
}

// TestParse_ParseResultVersion tests that ParseResult.Version is always "1".
func TestParse_ParseResultVersion(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [A](a.md)\n")

	result, _, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Version != "1" {
		t.Errorf("ParseResult.Version = %q, want %q", result.Version, "1")
	}
}

// TestParse_Lines_SplitCorrectly tests that Lines contains the source lines without endings.
func TestParse_Lines_SplitCorrectly(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [A](a.md)\n- [B](b.md)\n")

	result, _, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 3 {
		t.Fatalf("len(Lines) = %d, want 3", len(result.Lines))
	}
	if result.Lines[0] != "<!-- prosemark-binder:v1 -->" {
		t.Errorf("Lines[0] = %q, want %q", result.Lines[0], "<!-- prosemark-binder:v1 -->")
	}
	if result.Lines[1] != "- [A](a.md)" {
		t.Errorf("Lines[1] = %q, want %q", result.Lines[1], "- [A](a.md)")
	}
	if result.Lines[2] != "- [B](b.md)" {
		t.Errorf("Lines[2] = %q, want %q", result.Lines[2], "- [B](b.md)")
	}
}

// --- List Scanning & Structural Link Extraction Tests (Phase-A steps 5-6) ---

// TestParse_NodeType_IsNode tests that structural list-item nodes have Type "node".
func TestParse_NodeType_IsNode(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Chapter](chapter.md)\n")

	result, _, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Root.Children) < 1 {
		t.Fatalf("Root.Children is empty; expected one structural node")
	}
	if result.Root.Children[0].Type != "node" {
		t.Errorf("node.Type = %q, want %q", result.Root.Children[0].Type, "node")
	}
}

// TestParse_InlineLinks tests inline link extraction for various formats (FR-001, FR-016).
func TestParse_InlineLinks(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		wantTarget string
		wantTitle  string
		wantNodes  int
	}{
		{
			name:       "simple inline link dash marker",
			src:        "<!-- prosemark-binder:v1 -->\n- [Chapter One](chapter-one.md)\n",
			wantTarget: "chapter-one.md",
			wantTitle:  "Chapter One",
			wantNodes:  1,
		},
		{
			name:       "inline link with tooltip title is ignored for display",
			src:        "<!-- prosemark-binder:v1 -->\n- [Chapter](chapter.md \"A tooltip\")\n",
			wantTarget: "chapter.md",
			wantTitle:  "Chapter",
			wantNodes:  1,
		},
		{
			name:       "empty bracket title derives from stem",
			src:        "<!-- prosemark-binder:v1 -->\n- [](chapter-one.md)\n",
			wantTarget: "chapter-one.md",
			wantTitle:  "chapter-one",
			wantNodes:  1,
		},
		{
			name:       "asterisk list marker",
			src:        "<!-- prosemark-binder:v1 -->\n* [Chapter](chapter.md)\n",
			wantTarget: "chapter.md",
			wantTitle:  "Chapter",
			wantNodes:  1,
		},
		{
			name:       "plus list marker",
			src:        "<!-- prosemark-binder:v1 -->\n+ [Chapter](chapter.md)\n",
			wantTarget: "chapter.md",
			wantTitle:  "Chapter",
			wantNodes:  1,
		},
		{
			name:       "multiple top-level nodes",
			src:        "<!-- prosemark-binder:v1 -->\n- [Ch1](ch1.md)\n- [Ch2](ch2.md)\n- [Ch3](ch3.md)\n",
			wantTarget: "ch1.md",
			wantTitle:  "Ch1",
			wantNodes:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := binder.Parse(context.Background(), []byte(tt.src), nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Root.Children) != tt.wantNodes {
				t.Fatalf("Root.Children len = %d, want %d", len(result.Root.Children), tt.wantNodes)
			}
			node := result.Root.Children[0]
			if node.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", node.Target, tt.wantTarget)
			}
			if node.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", node.Title, tt.wantTitle)
			}
		})
	}
}

// TestParse_Node_SourceMetadata tests that parsed nodes carry correct source metadata.
func TestParse_Node_SourceMetadata(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Chapter](chapter.md)\n")

	result, _, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Root.Children) < 1 {
		t.Fatalf("Root.Children is empty")
	}
	node := result.Root.Children[0]
	if node.Line != 2 {
		t.Errorf("node.Line = %d, want 2", node.Line)
	}
	if node.ListMarker != "-" {
		t.Errorf("node.ListMarker = %q, want %q", node.ListMarker, "-")
	}
	if node.Indent != 0 {
		t.Errorf("node.Indent = %d, want 0 (top-level item)", node.Indent)
	}
	if node.RawLine != "- [Chapter](chapter.md)" {
		t.Errorf("node.RawLine = %q, want %q", node.RawLine, "- [Chapter](chapter.md)")
	}
}

// TestParse_NestedList_BuildsHierarchy tests that indented list items build a node subtree.
func TestParse_NestedList_BuildsHierarchy(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Part](part.md)\n  - [Chapter](chapter.md)\n")

	result, _, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Root.Children) != 1 {
		t.Fatalf("Root.Children len = %d, want 1", len(result.Root.Children))
	}
	parent := result.Root.Children[0]
	if parent.Target != "part.md" {
		t.Errorf("parent.Target = %q, want %q", parent.Target, "part.md")
	}
	if len(parent.Children) != 1 {
		t.Fatalf("parent.Children len = %d, want 1", len(parent.Children))
	}
	child := parent.Children[0]
	if child.Target != "chapter.md" {
		t.Errorf("child.Target = %q, want %q", child.Target, "chapter.md")
	}
	if child.Title != "Chapter" {
		t.Errorf("child.Title = %q, want %q", child.Title, "Chapter")
	}
}

// TestParse_NestedList_DeepHierarchy tests multiple levels of nesting.
func TestParse_NestedList_DeepHierarchy(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n" +
		"- [Part](part.md)\n" +
		"  - [Chapter](chapter.md)\n" +
		"    - [Section](section.md)\n")

	result, _, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Root.Children) != 1 {
		t.Fatalf("Root.Children len = %d, want 1", len(result.Root.Children))
	}
	part := result.Root.Children[0]
	if len(part.Children) != 1 {
		t.Fatalf("part.Children len = %d, want 1", len(part.Children))
	}
	chapter := part.Children[0]
	if len(chapter.Children) != 1 {
		t.Fatalf("chapter.Children len = %d, want 1", len(chapter.Children))
	}
	section := chapter.Children[0]
	if section.Target != "section.md" {
		t.Errorf("section.Target = %q, want %q", section.Target, "section.md")
	}
}

// TestParse_ReferenceStyleLinks tests reference-style link resolution (FR-001).
func TestParse_ReferenceStyleLinks(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		wantTarget string
		wantTitle  string
	}{
		{
			name: "full reference link [Title][label]",
			src: "<!-- prosemark-binder:v1 -->\n" +
				"- [My Chapter][ch]\n" +
				"\n" +
				"[ch]: chapter.md\n",
			wantTarget: "chapter.md",
			wantTitle:  "My Chapter",
		},
		{
			name: "collapsed reference link [Title][]",
			src: "<!-- prosemark-binder:v1 -->\n" +
				"- [My Chapter][]\n" +
				"\n" +
				"[my chapter]: chapter.md\n",
			wantTarget: "chapter.md",
			wantTitle:  "My Chapter",
		},
		{
			name: "shortcut reference link [Title]",
			src: "<!-- prosemark-binder:v1 -->\n" +
				"- [My Chapter]\n" +
				"\n" +
				"[my chapter]: chapter.md\n",
			wantTarget: "chapter.md",
			wantTitle:  "My Chapter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := binder.Parse(context.Background(), []byte(tt.src), nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Root.Children) != 1 {
				t.Fatalf("Root.Children len = %d, want 1", len(result.Root.Children))
			}
			node := result.Root.Children[0]
			if node.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", node.Target, tt.wantTarget)
			}
			if node.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", node.Title, tt.wantTitle)
			}
		})
	}
}

// TestParse_RefDefs_PopulatedInResult tests that ParseResult.RefDefs is populated
// with all reference link definitions, keyed by normalized (lowercase) label.
func TestParse_RefDefs_PopulatedInResult(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n" +
		"- [Ch][ref1]\n" +
		"\n" +
		"[ref1]: chapter.md\n" +
		"[Ref2]: part.md \"Part Title\"\n")

	result, _, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RefDefs == nil {
		t.Fatal("ParseResult.RefDefs is nil")
	}
	ref1, ok := result.RefDefs["ref1"]
	if !ok {
		t.Fatal("RefDefs missing key 'ref1'")
	}
	if ref1.Target != "chapter.md" {
		t.Errorf("RefDefs[\"ref1\"].Target = %q, want %q", ref1.Target, "chapter.md")
	}
	ref2, ok := result.RefDefs["ref2"]
	if !ok {
		t.Fatal("RefDefs missing key 'ref2' (label should be normalized to lowercase)")
	}
	if ref2.Target != "part.md" {
		t.Errorf("RefDefs[\"ref2\"].Target = %q, want %q", ref2.Target, "part.md")
	}
	if ref2.Title != "Part Title" {
		t.Errorf("RefDefs[\"ref2\"].Title = %q, want %q", ref2.Title, "Part Title")
	}
}

// TestParse_Wikilinks tests Obsidian-style wikilink extraction and title derivation (FR-007, FR-016).
func TestParse_Wikilinks(t *testing.T) {
	project := &binder.Project{
		Version: "1",
		Files:   []string{"chapter.md", "part.md", "subfolder/deep.md"},
	}
	tests := []struct {
		name       string
		src        string
		wantTarget string
		wantTitle  string
	}{
		{
			name:       "simple wikilink uses stem as title",
			src:        "<!-- prosemark-binder:v1 -->\n- [[chapter]]\n",
			wantTarget: "chapter.md",
			wantTitle:  "chapter",
		},
		{
			name:       "wikilink with alias uses alias as title",
			src:        "<!-- prosemark-binder:v1 -->\n- [[chapter|My Chapter]]\n",
			wantTarget: "chapter.md",
			wantTitle:  "My Chapter",
		},
		{
			name:       "wikilink with empty alias falls back to stem",
			src:        "<!-- prosemark-binder:v1 -->\n- [[chapter|]]\n",
			wantTarget: "chapter.md",
			wantTitle:  "chapter",
		},
		{
			name:       "wikilink with subfolder path uses leaf stem as title",
			src:        "<!-- prosemark-binder:v1 -->\n- [[subfolder/deep]]\n",
			wantTarget: "subfolder/deep.md",
			wantTitle:  "deep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := binder.Parse(context.Background(), []byte(tt.src), project)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Root.Children) != 1 {
				t.Fatalf("Root.Children len = %d, want 1", len(result.Root.Children))
			}
			node := result.Root.Children[0]
			if node.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", node.Target, tt.wantTarget)
			}
			if node.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", node.Title, tt.wantTitle)
			}
		})
	}
}

// TestParse_NonMarkdownTarget_EmitsBNDW007 tests that links to non-.md files
// within list items emit BNDW007 and are not added to the tree (FR-001).
func TestParse_NonMarkdownTarget_EmitsBNDW007(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Image](picture.png)\n")

	_, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeNonMarkdownTarget {
			found = true
			if d.Severity != "warning" {
				t.Errorf("BNDW007 severity = %q, want %q", d.Severity, "warning")
			}
		}
	}
	if !found {
		t.Error("expected BNDW007 for non-.md link target in list item, got none")
	}
}

// TestParse_IllegalPathChars_EmitsBNDE001 tests that inline link targets containing
// illegal characters emit BNDE001 (FR-002).
func TestParse_IllegalPathChars_EmitsBNDE001(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "greater-than in path",
			src:  "<!-- prosemark-binder:v1 -->\n- [Bad](chapter>.md)\n",
		},
		{
			name: "null byte in path",
			src:  "<!-- prosemark-binder:v1 -->\n- [Bad](chapter\x00.md)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, diags, err := binder.Parse(context.Background(), []byte(tt.src), nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			found := false
			for _, d := range diags {
				if d.Code == binder.CodeIllegalPathChars {
					found = true
					if d.Severity != "error" {
						t.Errorf("BNDE001 severity = %q, want %q", d.Severity, "error")
					}
				}
			}
			if !found {
				t.Errorf("expected BNDE001 for illegal path chars, got none")
			}
		})
	}
}

// TestParse_PathEscapesRoot_EmitsBNDE002 tests that link targets escaping the project
// root via ../ emit BNDE002 (FR-002).
func TestParse_PathEscapesRoot_EmitsBNDE002(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Bad](../escape.md)\n")

	_, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodePathEscapesRoot {
			found = true
			if d.Severity != "error" {
				t.Errorf("BNDE002 severity = %q, want %q", d.Severity, "error")
			}
		}
	}
	if !found {
		t.Error("expected BNDE002 for path escaping project root, got none")
	}
}

// TestParse_LinkOutsideList_EmitsBNDW006 tests that .md links appearing outside of
// list items (e.g. in prose paragraphs) emit BNDW006 (FR-001).
func TestParse_LinkOutsideList_EmitsBNDW006(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\nSee also [a chapter](chapter.md) for details.\n")

	_, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeLinkOutsideList {
			found = true
			if d.Severity != "warning" {
				t.Errorf("BNDW006 severity = %q, want %q", d.Severity, "warning")
			}
		}
	}
	if !found {
		t.Error("expected BNDW006 for .md link outside list item, got none")
	}
}

// --- Path Validation: Percent-Encoded Paths (plan.md §H1) ---

// TestParse_PercentEncoded_IllegalChars_EmitsBNDE001 tests that percent-encoded paths
// are decoded via url.PathUnescape before illegal-character checks, so %3E ('>') and
// %3C ('<') trigger BNDE001 even though the raw bytes are valid ASCII (FR-002, plan §H1).
func TestParse_PercentEncoded_IllegalChars_EmitsBNDE001(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "percent-encoded greater-than %3E",
			src:  "<!-- prosemark-binder:v1 -->\n- [Bad](%3E.md)\n",
		},
		{
			name: "percent-encoded less-than %3C",
			src:  "<!-- prosemark-binder:v1 -->\n- [Bad](%3Cfile.md)\n",
		},
		{
			name: "malformed percent encoding %GG",
			src:  "<!-- prosemark-binder:v1 -->\n- [Bad](%GGinvalid.md)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, diags, err := binder.Parse(context.Background(), []byte(tt.src), nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			found := false
			for _, d := range diags {
				if d.Code == binder.CodeIllegalPathChars {
					found = true
					if d.Severity != "error" {
						t.Errorf("BNDE001 severity = %q, want %q", d.Severity, "error")
					}
				}
			}
			if !found {
				t.Errorf("expected BNDE001 after percent-decoding, got diags: %v", diags)
			}
		})
	}
}

// TestParse_PercentEncoded_RootEscape_EmitsBNDE002 tests that %2E%2E/ (which decodes
// to ../) is caught as a root-escape after url.PathUnescape (plan §H1).
func TestParse_PercentEncoded_RootEscape_EmitsBNDE002(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Bad](%2E%2E/secret.md)\n")

	_, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodePathEscapesRoot {
			found = true
			if d.Severity != "error" {
				t.Errorf("BNDE002 severity = %q, want %q", d.Severity, "error")
			}
		}
	}
	if !found {
		t.Error("expected BNDE002 for percent-encoded root-escape path, got none")
	}
}

// TestParse_PercentEncoded_DecodedTargetStoredInNode tests that the decoded path
// (not the raw percent-encoded form) is stored in Node.Target (plan §H1).
func TestParse_PercentEncoded_DecodedTargetStoredInNode(t *testing.T) {
	// %63hapter.md decodes to chapter.md (c = %63)
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Ch](%63hapter.md)\n")

	result, _, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Root.Children) != 1 {
		t.Fatalf("Root.Children len = %d, want 1", len(result.Root.Children))
	}
	got := result.Root.Children[0].Target
	if got != "chapter.md" {
		t.Errorf("Target = %q, want %q (decoded form)", got, "chapter.md")
	}
}

// --- Wikilink Resolution: Phase-A step 8 (FR-007) ---

// TestParse_WikilinkBasenameMatch_ResolvesSubdirFile tests that a wikilink stem
// resolves by basename across project files, not only root-level exact matches.
// [[chapter]] must find "sub/chapter.md" when that is the only project file (FR-007).
func TestParse_WikilinkBasenameMatch_ResolvesSubdirFile(t *testing.T) {
	project := &binder.Project{
		Version: "1",
		Files:   []string{"sub/chapter.md"},
	}
	src := []byte("<!-- prosemark-binder:v1 -->\n- [[chapter]]\n")

	result, diags, err := binder.Parse(context.Background(), src, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, d := range diags {
		if d.Code == binder.CodeAmbiguousWikilink {
			t.Errorf("unexpected BNDE003 when only one match exists: %+v", d)
		}
	}
	if len(result.Root.Children) != 1 {
		t.Fatalf("Root.Children len = %d, want 1 (wikilink must resolve by basename)", len(result.Root.Children))
	}
	if result.Root.Children[0].Target != "sub/chapter.md" {
		t.Errorf("Target = %q, want %q", result.Root.Children[0].Target, "sub/chapter.md")
	}
}

// TestParse_WikilinkProximityTiebreak_ShallowestWins tests that when multiple files
// share a basename, the shallowest-path file is selected (FR-007).
func TestParse_WikilinkProximityTiebreak_ShallowestWins(t *testing.T) {
	// Both files match stem "chapter" by basename; "sub/chapter.md" (depth 1) is shallower
	// than "deep/sub/chapter.md" (depth 2).
	project := &binder.Project{
		Version: "1",
		Files:   []string{"deep/sub/chapter.md", "sub/chapter.md"},
	}
	src := []byte("<!-- prosemark-binder:v1 -->\n- [[chapter]]\n")

	result, diags, err := binder.Parse(context.Background(), src, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, d := range diags {
		if d.Code == binder.CodeAmbiguousWikilink {
			t.Errorf("unexpected BNDE003 when proximity tiebreak should resolve: %+v", d)
		}
	}
	if len(result.Root.Children) != 1 {
		t.Fatalf("Root.Children len = %d, want 1", len(result.Root.Children))
	}
	if result.Root.Children[0].Target != "sub/chapter.md" {
		t.Errorf("Target = %q, want %q (shallowest match must win)", result.Root.Children[0].Target, "sub/chapter.md")
	}
}

// TestParse_AmbiguousWikilink_EmitsBNDE003 tests that a wikilink stem matching files
// in multiple directories at equal depth emits BNDE003 (FR-007, spec §US-1 sc8).
func TestParse_AmbiguousWikilink_EmitsBNDE003(t *testing.T) {
	// a/deep.md and b/deep.md are both at depth 1 — proximity tiebreak cannot resolve.
	project := &binder.Project{
		Version: "1",
		Files:   []string{"a/deep.md", "b/deep.md"},
	}
	src := []byte("<!-- prosemark-binder:v1 -->\n- [[deep]]\n")

	_, diags, err := binder.Parse(context.Background(), src, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeAmbiguousWikilink {
			found = true
			if d.Severity != "error" {
				t.Errorf("BNDE003 severity = %q, want %q", d.Severity, "error")
			}
		}
	}
	if !found {
		t.Error("expected BNDE003 for ambiguous wikilink with same-depth matches, got none")
	}
}

// TestParse_WikilinkCaseInsensitiveMatch_EmitsBNDW009 tests that a wikilink stem
// matching a project file only via case-insensitive comparison emits BNDW009 and still
// resolves the node to the actual filename (spec §US-1 sc7, FR-002).
func TestParse_WikilinkCaseInsensitiveMatch_EmitsBNDW009(t *testing.T) {
	// project has "chapter.md"; wikilink uses [[Chapter]] (case mismatch)
	project := &binder.Project{
		Version: "1",
		Files:   []string{"chapter.md"},
	}
	src := []byte("<!-- prosemark-binder:v1 -->\n- [[Chapter]]\n")

	result, diags, err := binder.Parse(context.Background(), src, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeCaseInsensitiveMatch {
			found = true
			if d.Severity != "warning" {
				t.Errorf("BNDW009 severity = %q, want %q", d.Severity, "warning")
			}
		}
	}
	if !found {
		t.Error("expected BNDW009 for case-insensitive wikilink match, got none")
	}
	// The node must still resolve to the actual filename from project.Files.
	if len(result.Root.Children) != 1 {
		t.Fatalf("Root.Children len = %d, want 1 (case-insensitive match must still resolve)", len(result.Root.Children))
	}
	if result.Root.Children[0].Target != "chapter.md" {
		t.Errorf("Target = %q, want %q (actual project file casing)", result.Root.Children[0].Target, "chapter.md")
	}
}

// --- Secondary Diagnostics: Phase-A step 9 (BNDW002–BNDW008) ---

// TestParse_MultipleStructLinks_EmitsBNDW002 tests that a list item containing more
// than one structural .md link emits BNDW002 and uses only the first as the node (FR-001).
func TestParse_MultipleStructLinks_EmitsBNDW002(t *testing.T) {
	// Two inline .md links in one list item; first becomes the node, second triggers BNDW002.
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Ch1](ch1.md) and also [Ch2](ch2.md)\n")

	result, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeMultipleStructLinks {
			found = true
			if d.Severity != "warning" {
				t.Errorf("BNDW002 severity = %q, want %q", d.Severity, "warning")
			}
		}
	}
	if !found {
		t.Error("expected BNDW002 for list item with multiple structural links, got none")
	}
	// First link must remain the structural node.
	if len(result.Root.Children) != 1 {
		t.Fatalf("Root.Children len = %d, want 1", len(result.Root.Children))
	}
	if result.Root.Children[0].Target != "ch1.md" {
		t.Errorf("Target = %q, want %q (first link must be structural)", result.Root.Children[0].Target, "ch1.md")
	}
}

// TestParse_DuplicateFileRef_EmitsBNDW003 tests that referencing the same .md file
// in more than one list item emits BNDW003 (FR-002).
func TestParse_DuplicateFileRef_EmitsBNDW003(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](chapter.md)\n- [Chapter Again](chapter.md)\n")

	_, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeDuplicateFileRef {
			found = true
			if d.Severity != "warning" {
				t.Errorf("BNDW003 severity = %q, want %q", d.Severity, "warning")
			}
		}
	}
	if !found {
		t.Error("expected BNDW003 for duplicate file reference, got none")
	}
}

// TestParse_MissingTargetFile_EmitsBNDW004 tests that a link target absent from
// project.Files emits BNDW004 when project context is provided (FR-002).
func TestParse_MissingTargetFile_EmitsBNDW004(t *testing.T) {
	project := &binder.Project{
		Version: "1",
		Files:   []string{"other.md"},
	}
	// chapter.md is not listed in project.Files.
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Chapter](chapter.md)\n")

	_, diags, err := binder.Parse(context.Background(), src, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeMissingTargetFile {
			found = true
			if d.Severity != "warning" {
				t.Errorf("BNDW004 severity = %q, want %q", d.Severity, "warning")
			}
		}
	}
	if !found {
		t.Error("expected BNDW004 for link target not present in project.Files, got none")
	}
}

// TestParse_MissingTargetFile_NoProjectContext_NoDiagnostic tests that BNDW004 is NOT
// emitted when project is nil (no project context to validate against).
func TestParse_MissingTargetFile_NoProjectContext_NoDiagnostic(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Chapter](chapter.md)\n")

	_, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, d := range diags {
		if d.Code == binder.CodeMissingTargetFile {
			t.Errorf("unexpected BNDW004 when project is nil: %+v", d)
		}
	}
}

// TestParse_SelfReferentialLink_EmitsBNDW008 tests that a link targeting the binder
// file itself (_binder.md) emits BNDW008 (FR-002).
func TestParse_SelfReferentialLink_EmitsBNDW008(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Self](_binder.md)\n")

	_, diags, err := binder.Parse(context.Background(), src, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, d := range diags {
		if d.Code == binder.CodeSelfReferentialLink {
			found = true
			if d.Severity != "warning" {
				t.Errorf("BNDW008 severity = %q, want %q", d.Severity, "warning")
			}
		}
	}
	if !found {
		t.Error("expected BNDW008 for link targeting _binder.md itself, got none")
	}
}
