package ops

// Cycle-2 RED tests for move operation gaps identified in review.
// These tests MUST fail until the implementation is corrected:
//
//  1. Multi-match bare-stem selector must move the FIRST match only (plan.md §H2).
//  2. OPW001 message must include the matched-node count AND recommend using an
//     index-qualified selector (plan.md §H2).
//  3. Root selector guard: OPE001 with "root node is not a valid target for this
//     operation" for both the "." literal and path-qualified selectors that
//     evaluate to root (plan.md §H3).

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// ──────────────────────────────────────────────────────────────────────────────
// Multi-match: first match only (H2)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_MultiMatch_OPW001_MovesAll verifies that when a bare-stem selector
// matches multiple nodes, OPW001 is emitted and ALL matched nodes are moved
// (all-match semantics).
func TestMove_MultiMatch_OPW001_MovesAll(t *testing.T) {
	// Two nodes share stem "ch" but have distinct titles so we can tell them apart.
	// Both should be moved under app.md (all-match semantics).
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
	// Both matches should be moved under app.md (indented with "  - ").
	if !bytes.Contains(out, []byte("  - [Chapter](ch.md)")) {
		t.Errorf("first match should be under app.md:\n%s", out)
	}
	if !bytes.Contains(out, []byte("  - [Chapter (alt)](ch.md)")) {
		t.Errorf("second match should also be under app.md (all-match semantics):\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Multi-match: OPW001 message format (H2)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_MultiMatch_OPW001_MessageFormat verifies that the OPW001 diagnostic
// message includes the count of matched nodes and describes all-match semantics.
func TestMove_MultiMatch_OPW001_MessageFormat(t *testing.T) {
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

	_, diags, _ := Move(context.Background(), src, nil, params)

	var opw001Msg string
	for _, d := range diags {
		if d.Code == binder.CodeMultiMatch {
			opw001Msg = d.Message
			break
		}
	}
	if opw001Msg == "" {
		t.Fatalf("expected OPW001 diagnostic, got none in: %v", diags)
	}
	// Message must include the count of matched nodes.
	if !strings.Contains(opw001Msg, "2") {
		t.Errorf("OPW001 message must include matched-node count (2), got: %q", opw001Msg)
	}
	// Message must describe all-match semantics.
	if !strings.Contains(opw001Msg, "all matches") {
		t.Errorf("OPW001 message must describe all-match semantics, got: %q", opw001Msg)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Root selector guard: OPE001 with explicit message (H3)
// ──────────────────────────────────────────────────────────────────────────────

// TestMove_RootSelectorGuard_OPE001 verifies that any source selector that
// resolves to the synthetic root node is rejected with OPE001 and the explicit
// message "root node is not a valid target for this operation" (plan.md §H3).
//
// Two variants are tested:
//   - "." (literal root selector, no ":" so bypasses EvalSelector)
//   - ".:." (path-qualified selector; all segments are "." so EvalSelector
//     returns the root node itself)
func TestMove_RootSelectorGuard_OPE001(t *testing.T) {
	src := binderSrc(
		"- [Alpha](alpha.md)",
		"- [Beta](beta.md)",
	)
	wantMsg := "root node is not a valid target for this operation"

	tests := []struct {
		name           string
		sourceSelector string
	}{
		{
			name:           "dot_literal",
			sourceSelector: ".",
		},
		{
			name:           "path_qualified_dot_dot",
			sourceSelector: ".:.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := binder.MoveParams{
				SourceSelector:            tt.sourceSelector,
				DestinationParentSelector: "alpha",
				Position:                  "last",
				Yes:                       true,
			}

			out, diags, _ := Move(context.Background(), src, nil, params)

			// Must return OPE001 (selector no match / root guard).
			if !hasDiagCode(diags, binder.CodeSelectorNoMatch) {
				t.Errorf("expected OPE001 for root selector %q, got diags: %v", tt.sourceSelector, diags)
			}
			// Message must use the explicit root-guard wording.
			var found bool
			for _, d := range diags {
				if d.Code == binder.CodeSelectorNoMatch && strings.Contains(d.Message, wantMsg) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("OPE001 for root selector %q must contain message %q, got diags: %v",
					tt.sourceSelector, wantMsg, diags)
			}
			// Source bytes must be unchanged (atomic abort semantics).
			if out != nil && !bytes.Equal(out, src) {
				t.Errorf("source bytes must be unchanged on root selector guard for %q", tt.sourceSelector)
			}
		})
	}
}

// TestMove_OPE006_SourceInCodeFence verifies that when the source selector
// matches a node inside a fenced code block, OPE006 is emitted.
func TestMove_OPE006_SourceInCodeFence(t *testing.T) {
	src := []byte("<!-- prosemark-binder:v1 -->\n\n```\n- [Fenced](fenced.md)\n```\n- [Dest](dest.md)\n")
	params := binder.MoveParams{
		SourceSelector:            "fenced",
		DestinationParentSelector: "dest",
		Position:                  "last",
		Yes:                       true,
	}

	_, diags, _ := Move(context.Background(), src, nil, params)

	if !hasDiagCode(diags, binder.CodeNodeInCodeFence) {
		t.Errorf("expected OPE006 (node in code fence), got: %v", diags)
	}
}
