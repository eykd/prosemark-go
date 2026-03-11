package cmd

import (
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

func TestAttachSuggestions_MappedCodes(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		// OPE codes
		{"OPE001", binder.CodeSelectorNoMatch, "Run 'pmk parse --json' to list available nodes and their selectors."},
		{"OPE002", binder.CodeAmbiguousBareStem, "Use a full path selector (e.g., 'parent/child.md') to disambiguate."},
		{"OPE003", binder.CodeCycleDetected, "The destination is a descendant of the source. Choose a different destination."},
		{"OPE004", binder.CodeInvalidTargetPath, "Check that the target path contains only valid filename characters."},
		{"OPE005", binder.CodeTargetIsBinder, "The binder file cannot be added as a node. Choose a different target."},
		{"OPE006", binder.CodeNodeInCodeFence, "The node is inside a code fence. Move it outside the fenced block."},
		{"OPE007", binder.CodeSiblingNotFound, "The sibling selector matched no nodes. Run 'pmk parse --json' to verify."},
		{"OPE008", binder.CodeIndexOutOfBounds, "The index is out of bounds. Run 'pmk parse --json' to check child count."},
		{"OPE009", binder.CodeIOOrParseFailure, "Check that '_binder.md' exists and is readable. Run 'pmk doctor' to diagnose."},
		{"OPE010", binder.CodeConflictingFlags, "Specify only one positioning flag: --first, --at, --before, or --after."},
		// BNDE codes
		{"BNDE001", binder.CodeIllegalPathChars, "Remove illegal characters from the file path."},
		{"BNDE002", binder.CodePathEscapesRoot, "Paths must not escape the project root with '../'."},
		{"BNDE003", binder.CodeAmbiguousWikilink, "Use a full path instead of a wikilink to resolve the ambiguity."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := []binder.Diagnostic{
				{Severity: "error", Code: tt.code, Message: "some message"},
			}

			attachSuggestions(diags)

			if diags[0].Suggestion != tt.want {
				t.Errorf("attachSuggestions() suggestion = %q, want %q", diags[0].Suggestion, tt.want)
			}
		})
	}
}

func TestAttachSuggestions_OPE011_MentionsYesFlag(t *testing.T) {
	diags := []binder.Diagnostic{
		{Severity: "error", Code: binder.CodeMissingConfirmation, Message: "requires --yes"},
	}

	attachSuggestions(diags)

	if diags[0].Suggestion == "" {
		t.Fatal("expected non-empty suggestion for OPE011 (CodeMissingConfirmation)")
	}
	if got := diags[0].Suggestion; !strings.Contains(got, "--yes") {
		t.Errorf("suggestion for OPE011 should mention --yes, got: %q", got)
	}
}

func TestAttachSuggestions_UnknownCode(t *testing.T) {
	diags := []binder.Diagnostic{
		{Severity: "error", Code: "UNKNOWN999", Message: "unknown error"},
	}

	attachSuggestions(diags)

	if diags[0].Suggestion != "" {
		t.Errorf("attachSuggestions() suggestion = %q, want empty for unknown code", diags[0].Suggestion)
	}
}

func TestAttachSuggestions_PreservesExistingFields(t *testing.T) {
	diags := []binder.Diagnostic{
		{
			Severity: "error",
			Code:     binder.CodeSelectorNoMatch,
			Message:  "original message",
		},
	}

	attachSuggestions(diags)

	if diags[0].Severity != "error" {
		t.Errorf("Severity changed: got %q", diags[0].Severity)
	}
	if diags[0].Code != binder.CodeSelectorNoMatch {
		t.Errorf("Code changed: got %q", diags[0].Code)
	}
	if diags[0].Message != "original message" {
		t.Errorf("Message changed: got %q", diags[0].Message)
	}
}

func TestPrepareDiagnostics_NilBecomesEmpty(t *testing.T) {
	got := prepareDiagnostics(nil)
	if got == nil {
		t.Fatal("prepareDiagnostics(nil) returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("prepareDiagnostics(nil) returned %d items, want 0", len(got))
	}
}

func TestPrepareDiagnostics_AttachesSuggestions(t *testing.T) {
	diags := []binder.Diagnostic{
		{Severity: "error", Code: binder.CodeSelectorNoMatch, Message: "not found"},
	}
	got := prepareDiagnostics(diags)
	if got[0].Suggestion == "" {
		t.Error("prepareDiagnostics should attach suggestions to mapped codes")
	}
}

func TestAttachSuggestions_MultipleDiagnostics(t *testing.T) {
	diags := []binder.Diagnostic{
		{Severity: "error", Code: binder.CodeSelectorNoMatch, Message: "first"},
		{Severity: "warning", Code: "OPW001", Message: "second"},
		{Severity: "error", Code: binder.CodeIllegalPathChars, Message: "third"},
	}

	attachSuggestions(diags)

	if diags[0].Suggestion == "" {
		t.Error("first diagnostic should have suggestion")
	}
	if diags[1].Suggestion != "" {
		t.Errorf("warning code OPW001 should have no suggestion, got %q", diags[1].Suggestion)
	}
	if diags[2].Suggestion == "" {
		t.Error("third diagnostic should have suggestion")
	}
}
