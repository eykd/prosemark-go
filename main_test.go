package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/eykd/prosemark-go/cmd"
)

func TestHandleCommandError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantStderr string
	}{
		{
			name:       "nil error returns 0",
			err:        nil,
			wantCode:   0,
			wantStderr: "",
		},
		{
			name:       "plain error prints message and returns 1",
			err:        fmt.Errorf("something broke"),
			wantCode:   1,
			wantStderr: "something broke\n",
		},
		{
			name:       "ExitError prints inner message and returns code",
			err:        &cmd.ExitError{Code: 2, Err: fmt.Errorf("validation failed")},
			wantCode:   2,
			wantStderr: "validation failed\n",
		},
		{
			name:       "wrapped ExitError extracted via errors.As",
			err:        fmt.Errorf("command failed: %w", &cmd.ExitError{Code: 3, Err: fmt.Errorf("not found")}),
			wantCode:   3,
			wantStderr: "not found\n",
		},
		{
			name:       "ExitError with code 0 returns 0",
			err:        &cmd.ExitError{Code: 0, Err: fmt.Errorf("success with message")},
			wantCode:   0,
			wantStderr: "success with message\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := new(bytes.Buffer)

			gotCode := handleCommandError(tt.err, stderr)

			if gotCode != tt.wantCode {
				t.Errorf("handleCommandError() code = %d, want %d", gotCode, tt.wantCode)
			}
			if stderr.String() != tt.wantStderr {
				t.Errorf("handleCommandError() stderr = %q, want %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
