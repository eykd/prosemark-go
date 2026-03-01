package cmd

import (
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
	"github.com/eykd/prosemark-go/internal/node"
)

// TestHasSeverityError verifies the single canonical severity predicate that
// both hasDiagnosticError and hasErrorDiagnostic should delegate to.
//
// RED: hasSeverityError does not exist yet; this file will not compile until
// cmd/diagnostics.go defines it.
func TestHasSeverityError(t *testing.T) {
	tests := []struct {
		name string
		sev  string
		want bool
	}{
		{"error constant matches", string(node.SeverityError), true},
		{"warning is not error", string(node.SeverityWarning), false},
		{"empty string is not error", "", false},
		{"unrelated string is not error", "critical", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasSeverityError(tt.sev); got != tt.want {
				t.Errorf("hasSeverityError(%q) = %v, want %v", tt.sev, got, tt.want)
			}
		})
	}
}

// TestHasDiagnosticError_DelegatesViaPrimitive verifies that hasDiagnosticError
// (the binder.Diagnostic predicate used in addchild.go) produces results consistent
// with the canonical hasSeverityError primitive.
func TestHasDiagnosticError_DelegatesViaPrimitive(t *testing.T) {
	tests := []struct {
		name  string
		diags []binder.Diagnostic
		want  bool
	}{
		{"nil slice", nil, false},
		{"empty slice", []binder.Diagnostic{}, false},
		{"single error", []binder.Diagnostic{{Severity: string(node.SeverityError)}}, true},
		{"single warning", []binder.Diagnostic{{Severity: string(node.SeverityWarning)}}, false},
		{"error among warnings", []binder.Diagnostic{
			{Severity: string(node.SeverityWarning)},
			{Severity: string(node.SeverityError)},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasDiagnosticError(tt.diags); got != tt.want {
				t.Errorf("hasDiagnosticError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHasErrorDiagnostic_DelegatesViaPrimitive verifies that hasErrorDiagnostic
// (the node.AuditDiagnostic predicate used in doctor.go) produces results consistent
// with the canonical hasSeverityError primitive.
func TestHasErrorDiagnostic_DelegatesViaPrimitive(t *testing.T) {
	tests := []struct {
		name  string
		diags []node.AuditDiagnostic
		want  bool
	}{
		{"nil slice", nil, false},
		{"empty slice", []node.AuditDiagnostic{}, false},
		{"single error", []node.AuditDiagnostic{{Severity: node.SeverityError}}, true},
		{"single warning", []node.AuditDiagnostic{{Severity: node.SeverityWarning}}, false},
		{"error among warnings", []node.AuditDiagnostic{
			{Severity: node.SeverityWarning},
			{Severity: node.SeverityError},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasErrorDiagnostic(tt.diags); got != tt.want {
				t.Errorf("hasErrorDiagnostic() = %v, want %v", got, tt.want)
			}
		})
	}
}
