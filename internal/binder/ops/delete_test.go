package ops

// Tests for the delete operation.

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// looseBinder builds a binder source where each list item is separated by a
// blank line (loose list), which lets deletion tests create consecutive blank
// lines that must be collapsed.
func looseBinder(items ...string) []byte {
	var b strings.Builder
	b.WriteString("<!-- prosemark-binder:v1 -->\n\n")
	for i, item := range items {
		b.WriteString(item)
		b.WriteByte('\n')
		if i < len(items)-1 {
			b.WriteByte('\n') // blank separator between items
		}
	}
	return []byte(b.String())
}

// ──────────────────────────────────────────────────────────────────────────────
// Basic deletion: leaf node and node-with-children
// ──────────────────────────────────────────────────────────────────────────────

// TestDelete_LeafNode_Removed verifies that deleting a leaf node removes its
// list-item line and leaves all other nodes intact.
func TestDelete_LeafNode_Removed(t *testing.T) {
	src := binderSrc(
		"- [Chapter One](chapter-one.md)",
		"- [Chapter Two](chapter-two.md)",
		"- [Chapter Three](chapter-three.md)",
	)
	params := binder.DeleteParams{
		Selector: "chapter-two",
		Yes:      true,
	}

	out, diags, err := Delete(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	if bytes.Contains(out, []byte("chapter-two.md")) {
		t.Errorf("deleted node should not appear in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("chapter-one.md")) {
		t.Errorf("chapter-one.md should remain in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("chapter-three.md")) {
		t.Errorf("chapter-three.md should remain in output:\n%s", out)
	}
}

// TestDelete_NodeWithChildren_RemovesEntireSubtree verifies that deleting a
// non-leaf node removes the node and all its nested descendants.
func TestDelete_NodeWithChildren_RemovesEntireSubtree(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Part One](part-one.md)\n" +
		"  - [Chapter One](chapter-one.md)\n" +
		"  - [Chapter Two](chapter-two.md)\n" +
		"- [Part Two](part-two.md)\n")
	params := binder.DeleteParams{
		Selector: "part-one",
		Yes:      true,
	}

	out, diags, err := Delete(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	if bytes.Contains(out, []byte("part-one.md")) {
		t.Errorf("deleted node should not appear in output:\n%s", out)
	}
	if bytes.Contains(out, []byte("chapter-one.md")) {
		t.Errorf("child chapter-one.md should be deleted with parent:\n%s", out)
	}
	if bytes.Contains(out, []byte("chapter-two.md")) {
		t.Errorf("child chapter-two.md should be deleted with parent:\n%s", out)
	}
	if !bytes.Contains(out, []byte("part-two.md")) {
		t.Errorf("part-two.md should remain in output:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Cleanup: consecutive blank line collapse
// ──────────────────────────────────────────────────────────────────────────────

// TestDelete_ConsecutiveBlankLines_Collapsed verifies that when deletion
// creates two or more consecutive blank lines, they are collapsed to a single
// blank line.
func TestDelete_ConsecutiveBlankLines_Collapsed(t *testing.T) {
	// Loose list: blank lines between items. Deleting "beta" leaves the blank
	// before and after it, creating two consecutive blank lines.
	src := looseBinder(
		"- [Alpha](alpha.md)",
		"- [Beta](beta.md)",
		"- [Gamma](gamma.md)",
	)
	params := binder.DeleteParams{
		Selector: "beta",
		Yes:      true,
	}

	out, diags, err := Delete(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	if bytes.Contains(out, []byte("beta.md")) {
		t.Errorf("deleted node should not appear in output:\n%s", out)
	}
	// Two or more consecutive newlines collapse to at most two (\n\n = one blank line).
	if bytes.Contains(out, []byte("\n\n\n")) {
		t.Errorf("consecutive blank lines must be collapsed to one:\n%q", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Cleanup: trailing blank lines at EOF removed
// ──────────────────────────────────────────────────────────────────────────────

// TestDelete_LastTopLevelNode_TrailingBlanksRemoved verifies that after
// deleting the last top-level node, trailing blank lines at the end of the
// file are stripped.
func TestDelete_LastTopLevelNode_TrailingBlanksRemoved(t *testing.T) {
	// Source ends with a blank line after the only top-level node.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n- [Only](only.md)\n\n")
	params := binder.DeleteParams{
		Selector: "only",
		Yes:      true,
	}

	out, diags, err := Delete(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	if bytes.Contains(out, []byte("only.md")) {
		t.Errorf("deleted node should not appear in output:\n%s", out)
	}
	// No trailing blank lines at end-of-file.
	if bytes.HasSuffix(out, []byte("\n\n")) {
		t.Errorf("trailing blank lines must be removed from EOF:\n%q", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Cleanup: empty sublist pruned (OPW004)
// ──────────────────────────────────────────────────────────────────────────────

// TestDelete_EmptySublist_Pruned_OPW004 verifies that when deleting the only
// child of a parent leaves the parent with an empty sublist, the empty sublist
// is pruned from the source and an OPW004 warning is emitted.
func TestDelete_EmptySublist_Pruned_OPW004(t *testing.T) {
	// "parent" has exactly one child "only-child"; deleting it makes parent
	// a leaf and its sublist empty.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Parent](parent.md)\n" +
		"  - [Only Child](only-child.md)\n" +
		"- [Sibling](sibling.md)\n")
	params := binder.DeleteParams{
		Selector: "only-child",
		Yes:      true,
	}

	out, diags, err := Delete(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasDiagCode(diags, binder.CodeEmptySublistPruned) {
		t.Errorf("expected OPW004 (empty sublist pruned), got: %v", diags)
	}
	if bytes.Contains(out, []byte("only-child.md")) {
		t.Errorf("deleted node should not appear in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("parent.md")) {
		t.Errorf("parent node should remain in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("sibling.md")) {
		t.Errorf("sibling node should remain in output:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Warning: non-structural content destroyed (OPW003)
// ──────────────────────────────────────────────────────────────────────────────

// TestDelete_InlineProse_OPW003 verifies that when a deleted node's list item
// contains inline prose beyond its structural link, an OPW003 warning is
// emitted to inform the caller that non-structural content was destroyed.
func TestDelete_InlineProse_OPW003(t *testing.T) {
	// The list item for "chapter-one" has extra prose after the structural link.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One](chapter-one.md) — some extra prose content\n" +
		"- [Chapter Two](chapter-two.md)\n")
	params := binder.DeleteParams{
		Selector: "chapter-one",
		Yes:      true,
	}

	out, diags, err := Delete(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasDiagCode(diags, binder.CodeNonStructuralDestroyed) {
		t.Errorf("expected OPW003 (non-structural content destroyed), got: %v", diags)
	}
	if bytes.Contains(out, []byte("chapter-one.md")) {
		t.Errorf("deleted node should not appear in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("chapter-two.md")) {
		t.Errorf("chapter-two.md should remain in output:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Reference definition preserved (FR-014)
// ──────────────────────────────────────────────────────────────────────────────

// TestDelete_ReferenceDefinition_Preserved verifies that when a node uses a
// reference-style link, deleting the node removes the list item but leaves
// the reference definition intact in the file.
func TestDelete_ReferenceDefinition_Preserved(t *testing.T) {
	// Node uses reference link syntax; its definition appears at the bottom.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One][ch1]\n" +
		"- [Chapter Two](chapter-two.md)\n" +
		"\n" +
		"[ch1]: chapter-one.md\n")
	params := binder.DeleteParams{
		Selector: "chapter-one",
		Yes:      true,
	}

	out, diags, err := Delete(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// The list item for chapter-one should be gone.
	if bytes.Contains(out, []byte("- [Chapter One][ch1]")) {
		t.Errorf("list item for deleted node should not appear:\n%s", out)
	}
	// But the reference definition must remain.
	if !bytes.Contains(out, []byte("[ch1]: chapter-one.md")) {
		t.Errorf("reference definition must be preserved after deletion:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Error abort cases: OPE001, root-selector guard, missing --yes
// ──────────────────────────────────────────────────────────────────────────────

// TestDelete_ErrorCodesAbortMutation is a table-driven test verifying each
// error condition returns the correct diagnostic and leaves the source bytes
// unchanged (atomic abort semantics).
func TestDelete_ErrorCodesAbortMutation(t *testing.T) {
	baseSrc := binderSrc(
		"- [Alpha](alpha.md)",
		"- [Beta](beta.md)",
	)

	tests := []struct {
		name     string
		src      []byte
		params   binder.DeleteParams
		wantCode string
	}{
		{
			name: "OPE001_selector_no_match",
			src:  baseSrc,
			params: binder.DeleteParams{
				Selector: "nonexistent",
				Yes:      true,
			},
			wantCode: binder.CodeSelectorNoMatch,
		},
		{
			name: "OPE001_root_selector_not_valid_target",
			src:  baseSrc,
			params: binder.DeleteParams{
				Selector: ".",
				Yes:      true,
			},
			wantCode: binder.CodeSelectorNoMatch,
		},
		{
			name: "OPE009_missing_yes_confirmation",
			src:  baseSrc,
			params: binder.DeleteParams{
				Selector: "alpha",
				Yes:      false,
			},
			wantCode: binder.CodeIOOrParseFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, diags, _ := Delete(context.Background(), tt.src, nil, tt.params)

			if !hasDiagCode(diags, tt.wantCode) {
				t.Errorf("expected diagnostic %s, got: %v", tt.wantCode, diags)
			}
			if out != nil && !bytes.Equal(out, tt.src) {
				t.Errorf("source bytes must be unchanged on %s abort:\ngot:  %q\nwant: %q",
					tt.wantCode, out, tt.src)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Multi-match selector (OPW001): targets first match only
// ──────────────────────────────────────────────────────────────────────────────

// TestDelete_MultiMatch_OPW001_DeletesAll verifies that when a bare-stem
// selector matches multiple nodes, OPW001 is emitted and ALL matched nodes
// are deleted (all-match semantics).
func TestDelete_MultiMatch_OPW001_DeletesAll(t *testing.T) {
	// Two nodes share the bare stem "one" via different directory paths.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [First One](first/one.md)\n" +
		"- [Second One](second/one.md)\n" +
		"- [Other](other.md)\n")
	params := binder.DeleteParams{
		Selector: "one", // matches both first/one.md and second/one.md
		Yes:      true,
	}

	out, diags, err := Delete(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasDiagCode(diags, binder.CodeMultiMatch) {
		t.Errorf("expected OPW001 (multi-match), got: %v", diags)
	}
	// Both matches should be deleted (all-match semantics).
	if bytes.Contains(out, []byte("first/one.md")) {
		t.Errorf("first match should be deleted:\n%s", out)
	}
	if bytes.Contains(out, []byte("second/one.md")) {
		t.Errorf("second match should be deleted (all-match semantics):\n%s", out)
	}
	// Unrelated node should remain.
	if !bytes.Contains(out, []byte("other.md")) {
		t.Errorf("other.md should remain:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Parse error path (OPE009) via deleteParseBinderFn mock
// ──────────────────────────────────────────────────────────────────────────────

// TestDelete_ColonSelector_DelegatesToEvalSelector verifies that a colon-containing
// selector delegates to binder.EvalSelector rather than the flat deep search.
func TestDelete_ColonSelector_DelegatesToEvalSelector(t *testing.T) {
	src := binderSrc("- [Alpha](alpha.md)", "- [Beta](beta.md)")
	params := binder.DeleteParams{
		Selector: ".:alpha", // colon → delegates to EvalSelector
		Yes:      true,
	}

	out, _, err := Delete(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes.Contains(out, []byte("alpha.md")) {
		t.Errorf("alpha.md should be deleted:\n%s", out)
	}
}

// TestDelete_OPE002_ProjectAmbiguousBareStem verifies that when a bare-stem
// selector matches multiple files in the project, OPE002 is emitted.
func TestDelete_OPE002_ProjectAmbiguousBareStem(t *testing.T) {
	src := binderSrc("- [Alpha](alpha.md)")
	proj := &binder.Project{
		Files: []string{"dir1/one.md", "dir2/one.md"},
	}
	params := binder.DeleteParams{
		Selector: "one",
		Yes:      true,
	}

	_, diags, _ := Delete(context.Background(), src, proj, params)

	if !hasDiagCode(diags, binder.CodeAmbiguousBareStem) {
		t.Errorf("expected OPE002 (ambiguous bare stem), got: %v", diags)
	}
}

// TestDelete_OPE006_NodeInCodeFence verifies that when the selector matches a
// node inside a fenced code block, OPE006 is emitted.
func TestDelete_OPE006_NodeInCodeFence(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n\n```\n- [Fenced](fenced.md)\n```\n")
	params := binder.DeleteParams{
		Selector: "fenced",
		Yes:      true,
	}

	_, diags, _ := Delete(context.Background(), src, nil, params)

	if !hasDiagCode(diags, binder.CodeNodeInCodeFence) {
		t.Errorf("expected OPE006 (node in code fence), got: %v", diags)
	}
}

// TestDelete_PathSelector_WithSlash verifies that a selector containing "/"
// matches a node by its full relative path (deleteNodeMatchesSelector path branch).
func TestDelete_PathSelector_WithSlash(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n\n- [Part](part.md)\n  - [Chapter](subfolder/ch.md)\n")
	params := binder.DeleteParams{
		Selector: "subfolder/ch", // path selector with "/" → exact target match
		Yes:      true,
	}

	out, _, err := Delete(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes.Contains(out, []byte("subfolder/ch.md")) {
		t.Errorf("subfolder/ch.md should be deleted:\n%s", out)
	}
}

// TestDelete_ParseError_OPE009 verifies that when the underlying parser
// returns an error, Delete propagates it and emits an OPE009 diagnostic
// without mutating the source bytes.
func TestDelete_ParseError_OPE009(t *testing.T) {
	orig := deleteParseBinderFn
	t.Cleanup(func() { deleteParseBinderFn = orig })

	testErr := errors.New("mock parse failure")
	deleteParseBinderFn = func(_ context.Context, _ []byte, _ *binder.Project) (*binder.ParseResult, []binder.Diagnostic, error) {
		return nil, nil, testErr
	}

	src := binderSrc("- [Alpha](alpha.md)")
	out, diags, err := Delete(context.Background(), src, nil, binder.DeleteParams{
		Selector: "alpha",
		Yes:      true,
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
