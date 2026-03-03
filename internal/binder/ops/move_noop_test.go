package ops

// RED tests for no-op move detection (prosemark-go-737).
//
// Moving a node to its current parent is a no-op: the document structure is
// unchanged. Before this fix the implementation would temporarily detach the
// source node (making the parent's sublist empty), trigger the OPW004 pruning
// check, then re-attach it — resulting in a spurious "empty sublist was pruned"
// warning even though the end state was identical to the start state.
//
// These tests MUST fail until the implementation detects the no-op and skips
// the spurious warning.

import (
	"bytes"
	"context"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// ──────────────────────────────────────────────────────────────────────────────
// No-op move: sole child moved to same parent emits no OPW004 (prosemark-go-737)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_NoOp_SoleChildToSameParent verifies that moving a node to its
// current parent (a structural no-op) does NOT emit OPW004.
//
// sec-a.md is the sole child of ch1.md. Moving it to ch1.md (its current
// parent) leaves the document unchanged. OPW004 must NOT be emitted because no
// sublist was actually pruned — the empty-sublist state is transient and
// disappears once the node is re-inserted.
func TestMove_NoOp_SoleChildToSameParent(t *testing.T) {
	src := binderSrc(
		"- [Chapter One](ch1.md)",
		"  - [Section A](sec-a.md)",
		"- [Chapter Two](ch2.md)",
	)
	params := binder.MoveParams{
		SourceSelector:            "sec-a.md",
		DestinationParentSelector: "ch1.md", // same as current parent
		Position:                  "last",
		Yes:                       true,
	}

	out, diags := Move(context.Background(), src, nil, params)
	// A no-op move must not produce any diagnostic — specifically no OPW004.
	if hasDiagCode(diags, binder.CodeEmptySublistPruned) {
		t.Errorf("no-op move must not emit OPW004 (empty sublist pruned), got: %v", diags)
	}
	// Output must equal the input (nothing changed structurally).
	if !bytes.Equal(out, src) {
		t.Errorf("no-op move must leave document unchanged.\ngot:\n%s\nwant:\n%s", out, src)
	}
}

// TestMove_NoOp_TableDriven covers additional no-op move scenarios using a
// table-driven style to ensure robustness across different tree shapes.
func TestMove_NoOp_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		src      []byte
		params   binder.MoveParams
		wantCode string // diagnostic code that must NOT appear
	}{
		{
			name: "sole_child_moved_to_same_parent_position_last",
			src: binderSrc(
				"- [Part](part.md)",
				"  - [Chapter](ch.md)",
			),
			params: binder.MoveParams{
				SourceSelector:            "ch.md",
				DestinationParentSelector: "part.md",
				Position:                  "last",
				Yes:                       true,
			},
			wantCode: binder.CodeEmptySublistPruned,
		},
		{
			name: "sole_child_moved_to_same_parent_position_first",
			src: binderSrc(
				"- [Part](part.md)",
				"  - [Chapter](ch.md)",
			),
			params: binder.MoveParams{
				SourceSelector:            "ch.md",
				DestinationParentSelector: "part.md",
				Position:                  "first",
				Yes:                       true,
			},
			wantCode: binder.CodeEmptySublistPruned,
		},
		{
			name: "root_child_moved_to_root_no_opw004",
			src: binderSrc(
				"- [Alpha](alpha.md)",
				"- [Beta](beta.md)",
			),
			params: binder.MoveParams{
				SourceSelector:            "alpha.md",
				DestinationParentSelector: ".",
				Position:                  "last",
				Yes:                       true,
			},
			// Moving a root-level node to root is not a strict no-op (position
			// may change), but it must still never emit OPW004 since the root
			// sublist can never be "emptied" by a move.
			wantCode: binder.CodeEmptySublistPruned,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, diags := Move(context.Background(), tt.src, nil, tt.params)
			if hasDiagCode(diags, tt.wantCode) {
				t.Errorf("no-op move must not emit %s, got diags: %v", tt.wantCode, diags)
			}
		})
	}
}
