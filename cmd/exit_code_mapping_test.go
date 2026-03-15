package cmd

import (
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

func TestExitCodeForDiagnostics_NilInput(t *testing.T) {
	got := ExitCodeForDiagnostics(nil)
	if got != 0 {
		t.Errorf("ExitCodeForDiagnostics(nil) = %d, want 0", got)
	}
}

func TestExitCodeForDiagnostics_EmptyInput(t *testing.T) {
	got := ExitCodeForDiagnostics([]binder.Diagnostic{})
	if got != 0 {
		t.Errorf("ExitCodeForDiagnostics([]) = %d, want 0", got)
	}
}

func TestExitCodeForDiagnostics_WarningOnlyReturnsZero(t *testing.T) {
	diags := []binder.Diagnostic{
		{Severity: "warning", Code: "OPW001", Message: "multi match"},
		{Severity: "warning", Code: "BNDW001", Message: "missing pragma"},
	}
	got := ExitCodeForDiagnostics(diags)
	if got != 0 {
		t.Errorf("ExitCodeForDiagnostics(warnings only) = %d, want 0", got)
	}
}

func TestExitCodeForDiagnostics_MappedCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantExit int
	}{
		// ExitUsage (1) — conflicting flags
		{"OPE010 -> 1", binder.CodeConflictingFlags, ExitUsage},

		// ExitValidation (2) — validation errors
		{"OPE004 -> 2", binder.CodeInvalidTargetPath, ExitValidation},
		{"OPE005 -> 2", binder.CodeTargetIsBinder, ExitValidation},
		{"BNDE001 -> 2", binder.CodeIllegalPathChars, ExitValidation},
		{"BNDE002 -> 2", binder.CodePathEscapesRoot, ExitValidation},
		{"BNDE003 -> 2", binder.CodeAmbiguousWikilink, ExitValidation},

		// ExitNotFound (3) — not found errors
		{"OPE001 -> 3", binder.CodeSelectorNoMatch, ExitNotFound},
		{"OPE007 -> 3", binder.CodeSiblingNotFound, ExitNotFound},
		{"OPE008 -> 3", binder.CodeIndexOutOfBounds, ExitNotFound},

		// ExitConflict (5) — conflict/cycle errors
		{"OPE002 -> 5", binder.CodeAmbiguousBareStem, ExitConflict},
		{"OPE003 -> 5", binder.CodeCycleDetected, ExitConflict},
		{"OPE006 -> 5", binder.CodeNodeInCodeFence, ExitConflict},
		{"OPE014 -> 5", binder.CodeSelfMove, ExitConflict},

		// ExitTransient (6) — IO/parse failure
		{"OPE009 -> 6", binder.CodeIOOrParseFailure, ExitTransient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := []binder.Diagnostic{
				{Severity: "error", Code: tt.code, Message: "test"},
			}
			got := ExitCodeForDiagnostics(diags)
			if got != tt.wantExit {
				t.Errorf("ExitCodeForDiagnostics(%s) = %d, want %d", tt.code, got, tt.wantExit)
			}
		})
	}
}

func TestExitCodeForDiagnostics_OPE011_MapsToUsage(t *testing.T) {
	diags := []binder.Diagnostic{
		{Severity: "error", Code: binder.CodeMissingConfirmation, Message: "requires --yes"},
	}
	got := ExitCodeForDiagnostics(diags)
	if got != ExitUsage {
		t.Errorf("ExitCodeForDiagnostics(%s) = %d, want %d (ExitUsage)", binder.CodeMissingConfirmation, got, ExitUsage)
	}
}

func TestExitCodeForDiagnostics_FirstErrorWins(t *testing.T) {
	diags := []binder.Diagnostic{
		{Severity: "warning", Code: "OPW001", Message: "skip me"},
		{Severity: "error", Code: binder.CodeSelectorNoMatch, Message: "first error"},   // -> 3
		{Severity: "error", Code: binder.CodeConflictingFlags, Message: "second error"}, // -> 1
	}
	got := ExitCodeForDiagnostics(diags)
	if got != ExitNotFound {
		t.Errorf("ExitCodeForDiagnostics(first error wins) = %d, want %d", got, ExitNotFound)
	}
}

func TestExitCodeForDiagnostics_UnmappedErrorCodeDefaultsToOne(t *testing.T) {
	diags := []binder.Diagnostic{
		{Severity: "error", Code: "UNKNOWN999", Message: "unknown code"},
	}
	got := ExitCodeForDiagnostics(diags)
	if got != ExitUsage {
		t.Errorf("ExitCodeForDiagnostics(unmapped) = %d, want %d", got, ExitUsage)
	}
}
