package cmd

import (
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

// hasErrorDiagnostic reports whether any node.AuditDiagnostic in diags has error severity.
func hasErrorDiagnostic(diags []node.AuditDiagnostic) bool {
	for _, d := range diags {
		if hasSeverityError(string(d.Severity)) {
			return true
		}
	}
	return false
}
