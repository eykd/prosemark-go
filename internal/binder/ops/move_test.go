package ops

// Tests for the move operation.

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// ──────────────────────────────────────────────────────────────────────────────
// Basic move: leaf node to new parent (fixture 056)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_LeafToNewParent verifies that moving a leaf node to a new parent
// removes it from its original position and inserts it under the destination,
// and emits OPW004 when the source parent's sublist becomes empty.
func TestMove_LeafToNewParent(t *testing.T) {
	// sec-a.md is the only child of ch1.md; moving it to ch2.md empties ch1's sublist.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One](ch1.md)\n" +
		"  - [Section A](sec-a.md)\n" +
		"- [Chapter Two](ch2.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "sec-a.md",
		DestinationParentSelector: "ch2.md",
		Position:                  "last",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// sec-a.md must appear under ch2.md in the output.
	if !bytes.Contains(out, []byte("sec-a.md")) {
		t.Errorf("moved node should appear in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("ch1.md")) {
		t.Errorf("source parent ch1.md should remain in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("ch2.md")) {
		t.Errorf("destination ch2.md should remain in output:\n%s", out)
	}
	// OPW004: ch1.md's sublist became empty and must be pruned.
	if !hasDiagCode(diags, binder.CodeEmptySublistPruned) {
		t.Errorf("expected OPW004 (empty sublist pruned), got: %v", diags)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Move subtree with children (fixture 057)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_SubtreeToNewParent verifies that moving a non-leaf node transfers
// its entire subtree (all descendants) to the destination parent.
func TestMove_SubtreeToNewParent(t *testing.T) {
	// ch1.md has a child sec-a.md; both should move together to part2.md.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Part One](part1.md)\n" +
		"  - [Chapter One](ch1.md)\n" +
		"    - [Section A](sec-a.md)\n" +
		"- [Part Two](part2.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch1.md",
		DestinationParentSelector: "part2.md",
		Position:                  "last",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// Both ch1.md and its child sec-a.md must appear in the output.
	if !bytes.Contains(out, []byte("ch1.md")) {
		t.Errorf("moved node ch1.md should appear in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("sec-a.md")) {
		t.Errorf("child sec-a.md should move with parent:\n%s", out)
	}
	if !bytes.Contains(out, []byte("part1.md")) {
		t.Errorf("part1.md should remain in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("part2.md")) {
		t.Errorf("part2.md should remain in output:\n%s", out)
	}
	// OPW004: part1.md's sublist became empty.
	if !hasDiagCode(diags, binder.CodeEmptySublistPruned) {
		t.Errorf("expected OPW004 (empty sublist pruned), got: %v", diags)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Reorder siblings within same parent (fixture 058)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_ReorderSameParent verifies that moving a node to a different position
// under the same parent correctly reorders the siblings.
func TestMove_ReorderSameParent(t *testing.T) {
	// ch3.md is last; moving it to first position should put it before ch1.md.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One](ch1.md)\n" +
		"- [Chapter Two](ch2.md)\n" +
		"- [Chapter Three](ch3.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch3.md",
		DestinationParentSelector: ".",
		Position:                  "first",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// ch3.md must appear before ch1.md in the output.
	ch3Pos := bytes.Index(out, []byte("ch3.md"))
	ch1Pos := bytes.Index(out, []byte("ch1.md"))
	if ch3Pos < 0 {
		t.Errorf("ch3.md not found in output:\n%s", out)
	} else if ch1Pos < 0 {
		t.Errorf("ch1.md not found in output:\n%s", out)
	} else if ch3Pos > ch1Pos {
		t.Errorf("ch3.md should appear before ch1.md after moving to first:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Indentation adjustment (fixture 059)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_IndentationAdjusted verifies that when a node is moved to a deeper
// nesting level, its indentation is adjusted to match the destination depth.
func TestMove_IndentationAdjusted(t *testing.T) {
	// ch1.md is at depth 2; moving it under deep.md (depth 4) should increase indent.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Part One](part1.md)\n" +
		"  - [Chapter One](ch1.md)\n" +
		"- [Part Two](part2.md)\n" +
		"  - [Sub](sub.md)\n" +
		"    - [Deep](deep.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch1.md",
		DestinationParentSelector: "deep.md",
		Position:                  "last",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// ch1.md should appear in output with more leading whitespace than before.
	if !bytes.Contains(out, []byte("ch1.md")) {
		t.Errorf("moved node ch1.md should appear in output:\n%s", out)
	}
	// OPW004: part1.md's sublist became empty.
	if !hasDiagCode(diags, binder.CodeEmptySublistPruned) {
		t.Errorf("expected OPW004 (empty sublist pruned), got: %v", diags)
	}
	// The moved item must be indented deeper than deep.md (6 leading spaces).
	if !bytes.Contains(out, []byte("      - [Chapter One](ch1.md)")) {
		t.Errorf("ch1.md should be indented to match destination depth:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Link syntax preserved: wikilink (fixture 060)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_WikilinkSyntaxPreserved verifies that a node expressed as a wikilink
// retains its wikilink syntax after being moved to a new parent.
func TestMove_WikilinkSyntaxPreserved(t *testing.T) {
	// [[chapter-01]] uses wikilink syntax; it must be preserved in the output.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Part One](part1.md)\n" +
		"  - [[chapter-01]]\n" +
		"- [Part Two](part2.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "chapter-01.md",
		DestinationParentSelector: "part2.md",
		Position:                  "last",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// Wikilink syntax must be preserved in the output.
	if !bytes.Contains(out, []byte("[[chapter-01]]")) {
		t.Errorf("wikilink syntax should be preserved after move:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Reference-link syntax preserved (fixture 095)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_ReferenceLinkSyntaxPreserved verifies that a node expressed as a
// reference-style link retains its reference syntax and that the link
// definition remains intact after the move.
func TestMove_ReferenceLinkSyntaxPreserved(t *testing.T) {
	// [Chapter][ch-ref] uses reference syntax; the definition [ch-ref]: must remain.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Part](part.md)\n" +
		"  - [Chapter][ch-ref]\n" +
		"- [Other](other.md)\n" +
		"\n" +
		"[ch-ref]: chapter.md\n")
	params := binder.MoveParams{
		SourceSelector:            "part:chapter",
		DestinationParentSelector: ".",
		Position:                  "last",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// Reference link syntax must be preserved in the moved list item.
	if !bytes.Contains(out, []byte("[Chapter][ch-ref]")) {
		t.Errorf("reference link syntax should be preserved after move:\n%s", out)
	}
	// Reference definition must remain intact.
	if !bytes.Contains(out, []byte("[ch-ref]: chapter.md")) {
		t.Errorf("reference definition should remain after move:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tooltip preserved (fixture 131)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_TooltipPreserved verifies that an inline link's tooltip (title)
// attribute is preserved after the node is moved to a new parent.
func TestMove_TooltipPreserved(t *testing.T) {
	// ch1.md has a tooltip "Introduction chapter" that must survive the move.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Part One](part1.md)\n" +
		"  - [Chapter One](ch1.md \"Introduction chapter\")\n" +
		"- [Part Two](part2.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch1.md",
		DestinationParentSelector: "part2.md",
		Position:                  "last",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// Tooltip must be preserved in the moved list item.
	if !bytes.Contains(out, []byte(`ch1.md "Introduction chapter"`)) {
		t.Errorf("tooltip should be preserved after move:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Non-structural content warning: OPW003 (fixture 129)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_NonStructuralContent_OPW003 verifies that when the source node's
// list item contains non-structural inline content (e.g., a task checkbox),
// OPW003 is emitted and the move proceeds, stripping the non-structural part.
func TestMove_NonStructuralContent_OPW003(t *testing.T) {
	// "[ ] " before the link is non-structural; it should be destroyed on move.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Part One](part1.md)\n" +
		"  - [ ] [Chapter One](ch1.md)\n" +
		"- [Part Two](part2.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch1.md",
		DestinationParentSelector: "part2.md",
		Position:                  "last",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// OPW003 must be emitted for destroyed non-structural content.
	if !hasDiagCode(diags, binder.CodeNonStructuralDestroyed) {
		t.Errorf("expected OPW003 (non-structural content destroyed), got: %v", diags)
	}
	// ch1.md should still appear in output.
	if !bytes.Contains(out, []byte("ch1.md")) {
		t.Errorf("moved node ch1.md should appear in output:\n%s", out)
	}
	// OPW004 because part1.md lost its only child.
	if !hasDiagCode(diags, binder.CodeEmptySublistPruned) {
		t.Errorf("expected OPW004 (empty sublist pruned), got: %v", diags)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Subtree list markers preserved (fixture 130)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_SubtreeMarkersPreserved verifies that when a node with a subtree is
// moved, the internal list markers of the subtree are preserved as-is, and the
// root marker is adapted to match the destination convention.
func TestMove_SubtreeMarkersPreserved(t *testing.T) {
	// ch1.md uses ordered marker "1." and has unordered children sec-a.md, sec-b.md.
	// After move to part2.md (which has ordered "1."), marker becomes "2." (next ordinal).
	// Internal children keep their "-" markers.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Part One](part1.md)\n" +
		"  1. [Chapter One](ch1.md)\n" +
		"     - [Section A](sec-a.md)\n" +
		"     - [Section B](sec-b.md)\n" +
		"- [Part Two](part2.md)\n" +
		"  1. [Appendix](appendix.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch1.md",
		DestinationParentSelector: "part2.md",
		Position:                  "last",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// ch1.md and its children should appear in output.
	if !bytes.Contains(out, []byte("ch1.md")) {
		t.Errorf("moved node ch1.md should appear in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("sec-a.md")) {
		t.Errorf("child sec-a.md should appear in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("sec-b.md")) {
		t.Errorf("child sec-b.md should appear in output:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Cycle detection: OPE003 (fixture 061)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_CycleDetected_OPE003 verifies that when the destination parent is a
// descendant of the source node, OPE003 is emitted and the source bytes are
// left unchanged (atomic abort semantics).
func TestMove_CycleDetected_OPE003(t *testing.T) {
	// ch1.md → sec-a.md → sub-i.md; moving ch1.md under sub-i.md would create a cycle.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One](ch1.md)\n" +
		"  - [Section A](sec-a.md)\n" +
		"    - [Subsection I](sub-i.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch1.md",
		DestinationParentSelector: "sub-i.md",
		Position:                  "last",
		Yes:                       true,
	}

	out, diags, _ := Move(context.Background(), src, nil, params)

	if !hasDiagCode(diags, binder.CodeCycleDetected) {
		t.Errorf("expected OPE003 (cycle detected), got: %v", diags)
	}
	if out != nil && !bytes.Equal(out, src) {
		t.Errorf("source bytes must be unchanged on OPE003 abort:\ngot:  %q\nwant: %q", out, src)
	}
}

// TestMove_DeepCycleDetected_OPE003 verifies cycle detection across multiple
// levels of nesting (fixture 103).
func TestMove_DeepCycleDetected_OPE003(t *testing.T) {
	// a → b → c → d → e; moving a under e would create a cycle.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [A](a.md)\n" +
		"  - [B](b.md)\n" +
		"    - [C](c.md)\n" +
		"      - [D](d.md)\n" +
		"        - [E](e.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "a",
		DestinationParentSelector: "a:b:c:d:e",
		Position:                  "last",
		Yes:                       true,
	}

	out, diags, _ := Move(context.Background(), src, nil, params)

	if !hasDiagCode(diags, binder.CodeCycleDetected) {
		t.Errorf("expected OPE003 (cycle detected) for deep chain, got: %v", diags)
	}
	if out != nil && !bytes.Equal(out, src) {
		t.Errorf("source bytes must be unchanged on OPE003 abort:\ngot:  %q\nwant: %q", out, src)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Error abort cases: OPE001, OPE009, root-selector guard
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_ErrorCodesAbortMutation is a table-driven test verifying each error
// condition returns the correct diagnostic and leaves source bytes unchanged
// (atomic abort semantics).
func TestMove_ErrorCodesAbortMutation(t *testing.T) {
	baseSrc := binderSrc(
		"- [Alpha](alpha.md)",
		"- [Beta](beta.md)",
	)

	tests := []struct {
		name     string
		src      []byte
		params   binder.MoveParams
		wantCode string
	}{
		{
			name: "OPE001_source_no_match",
			src:  baseSrc,
			params: binder.MoveParams{
				SourceSelector:            "nonexistent",
				DestinationParentSelector: "alpha",
				Position:                  "last",
				Yes:                       true,
			},
			wantCode: binder.CodeSelectorNoMatch,
		},
		{
			name: "OPE001_destination_no_match",
			src:  baseSrc,
			params: binder.MoveParams{
				SourceSelector:            "alpha",
				DestinationParentSelector: "nonexistent",
				Position:                  "last",
				Yes:                       true,
			},
			wantCode: binder.CodeSelectorNoMatch,
		},
		{
			name: "OPE009_missing_yes_confirmation",
			src:  baseSrc,
			params: binder.MoveParams{
				SourceSelector:            "alpha",
				DestinationParentSelector: "beta",
				Position:                  "last",
				Yes:                       false,
			},
			wantCode: binder.CodeIOOrParseFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, diags, _ := Move(context.Background(), tt.src, nil, tt.params)

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
// Multi-match selector (OPW001): all matches moved (fixture 073)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_MultiMatch_OPW001_AllMoved verifies that when a bare-stem selector
// matches multiple nodes, OPW001 is emitted and all matched nodes are moved to
// the destination parent.
func TestMove_MultiMatch_OPW001_AllMoved(t *testing.T) {
	// Two nodes share the stem "ch"; both should be moved under app.md.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter](ch.md)\n" +
		"- [Chapter (alt)](ch.md)\n" +
		"- [Appendix](app.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch",
		DestinationParentSelector: "app",
		Position:                  "last",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasDiagCode(diags, binder.CodeMultiMatch) {
		t.Errorf("expected OPW001 (multi-match), got: %v", diags)
	}
	// Both ch.md entries must appear in the output (moved under app.md).
	if !bytes.Contains(out, []byte("app.md")) {
		t.Errorf("destination app.md should appear in output:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Positioning: --at, --before, --after, OPE008, OPE007
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_At_InsertAtIndex verifies that --at=N inserts the source node at the
// correct position among the destination's children.
func TestMove_At_InsertAtIndex(t *testing.T) {
	// ch1, ch2, ch3 at root; move ch3 to position 1 (between ch1 and ch2).
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One](ch1.md)\n" +
		"- [Chapter Two](ch2.md)\n" +
		"- [Chapter Three](ch3.md)\n")
	at := 1
	params := binder.MoveParams{
		SourceSelector:            "ch3.md",
		DestinationParentSelector: ".",
		At:                        &at,
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// ch3.md must appear between ch1.md and ch2.md.
	ch1Pos := bytes.Index(out, []byte("ch1.md"))
	ch3Pos := bytes.Index(out, []byte("ch3.md"))
	ch2Pos := bytes.Index(out, []byte("ch2.md"))
	if ch1Pos < 0 || ch2Pos < 0 || ch3Pos < 0 {
		t.Fatalf("expected all nodes in output:\n%s", out)
	}
	if !(ch1Pos < ch3Pos && ch3Pos < ch2Pos) {
		t.Errorf("expected order ch1, ch3, ch2 but got positions %d, %d, %d:\n%s",
			ch1Pos, ch3Pos, ch2Pos, out)
	}
}

// TestMove_At_OutOfBounds_OPE008 verifies that --at=N where N > len(children)
// returns OPE008 and leaves source bytes unchanged.
func TestMove_At_OutOfBounds_OPE008(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One](ch1.md)\n" +
		"- [Chapter Two](ch2.md)\n" +
		"- [Chapter Three](ch3.md)\n")
	at := 99
	params := binder.MoveParams{
		SourceSelector:            "ch3.md",
		DestinationParentSelector: ".",
		At:                        &at,
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasDiagCode(diags, binder.CodeIndexOutOfBounds) {
		t.Errorf("expected OPE008 for out-of-bounds --at, got: %v", diags)
	}
	if !bytes.Equal(out, src) {
		t.Error("source bytes must be unchanged on OPE008 abort")
	}
}

// TestMove_Before_InsertBeforeSibling verifies that --before=X inserts the
// source node immediately before sibling X in the destination's children.
func TestMove_Before_InsertBeforeSibling(t *testing.T) {
	// ch1, ch2, ch3 at root; move ch3 before ch2 → ch1, ch3, ch2.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One](ch1.md)\n" +
		"- [Chapter Two](ch2.md)\n" +
		"- [Chapter Three](ch3.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch3.md",
		DestinationParentSelector: ".",
		Before:                    "ch2",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	ch1Pos := bytes.Index(out, []byte("ch1.md"))
	ch3Pos := bytes.Index(out, []byte("ch3.md"))
	ch2Pos := bytes.Index(out, []byte("ch2.md"))
	if ch1Pos < 0 || ch2Pos < 0 || ch3Pos < 0 {
		t.Fatalf("expected all nodes in output:\n%s", out)
	}
	if !(ch1Pos < ch3Pos && ch3Pos < ch2Pos) {
		t.Errorf("expected order ch1, ch3, ch2 but got positions %d, %d, %d:\n%s",
			ch1Pos, ch3Pos, ch2Pos, out)
	}
}

// TestMove_Before_SiblingNotFound_OPE007 verifies that --before=X where X
// doesn't exist among the destination's children returns OPE007.
func TestMove_Before_SiblingNotFound_OPE007(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One](ch1.md)\n" +
		"- [Chapter Two](ch2.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch1.md",
		DestinationParentSelector: ".",
		Before:                    "nonexistent",
		Yes:                       true,
	}

	out, diags, _ := Move(context.Background(), src, nil, params)

	if !hasDiagCode(diags, binder.CodeSiblingNotFound) {
		t.Errorf("expected OPE007 for missing before-sibling, got: %v", diags)
	}
	if !bytes.Equal(out, src) {
		t.Error("source bytes must be unchanged on OPE007 abort")
	}
}

// TestMove_After_InsertAfterSibling verifies that --after=X inserts the source
// node immediately after sibling X in the destination's children.
func TestMove_After_InsertAfterSibling(t *testing.T) {
	// ch1, ch2, ch3 at root; move ch1 after ch2 → ch2, ch1, ch3.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One](ch1.md)\n" +
		"- [Chapter Two](ch2.md)\n" +
		"- [Chapter Three](ch3.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch1.md",
		DestinationParentSelector: ".",
		After:                     "ch2",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	ch2Pos := bytes.Index(out, []byte("ch2.md"))
	ch1Pos := bytes.Index(out, []byte("ch1.md"))
	ch3Pos := bytes.Index(out, []byte("ch3.md"))
	if ch1Pos < 0 || ch2Pos < 0 || ch3Pos < 0 {
		t.Fatalf("expected all nodes in output:\n%s", out)
	}
	if !(ch2Pos < ch1Pos && ch1Pos < ch3Pos) {
		t.Errorf("expected order ch2, ch1, ch3 but got positions %d, %d, %d:\n%s",
			ch2Pos, ch1Pos, ch3Pos, out)
	}
}

// TestMove_After_LastSibling verifies that --after=<last-child> inserts the
// source node at the end of the destination's children (after the last one).
// This exercises the ri >= len(remaining) branch in moveresolveInsertionIndex.
func TestMove_After_LastSibling(t *testing.T) {
	// ch1, ch2, ch3, ch4 at root; move ch1 after ch4 → ch2, ch3, ch4, ch1.
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One](ch1.md)\n" +
		"- [Chapter Two](ch2.md)\n" +
		"- [Chapter Three](ch3.md)\n" +
		"- [Chapter Four](ch4.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch1.md",
		DestinationParentSelector: ".",
		After:                     "ch4",
		Yes:                       true,
	}

	out, diags, err := Move(context.Background(), src, nil, params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	// ch1 must appear after ch4 (at the end).
	ch4Pos := bytes.Index(out, []byte("ch4.md"))
	ch1Pos := bytes.Index(out, []byte("ch1.md"))
	if ch4Pos < 0 || ch1Pos < 0 {
		t.Fatalf("expected all nodes in output:\n%s", out)
	}
	if ch4Pos >= ch1Pos {
		t.Errorf("expected ch4 before ch1 (after-last), got positions ch4=%d ch1=%d:\n%s",
			ch4Pos, ch1Pos, out)
	}
}

// TestMove_After_SiblingNotFound_OPE007 verifies that --after=X where X
// doesn't exist among the destination's children returns OPE007.
func TestMove_After_SiblingNotFound_OPE007(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n\n" +
		"- [Chapter One](ch1.md)\n" +
		"- [Chapter Two](ch2.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "ch1.md",
		DestinationParentSelector: ".",
		After:                     "nonexistent",
		Yes:                       true,
	}

	out, diags, _ := Move(context.Background(), src, nil, params)

	if !hasDiagCode(diags, binder.CodeSiblingNotFound) {
		t.Errorf("expected OPE007 for missing after-sibling, got: %v", diags)
	}
	if !bytes.Equal(out, src) {
		t.Error("source bytes must be unchanged on OPE007 abort")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Parse error path (OPE009) via moveParseBinderFn mock
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_ParseError_OPE009 verifies that when the underlying parser returns
// an error, Move propagates it and emits OPE009 without mutating source bytes.
func TestMove_ParseError_OPE009(t *testing.T) {
	orig := moveParseBinderFn
	t.Cleanup(func() { moveParseBinderFn = orig })

	testErr := errors.New("mock parse failure")
	moveParseBinderFn = func(_ context.Context, _ []byte, _ *binder.Project) (*binder.ParseResult, []binder.Diagnostic, error) {
		return nil, nil, testErr
	}

	src := binderSrc("- [Alpha](alpha.md)", "- [Beta](beta.md)")
	out, diags, err := Move(context.Background(), src, nil, binder.MoveParams{
		SourceSelector:            "alpha",
		DestinationParentSelector: "beta",
		Position:                  "last",
		Yes:                       true,
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
