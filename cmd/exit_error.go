package cmd

// Exit code constants for CLI operations.
const (
	ExitSuccess    = 0
	ExitUsage      = 1
	ExitValidation = 2
	ExitNotFound   = 3
	ExitConflict   = 5
	ExitTransient  = 6
)

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
