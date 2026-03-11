package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/eykd/prosemark-go/internal/node"
)

func TestExitError_Error_DelegatesToInnerError(t *testing.T) {
	inner := fmt.Errorf("something went wrong")
	exitErr := &ExitError{Code: 2, Err: inner}

	got := exitErr.Error()
	want := "something went wrong"

	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestExitError_Unwrap_ReturnsInnerError(t *testing.T) {
	inner := fmt.Errorf("inner error")
	exitErr := &ExitError{Code: 3, Err: inner}

	got := exitErr.Unwrap()

	if got != inner {
		t.Errorf("Unwrap() = %v, want %v", got, inner)
	}
}

func TestExitError_ImplementsErrorInterface(t *testing.T) {
	var _ error = &ExitError{Code: 1, Err: fmt.Errorf("test")}
}

func TestExitError_ExtractableViaErrorsAs(t *testing.T) {
	inner := fmt.Errorf("wrapped error")
	original := &ExitError{Code: 5, Err: inner}
	wrapped := fmt.Errorf("command failed: %w", original)

	var extracted *ExitError
	if !errors.As(wrapped, &extracted) {
		t.Fatal("errors.As failed to extract ExitError from wrapped error")
	}
	if extracted.Code != 5 {
		t.Errorf("extracted Code = %d, want 5", extracted.Code)
	}
	if extracted.Err != inner {
		t.Errorf("extracted Err = %v, want %v", extracted.Err, inner)
	}
}

func TestExitCodeForAuditDiagnostics(t *testing.T) {
	tests := []struct {
		name  string
		diags []node.AuditDiagnostic
		want  int
	}{
		{
			"empty input returns 0",
			nil,
			ExitSuccess,
		},
		{
			"warning only returns 0",
			[]node.AuditDiagnostic{
				{Code: node.AUD006, Severity: node.SeverityWarning, Message: "empty body"},
			},
			ExitSuccess,
		},
		{
			"AUD001 error returns 2",
			[]node.AuditDiagnostic{
				{Code: node.AUD001, Severity: node.SeverityError, Message: "missing file"},
			},
			ExitValidation,
		},
		{
			"AUD002 error returns 2",
			[]node.AuditDiagnostic{
				{Code: node.AUD002, Severity: node.SeverityError, Message: "orphaned node"},
			},
			ExitValidation,
		},
		{
			"AUD003 error returns 2",
			[]node.AuditDiagnostic{
				{Code: node.AUD003, Severity: node.SeverityError, Message: "duplicate ref"},
			},
			ExitValidation,
		},
		{
			"AUD004 error returns 2",
			[]node.AuditDiagnostic{
				{Code: node.AUD004, Severity: node.SeverityError, Message: "id mismatch"},
			},
			ExitValidation,
		},
		{
			"AUD005 error returns 2",
			[]node.AuditDiagnostic{
				{Code: node.AUD005, Severity: node.SeverityError, Message: "missing field"},
			},
			ExitValidation,
		},
		{
			"AUD007 error returns 2",
			[]node.AuditDiagnostic{
				{Code: node.AUD007, Severity: node.SeverityError, Message: "bad yaml"},
			},
			ExitValidation,
		},
		{
			"AUD008 error returns 2",
			[]node.AuditDiagnostic{
				{Code: node.AUD008, Severity: node.SeverityError, Message: "bad config"},
			},
			ExitValidation,
		},
		{
			"unmapped error code defaults to 1",
			[]node.AuditDiagnostic{
				{Code: "AUD999", Severity: node.SeverityError, Message: "unknown"},
			},
			ExitUsage,
		},
		{
			"warnings before error are skipped",
			[]node.AuditDiagnostic{
				{Code: node.AUD006, Severity: node.SeverityWarning, Message: "empty body"},
				{Code: node.AUD001, Severity: node.SeverityError, Message: "missing file"},
			},
			ExitValidation,
		},
		{
			"first error wins",
			[]node.AuditDiagnostic{
				{Code: node.AUD001, Severity: node.SeverityError, Message: "missing file"},
				{Code: "AUD999", Severity: node.SeverityError, Message: "unknown"},
			},
			ExitValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCodeForAuditDiagnostics(tt.diags)
			if got != tt.want {
				t.Errorf("ExitCodeForAuditDiagnostics() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExitCodeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant int
		want     int
	}{
		{"success", ExitSuccess, 0},
		{"usage", ExitUsage, 1},
		{"validation", ExitValidation, 2},
		{"not found", ExitNotFound, 3},
		{"conflict", ExitConflict, 5},
		{"transient IO", ExitTransient, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.constant, tt.want)
			}
		})
	}
}
