package cmd

import "github.com/eykd/prosemark-go/internal/binder"

// Exit code constants for CLI operations.
const (
	ExitSuccess    = 0
	ExitUsage      = 1
	ExitValidation = 2
	ExitNotFound   = 3
	ExitConflict   = 5
	ExitTransient  = 6
)

// diagnosticExitMap maps diagnostic codes to exit codes.
var diagnosticExitMap = map[string]int{
	binder.CodeConflictingFlags:  ExitUsage,
	binder.CodeInvalidTargetPath: ExitValidation,
	binder.CodeTargetIsBinder:    ExitValidation,
	binder.CodeIllegalPathChars:  ExitValidation,
	binder.CodePathEscapesRoot:   ExitValidation,
	binder.CodeAmbiguousWikilink: ExitValidation,
	binder.CodeSelectorNoMatch:   ExitNotFound,
	binder.CodeSiblingNotFound:   ExitNotFound,
	binder.CodeIndexOutOfBounds:  ExitNotFound,
	binder.CodeAmbiguousBareStem: ExitConflict,
	binder.CodeCycleDetected:     ExitConflict,
	binder.CodeNodeInCodeFence:   ExitConflict,
	binder.CodeIOOrParseFailure:  ExitTransient,
}

// ExitCodeForDiagnostics returns the exit code for the first error diagnostic.
// Warning-only or empty input returns 0. Unmapped error codes default to 1.
func ExitCodeForDiagnostics(diags []binder.Diagnostic) int {
	for _, d := range diags {
		if d.Severity != "error" {
			continue
		}
		if code, ok := diagnosticExitMap[d.Code]; ok {
			return code
		}
		return ExitUsage
	}
	return ExitSuccess
}

// ExitError represents a CLI error with a specific exit code.
type ExitError struct {
	Code int
	Err  error
}

// Error delegates to the inner error's message.
func (e *ExitError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the inner error for use with errors.Is/As.
func (e *ExitError) Unwrap() error {
	return e.Err
}
