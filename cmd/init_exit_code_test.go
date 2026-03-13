package cmd

import (
	"bytes"
	"errors"
	"testing"
)

// TestInitCmd_OPE009_ReturnsExitTransient verifies that all OPE009 error paths
// in the init command return an ExitError with code ExitTransient (6), not a
// plain error that defaults to exit code 1.
func TestInitCmd_OPE009_ReturnsExitTransient(t *testing.T) {
	tests := []struct {
		name          string
		binderExists  bool
		binderStatErr error
		configStatErr error
		writeErrFor   map[string]error
	}{
		{
			name:         "binder already exists without force",
			binderExists: true,
		},
		{
			name:          "binder stat error",
			binderStatErr: errors.New("permission denied"),
		},
		{
			name:        "binder write error",
			writeErrFor: map[string]error{"_binder.md": errors.New("disk full")},
		},
		{
			name:          "config stat error",
			configStatErr: errors.New("permission denied"),
		},
		{
			name:        "config write error",
			writeErrFor: map[string]error{".prosemark.yml": errors.New("disk full")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockInitIO()
			mock.binderExists = tt.binderExists
			mock.binderStatErr = tt.binderStatErr
			mock.configStatErr = tt.configStatErr
			if tt.writeErrFor != nil {
				mock.writeErrFor = tt.writeErrFor
			}

			c := newInitCmdWithGetCWD(mock, func() (string, error) { return "/tmp/test", nil })
			c.SetOut(new(bytes.Buffer))
			c.SetErr(new(bytes.Buffer))
			c.SetArgs([]string{"--project", "/tmp/test"})

			err := c.Execute()
			if err == nil {
				t.Fatal("expected error from init command")
			}

			var exitErr *ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected ExitError, got plain error: %v", err)
			}
			if exitErr.Code != ExitTransient {
				t.Errorf("exit code = %d, want %d (ExitTransient)", exitErr.Code, ExitTransient)
			}
		})
	}
}
