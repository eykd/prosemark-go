package cmd

import (
	"fmt"

	"github.com/eykd/prosemark-go/internal/binder"
	"github.com/eykd/prosemark-go/internal/node"
)

// hasSeverityError is the canonical check: true when sev matches the error severity constant.
func hasSeverityError(sev string) bool {
	return sev == string(node.SeverityError)
}

// hasDiagnosticError reports whether any binder.Diagnostic in diags has error severity.
func hasDiagnosticError(diags []binder.Diagnostic) bool {
	for _, d := range diags {
		if hasSeverityError(d.Severity) {
			return true
		}
	}
	return false
}

// diagnosticExitError returns an ExitError for the given diagnostics. In JSON
// mode the error message is suppressed (Err is nil) because the JSON output
// already contains the diagnostics. cmdName is used in the human-readable
// message (e.g. "add has errors").
func diagnosticExitError(cmdName string, jsonMode bool, diags []binder.Diagnostic) *ExitError {
	exitErr := &ExitError{Code: ExitCodeForDiagnostics(diags)}
	if !jsonMode {
		exitErr.Err = fmt.Errorf("%s has errors", cmdName)
	}
	return exitErr
}

// hasAuditDiagnosticError reports whether any node.AuditDiagnostic in diags has error severity.
func hasAuditDiagnosticError(diags []node.AuditDiagnostic) bool {
	for _, d := range diags {
		if hasSeverityError(string(d.Severity)) {
			return true
		}
	}
	return false
}
