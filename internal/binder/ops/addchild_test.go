package ops

// Tests for the add-child operation.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// binderSrc builds a minimal binder source with the pragma header and the
// given raw list-item lines, each terminated by a single LF.
func binderSrc(items ...string) []byte {
	var b strings.Builder
	b.WriteString("<!-- prosemark-binder:v1 -->\n\n")
	for _, item := range items {
		b.WriteString(item)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// hasDiagCode reports whether any diagnostic in diags has the given code,
// regardless of severity.
func hasDiagCode(diags []binder.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

// ──────────────────────────────────────────────────────────────────────────────
// Position variants
// ──────────────────────────────────────────────────────────────────────────────

// TestAddChild_AppendLast_Default verifies that a new node is appended after
// the existing child when no position flag is provided (default = "last").
func TestAddChild_AppendLast_Default(t *testing.T) {
	src := binderSrc("- [Chapter One](chapter-one.md)")
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
	oneIdx := bytes.Index(out, []byte("chapter-one.md"))
	twoIdx := bytes.Index(out, []byte("chapter-two.md"))
	if oneIdx < 0 || twoIdx < 0 {
		t.Fatalf("expected both nodes in output:\n%s", out)
	}
	if oneIdx > twoIdx {
		t.Errorf("chapter-one should precede chapter-two:\n%s", out)
	}
}

// TestAddChild_PrependFirst verifies that a new node is inserted before all
// existing siblings when Position is "first".
func TestAddChild_PrependFirst(t *testing.T) {
	src := binderSrc(
		"- [Chapter One](chapter-one.md)",
		"- [Chapter Two](chapter-two.md)",
	)
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "intro.md",
		Title:          "Introduction",
		Position:       "first",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	introIdx := bytes.Index(out, []byte("intro.md"))
	oneIdx := bytes.Index(out, []byte("chapter-one.md"))
	if introIdx < 0 || oneIdx < 0 {
		t.Fatalf("expected both nodes in output:\n%s", out)
	}
	if introIdx > oneIdx {
		t.Errorf("intro.md should precede chapter-one.md:\n%s", out)
	}
}

// TestAddChild_InsertAtIndex verifies that At inserts at the correct zero-based
// position among siblings.
func TestAddChild_InsertAtIndex(t *testing.T) {
	src := binderSrc(
		"- [Alpha](alpha.md)",
		"- [Beta](beta.md)",
		"- [Gamma](gamma.md)",
	)
	at := 1 // insert between alpha (0) and beta (1)
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "interlude.md",
		Title:          "Interlude",
		At:             &at,
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	alphaIdx := bytes.Index(out, []byte("alpha.md"))
	interludeIdx := bytes.Index(out, []byte("interlude.md"))
	betaIdx := bytes.Index(out, []byte("beta.md"))
	if alphaIdx < 0 || interludeIdx < 0 || betaIdx < 0 {
		t.Fatalf("expected all nodes in output:\n%s", out)
	}
	if !(alphaIdx < interludeIdx && interludeIdx < betaIdx) {
		t.Errorf("expected alpha < interlude < beta:\n%s", out)
	}
}

// TestAddChild_BeforeSibling verifies that Before inserts the new node
// immediately before the named sibling.
func TestAddChild_BeforeSibling(t *testing.T) {
	src := binderSrc(
		"- [Alpha](alpha.md)",
		"- [Beta](beta.md)",
		"- [Gamma](gamma.md)",
	)
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "inserted.md",
		Title:          "Inserted",
		Before:         "beta",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	alphaIdx := bytes.Index(out, []byte("alpha.md"))
	insertedIdx := bytes.Index(out, []byte("inserted.md"))
	betaIdx := bytes.Index(out, []byte("beta.md"))
	if alphaIdx < 0 || insertedIdx < 0 || betaIdx < 0 {
		t.Fatalf("expected all nodes in output:\n%s", out)
	}
	if !(alphaIdx < insertedIdx && insertedIdx < betaIdx) {
		t.Errorf("expected alpha < inserted < beta:\n%s", out)
	}
}

// TestAddChild_AfterSibling verifies that After inserts the new node
// immediately after the named sibling.
func TestAddChild_AfterSibling(t *testing.T) {
	src := binderSrc(
		"- [Alpha](alpha.md)",
		"- [Beta](beta.md)",
		"- [Gamma](gamma.md)",
	)
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "after-alpha.md",
		Title:          "After Alpha",
		After:          "alpha",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	alphaIdx := bytes.Index(out, []byte("alpha.md"))
	afterAlphaIdx := bytes.Index(out, []byte("after-alpha.md"))
	betaIdx := bytes.Index(out, []byte("beta.md"))
	if alphaIdx < 0 || afterAlphaIdx < 0 || betaIdx < 0 {
		t.Fatalf("expected all nodes in output:\n%s", out)
	}
	if !(alphaIdx < afterAlphaIdx && afterAlphaIdx < betaIdx) {
		t.Errorf("expected alpha < after-alpha < beta:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Idempotency (OPW002)
// ──────────────────────────────────────────────────────────────────────────────

// TestAddChild_IdempotencySkip_OPW002 verifies that when the target already
// exists as a direct child and --force is not set, the operation is skipped
// and an OPW002 warning is emitted with the file left unchanged.
func TestAddChild_IdempotencySkip_OPW002(t *testing.T) {
	src := binderSrc("- [Chapter One](chapter-one.md)")
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "chapter-one.md",
		Title:          "Chapter One Again",
		Position:       "last",
		Force:          false,
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasDiagCode(diags, binder.CodeDuplicateSkipped) {
		t.Errorf("expected OPW002 diagnostic, got: %v", diags)
	}
	if !bytes.Equal(out, src) {
		t.Errorf("file should be unchanged on idempotency skip:\ngot:  %q\nwant: %q", out, src)
	}
}

// TestAddChild_Force_AllowsDuplicate verifies that --force permits inserting a
// node whose target already exists as a child.
func TestAddChild_Force_AllowsDuplicate(t *testing.T) {
	src := binderSrc("- [Chapter One](chapter-one.md)")
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "chapter-one.md",
		Title:          "Chapter One Duplicate",
		Position:       "last",
		Force:          true,
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	count := bytes.Count(out, []byte("chapter-one.md"))
	if count < 2 {
		t.Errorf("expected ≥2 occurrences of chapter-one.md after --force, got %d:\n%s", count, out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// List marker and indentation inheritance
// ──────────────────────────────────────────────────────────────────────────────

// TestAddChild_OrderedList_MaxPlusOne_PeriodStyle verifies that when siblings
// use period-style ordered markers, the new node uses ordinal max+1 with the
// same marker style.
func TestAddChild_OrderedList_MaxPlusOne_PeriodStyle(t *testing.T) {
	src := binderSrc(
		"1. [Chapter One](chapter-one.md)",
		"2. [Chapter Two](chapter-two.md)",
		"3. [Chapter Three](chapter-three.md)",
	)
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "chapter-four.md",
		Title:          "Chapter Four",
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// max existing ordinal is 3; new node should use 4 with period style
	if !bytes.Contains(out, []byte("4. ")) {
		t.Errorf("expected '4. ' marker in output:\n%s", out)
	}
}

// TestAddChild_OrderedList_MaxPlusOne_ParenStyle verifies that paren-style
// ordered list markers are matched and incremented correctly.
func TestAddChild_OrderedList_MaxPlusOne_ParenStyle(t *testing.T) {
	src := binderSrc(
		"1) [Chapter One](chapter-one.md)",
		"2) [Chapter Two](chapter-two.md)",
	)
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "chapter-three.md",
		Title:          "Chapter Three",
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	if !bytes.Contains(out, []byte("3) ")) {
		t.Errorf("expected '3) ' marker in output:\n%s", out)
	}
}

// TestAddChild_TabIndentationInherited verifies that when existing siblings
// use tab indentation, the new node also uses tab indentation.
func TestAddChild_TabIndentationInherited(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n\n- [Root](root.md)\n\t- [Child One](child-one.md)\n")
	params := binder.AddChildParams{
		ParentSelector: "root",
		Target:         "child-two.md",
		Title:          "Child Two",
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// The new node's line must start with a tab
	if !bytes.Contains(out, []byte("\t- [Child Two](child-two.md)")) {
		t.Errorf("expected tab-indented new child in output:\n%q", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Title escaping
// ──────────────────────────────────────────────────────────────────────────────

// TestAddChild_TitleBracketEscape verifies that '[' and ']' in the title are
// backslash-escaped in the serialized link text.
func TestAddChild_TitleBracketEscape(t *testing.T) {
	src := binderSrc()
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "chapter.md",
		Title:          "Chapter [One] and [Two]",
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	if !bytes.Contains(out, []byte(`Chapter \[One\] and \[Two\]`)) {
		t.Errorf("expected brackets backslash-escaped in output:\n%q", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Title derivation
// ──────────────────────────────────────────────────────────────────────────────

// TestAddChild_EmptyTitle_UsesTargetStem verifies that when no title is
// supplied, the title is derived from the target's file stem.
func TestAddChild_EmptyTitle_UsesTargetStem(t *testing.T) {
	src := binderSrc()
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "chapter-one.md",
		Title:          "", // empty → derive from stem
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// Stem of "chapter-one.md" is "chapter-one"; it should appear as the link text.
	if !bytes.Contains(out, []byte("[chapter-one]")) {
		t.Errorf("expected stem-derived title 'chapter-one' as link text in output:\n%q", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Line-ending fallback for first child
// ──────────────────────────────────────────────────────────────────────────────

// TestAddChild_FirstChild_LineEndingFallback_LF verifies that when a node has
// no existing children, the inserted line uses the file's majority line ending
// (LF in this case) rather than introducing a different style.
func TestAddChild_FirstChild_LineEndingFallback_LF(t *testing.T) {
	// All lines in this source use LF only.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n- [Parent](parent.md)\n")
	params := binder.AddChildParams{
		ParentSelector: "parent",
		Target:         "first-child.md",
		Title:          "First Child",
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// Locate the new node's line and confirm its ending is LF (not CRLF).
	idx := bytes.Index(out, []byte("first-child.md"))
	if idx < 0 {
		t.Fatalf("first-child.md missing from output:\n%q", out)
	}
	lineEnd := bytes.IndexByte(out[idx:], '\n')
	if lineEnd < 0 {
		t.Fatalf("new node line has no LF terminator:\n%q", out)
	}
	if lineEnd > 0 && out[idx+lineEnd-1] == '\r' {
		t.Errorf("new node line uses CRLF; expected LF-only in an LF-majority file:\n%q", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Error abort cases (OPE001 – OPE008)
// ──────────────────────────────────────────────────────────────────────────────

// TestAddChild_ErrorCodesAbortMutation is a table-driven test that verifies
// each OPE error code causes the operation to abort and emit the correct
// diagnostic, with the source bytes left unchanged.
func TestAddChild_ErrorCodesAbortMutation(t *testing.T) {
	at5 := 5 // out of bounds: only 2 children exist

	tests := []struct {
		name     string
		src      []byte
		params   binder.AddChildParams
		wantCode string
	}{
		{
			name: "OPE001_selector_no_match",
			src:  binderSrc("- [Alpha](alpha.md)"),
			params: binder.AddChildParams{
				ParentSelector: "nonexistent",
				Target:         "new.md",
				Title:          "New",
				Position:       "last",
			},
			wantCode: binder.CodeSelectorNoMatch,
		},
		{
			name: "OPE004_path_escapes_root",
			src:  binderSrc("- [Alpha](alpha.md)"),
			params: binder.AddChildParams{
				ParentSelector: ".",
				Target:         "../escape.md",
				Title:          "Escape",
				Position:       "last",
			},
			wantCode: binder.CodeInvalidTargetPath,
		},
		{
			name: "OPE004_illegal_char_switch",
			src:  binderSrc("- [Alpha](alpha.md)"),
			params: binder.AddChildParams{
				ParentSelector: ".",
				Target:         "foo<bar.md",
				Title:          "FooBar",
				Position:       "last",
			},
			wantCode: binder.CodeInvalidTargetPath,
		},
		{
			name: "OPE004_control_char",
			src:  binderSrc("- [Alpha](alpha.md)"),
			params: binder.AddChildParams{
				ParentSelector: ".",
				Target:         "\x01foo.md",
				Title:          "Foo",
				Position:       "last",
			},
			wantCode: binder.CodeInvalidTargetPath,
		},
		{
			name: "OPE004_non_md_extension",
			src:  binderSrc("- [Alpha](alpha.md)"),
			params: binder.AddChildParams{
				ParentSelector: ".",
				Target:         "foo.txt",
				Title:          "Foo",
				Position:       "last",
			},
			wantCode: binder.CodeInvalidTargetPath,
		},
		{
			name: "OPE005_target_is_binder",
			src:  binderSrc("- [Alpha](alpha.md)"),
			params: binder.AddChildParams{
				ParentSelector: ".",
				Target:         "_binder.md",
				Title:          "Binder",
				Position:       "last",
			},
			wantCode: binder.CodeTargetIsBinder,
		},
		{
			name: "OPE006_parent_in_code_fence",
			// A node whose list item falls inside a fenced code block.
			src: []byte("<!-- prosemark-binder:v1 -->\n\n```\n- [Fenced](fenced.md)\n```\n"),
			params: binder.AddChildParams{
				ParentSelector: "fenced",
				Target:         "child.md",
				Title:          "Child",
				Position:       "last",
			},
			wantCode: binder.CodeNodeInCodeFence,
		},
		{
			name: "OPE007_before_sibling_not_found",
			src:  binderSrc("- [Alpha](alpha.md)", "- [Beta](beta.md)"),
			params: binder.AddChildParams{
				ParentSelector: ".",
				Target:         "new.md",
				Title:          "New",
				Before:         "nonexistent-sibling",
			},
			wantCode: binder.CodeSiblingNotFound,
		},
		{
			name: "OPE007_after_sibling_not_found",
			src:  binderSrc("- [Alpha](alpha.md)", "- [Beta](beta.md)"),
			params: binder.AddChildParams{
				ParentSelector: ".",
				Target:         "new.md",
				Title:          "New",
				After:          "nonexistent-sibling",
			},
			wantCode: binder.CodeSiblingNotFound,
		},
		{
			name: "OPE008_at_index_out_of_bounds",
			src:  binderSrc("- [Alpha](alpha.md)", "- [Beta](beta.md)"),
			params: binder.AddChildParams{
				ParentSelector: ".",
				Target:         "new.md",
				Title:          "New",
				At:             &at5,
			},
			wantCode: binder.CodeIndexOutOfBounds,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, diags, _ := AddChild(context.Background(), tt.src, nil, tt.params)

			if !hasDiagCode(diags, tt.wantCode) {
				t.Errorf("expected diagnostic %s, got: %v", tt.wantCode, diags)
			}
			// Mutation must not have occurred.
			if out != nil && !bytes.Equal(out, tt.src) {
				t.Errorf("source bytes must be unchanged on %s abort:\ngot:  %q\nwant: %q",
					tt.wantCode, out, tt.src)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Percent-encoding round-trip (spaces in filenames)
// ──────────────────────────────────────────────────────────────────────────────

// TestAddChild_SpaceInTarget_RoundTrips verifies that a target with a literal
// space is stored with the raw space in the binder and the node is recoverable
// after a round-trip parse.
func TestAddChild_SpaceInTarget_RoundTrips(t *testing.T) {
	src := binderSrc()
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "my chapter.md",
		Title:          "My Chapter",
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// Raw-space form must appear in the binder.
	if !bytes.Contains(out, []byte("(my chapter.md)")) {
		t.Errorf("expected raw-space target '(my chapter.md)' in output:\n%s", out)
	}
	// Round-trip: parse the written binder and verify the node is visible.
	result, parseDiags, parseErr := binder.Parse(context.Background(), out, nil)
	if parseErr != nil {
		t.Fatalf("round-trip parse failed: %v", parseErr)
	}
	if hasDiagCode(parseDiags, "error") {
		t.Errorf("round-trip parse produced error diagnostics: %v", parseDiags)
	}
	if len(result.Root.Children) == 0 {
		t.Fatalf("no children after round-trip; node was lost")
	}
	if result.Root.Children[0].Target != "my chapter.md" {
		t.Errorf("round-trip target = %q; want %q", result.Root.Children[0].Target, "my chapter.md")
	}
}

// TestAddChild_PreEncodedSpaceTarget_RoundTrips verifies that a pre-encoded
// target (%20) is decoded and stored as raw-space, producing the same final
// binder state as a literal-space target.
func TestAddChild_PreEncodedSpaceTarget_RoundTrips(t *testing.T) {
	src := binderSrc()
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "my%20chapter.md", // pre-encoded by caller
		Title:          "My Chapter",
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// Raw-space form must appear in the binder (percent-encoding is decoded at input boundary).
	if !bytes.Contains(out, []byte("(my chapter.md)")) {
		t.Errorf("expected raw-space target '(my chapter.md)' in output:\n%s", out)
	}
	// Round-trip parse must recover the decoded target.
	result, _, _ := binder.Parse(context.Background(), out, nil)
	if len(result.Root.Children) == 0 {
		t.Fatalf("no children after round-trip")
	}
	if result.Root.Children[0].Target != "my chapter.md" {
		t.Errorf("round-trip target = %q; want %q", result.Root.Children[0].Target, "my chapter.md")
	}
}

// TestAddChild_ColonInTarget_ReturnsOPE004 verifies that a colon in the target
// is rejected with OPE004 and leaves the binder unchanged.
func TestAddChild_ColonInTarget_ReturnsOPE004(t *testing.T) {
	src := binderSrc("- [Alpha](alpha.md)")
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "part:one.md",
		Title:          "Part One",
		Position:       "last",
	}

	out, diags, err := AddChild(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasDiagCode(diags, binder.CodeInvalidTargetPath) {
		t.Errorf("expected OPE004, got: %v", diags)
	}
	if !bytes.Equal(out, src) {
		t.Errorf("source should be unchanged on colon-target rejection:\ngot:  %q\nwant: %q", out, src)
	}
}

// TestAddChild_OPW001_MultiMatchAppliesAll verifies that a bare stem matching
// multiple nodes emits OPW001 (multi-match warning) and applies the operation
// to all matched parents.
func TestAddChild_OPW001_MultiMatchAppliesAll(t *testing.T) {
	// Two nodes whose target bare-stems are identical ("one").
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [First One](first/one.md)\n" +
		"- [Second One](second/one.md)\n")
	params := binder.AddChildParams{
		ParentSelector: "one", // matches both first/one and second/one
		Target:         "new.md",
		Title:          "New",
		Position:       "last",
	}

	out, diags, _ := AddChild(context.Background(), src, nil, params)

	if !hasDiagCode(diags, binder.CodeMultiMatch) {
		t.Errorf("expected OPW001 (multi-match), got: %v", diags)
	}
	// Both parents should receive the new child.
	if bytes.Count(out, []byte("new.md")) != 2 {
		t.Errorf("expected child added to both parents, got:\n%s", out)
	}
}

// TestAddChild_PercentDecodeError_UsesOriginalTarget verifies that when the
// target has an invalid percent-encoding, percentDecodeOpTarget falls back to
// the original target string unchanged.
func TestAddChild_PercentDecodeError_UsesOriginalTarget(t *testing.T) {
	src := binderSrc("- [Alpha](alpha.md)")
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "%ZZfoo.md", // invalid percent-encoded sequence
		Title:          "Foo",
		Position:       "last",
	}

	out, _, _ := AddChild(context.Background(), src, nil, params)

	if !bytes.Contains(out, []byte("%ZZfoo.md")) {
		t.Errorf("expected original (non-decoded) target in output, got:\n%s", out)
	}
}

// TestAddChild_TabIndent_FirstChild verifies that when a tab-indented parent
// node has no children, the new child is indented with a double-tab.
func TestAddChild_TabIndent_FirstChild(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n\n- [Part](part.md)\n\t- [Chapter](ch.md)\n")
	params := binder.AddChildParams{
		ParentSelector: "ch",
		Target:         "new.md",
		Title:          "New",
		Position:       "last",
	}

	out, _, _ := AddChild(context.Background(), src, nil, params)

	if !bytes.Contains(out, []byte("\t\t- [New](new.md)")) {
		t.Errorf("expected double-tab indented new child, got:\n%s", out)
	}
}

// TestAddChild_OrderedList_InsertFirst_MaxOrdinalPlusOne verifies that inserting
// at position "first" (insertIdx=0) in an ordered list assigns maxOrdinal+1 as
// the new item's ordinal.
func TestAddChild_OrderedList_InsertFirst_MaxOrdinalPlusOne(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n\n- [Part](part.md)\n  1) [Ch1](ch1.md)\n  2) [Ch2](ch2.md)\n")
	params := binder.AddChildParams{
		ParentSelector: "part",
		Target:         "ch-new.md",
		Title:          "Ch New",
		Position:       "first", // insertIdx = 0 → uses maxOrdinal+1
	}

	out, _, _ := AddChild(context.Background(), src, nil, params)

	// maxOrdinal([1), 2)]) = 2; new marker = "3)"
	if !bytes.Contains(out, []byte("3) [Ch New](ch-new.md)")) {
		t.Errorf("expected '3)' marker for first-position insert in ordered list, got:\n%s", out)
	}
}
