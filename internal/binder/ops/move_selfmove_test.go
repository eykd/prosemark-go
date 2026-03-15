package ops

import (
	"bytes"
	"context"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// ──────────────────────────────────────────────────────────────────────────────
// Self-move detection: OPE014
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_SelfMove_OPE014 verifies that moving a node to itself produces a
// clear OPE014 diagnostic ("source and destination are the same node") instead
// of the misleading OPE003 cycle-detected error.
func TestMove_SelfMove_OPE014(t *testing.T) {
	tests := []struct {
		name   string
		src    []byte
		params binder.MoveParams
	}{
		{
			name: "leaf node moved to itself",
			src: binderSrc(
				"- [Alpha](alpha.md)",
				"- [Beta](beta.md)",
			),
			params: binder.MoveParams{
				SourceSelector:            "alpha.md",
				DestinationParentSelector: "alpha.md",
				Position:                  "last",
				Yes:                       true,
			},
		},
		{
			name: "parent node moved to itself",
			src: binderSrc(
				"- [Alpha](alpha.md)",
				"  - [Child](child.md)",
				"- [Beta](beta.md)",
			),
			params: binder.MoveParams{
				SourceSelector:            "alpha.md",
				DestinationParentSelector: "alpha.md",
				Position:                  "last",
				Yes:                       true,
			},
		},
		{
			name: "bare stem selector matches same node",
			src: binderSrc(
				"- [Alpha](alpha.md)",
				"- [Beta](beta.md)",
			),
			params: binder.MoveParams{
				SourceSelector:            "alpha",
				DestinationParentSelector: "alpha",
				Position:                  "last",
				Yes:                       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, diags := Move(context.Background(), tt.src, nil, tt.params)

			if hasDiagCode(diags, binder.CodeCycleDetected) {
				t.Errorf("should NOT emit OPE003 (cycle detected) for self-move; got: %v", diags)
			}
			if !hasDiagCode(diags, binder.CodeSelfMove) {
				t.Errorf("expected OPE014 (self-move), got: %v", diags)
			}
			if out != nil && !bytes.Equal(out, tt.src) {
				t.Errorf("source bytes must be unchanged on OPE014 abort:\ngot:  %q\nwant: %q", out, tt.src)
			}
		})
	}
}
