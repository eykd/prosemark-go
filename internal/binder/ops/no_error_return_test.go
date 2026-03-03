package ops

// Tests verifying that AddChild, Delete, and Move do not return an error value.
// These tests fail to compile until the signatures are updated to return
// ([]byte, []binder.Diagnostic) instead of ([]byte, []binder.Diagnostic, error).

import (
	"context"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestAddChild_NoErrorReturn verifies that AddChild returns only
// ([]byte, []binder.Diagnostic) with no error in the signature.
func TestAddChild_NoErrorReturn(t *testing.T) {
	src := binderSrc("- [Chapter One](chapter-one.md)")
	params := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "chapter-two.md",
		Title:          "Chapter Two",
		Position:       "last",
	}

	out, diags := AddChild(context.Background(), src, nil, params)

	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

// TestDelete_NoErrorReturn verifies that Delete returns only
// ([]byte, []binder.Diagnostic) with no error in the signature.
func TestDelete_NoErrorReturn(t *testing.T) {
	src := binderSrc(
		"- [Chapter One](chapter-one.md)",
		"- [Chapter Two](chapter-two.md)",
	)
	params := binder.DeleteParams{
		Selector: "chapter-two",
		Yes:      true,
	}

	out, diags := Delete(context.Background(), src, nil, params)

	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

// TestMove_NoErrorReturn verifies that Move returns only
// ([]byte, []binder.Diagnostic) with no error in the signature.
func TestMove_NoErrorReturn(t *testing.T) {
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

	out, diags := Move(context.Background(), src, nil, params)

	if hasDiagCode(diags, "error") {
		t.Errorf("unexpected error diagnostic: %v", diags)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}
