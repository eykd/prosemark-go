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
