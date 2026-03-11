package cmd

import (
	"errors"
	"fmt"
	"testing"
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
