package cmd

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitCmd_NonexistentProjectDir verifies that pmk init --project /nonexistent
// returns a clear "directory does not exist" error with an appropriate exit code,
// rather than exposing internal temp file paths with ExitTransient (6).
func TestInitCmd_NonexistentProjectDir(t *testing.T) {
	tests := []struct {
		name     string
		jsonMode bool
	}{
		{"human mode", false},
		{"json mode", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use real fileInitIO with a guaranteed-nonexistent directory
			// to exercise the actual error path.
			nonexistentDir := filepath.Join(t.TempDir(), "nonexistent")

			c := NewInitCmd(fileInitIO{})
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)

			args := []string{"--project", nonexistentDir}
			if tt.jsonMode {
				args = append(args, "--json")
			}
			c.SetArgs(args)

			err := c.Execute()
			if err == nil {
				t.Fatal("expected error for nonexistent project directory")
			}

			// Error should clearly indicate the directory does not exist.
			combined := err.Error() + errOut.String()
			if !strings.Contains(combined, "does not exist") {
				t.Errorf("error output should say 'does not exist', got error=%q stderr=%q",
					err.Error(), errOut.String())
			}

			// Error should NOT expose internal temp file implementation details.
			if strings.Contains(combined, "temp file") {
				t.Errorf("error should not expose temp file internals, got error=%q stderr=%q",
					err.Error(), errOut.String())
			}

			// A nonexistent directory is NOT a transient error.
			var exitErr *ExitError
			if errors.As(err, &exitErr) && exitErr.Code == ExitTransient {
				t.Errorf("exit code = %d (ExitTransient), nonexistent directory is not transient",
					exitErr.Code)
			}
		})
	}
}

// TestInitCmd_NonexistentProjectDir_NoFileIO verifies that the command rejects
// a nonexistent project directory before attempting any file I/O operations.
func TestInitCmd_NonexistentProjectDir_NoFileIO(t *testing.T) {
	mock := newMockInitIO()
	nonexistentDir := "/tmp/prosemark-nonexistent-" + t.Name()

	c := newInitCmdWithGetCWD(mock, func() (string, error) { return nonexistentDir, nil })
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--project", nonexistentDir})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent project directory, but command succeeded")
	}

	// No files should have been written — the directory check should happen first.
	if len(mock.written) > 0 {
		t.Errorf("no files should be written when project directory does not exist, got: %v",
			mock.written)
	}
}
