package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestReadBinderError_JSONOutput verifies that when --json is specified and
// ReadBinder fails, each command emits a JSON error response rather than
// plain text.  Bug: early binder-read failures bypass --json flag.
func TestReadBinderError_JSONOutput(t *testing.T) {
	binderErr := errors.New("stat /nonexistent/_binder.md: no such file or directory")

	t.Run("parse emits JSON on ReadBinder error", func(t *testing.T) {
		reader := &mockParseReader{binderErr: binderErr}
		c := NewParseCmd(reader)
		out := new(bytes.Buffer)
		c.SetOut(out)
		c.SetErr(new(bytes.Buffer))
		c.SetArgs([]string{"--project", "/nonexistent"})

		_ = c.Execute() // error expected

		var result parseOutput
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatalf("expected JSON parseOutput on stdout, got: %q (unmarshal err: %v)", out.String(), err)
		}
		if len(result.Diagnostics) == 0 {
			t.Fatal("expected at least one diagnostic")
		}
		var foundError bool
		for _, d := range result.Diagnostics {
			if d.Severity == "error" {
				foundError = true
				if d.Code != binder.CodeIOOrParseFailure {
					t.Errorf("diagnostic code = %q, want %q", d.Code, binder.CodeIOOrParseFailure)
				}
				break
			}
		}
		if !foundError {
			t.Errorf("expected error-severity diagnostic, got: %v", result.Diagnostics)
		}
	})

	t.Run("add emits JSON on ReadBinder error", func(t *testing.T) {
		mock := &mockAddChildIO{binderErr: binderErr}
		c := NewAddChildCmd(mock)
		out := new(bytes.Buffer)
		c.SetOut(out)
		c.SetErr(new(bytes.Buffer))
		c.SetArgs([]string{"--parent", ".", "--target", "foo.md", "--json", "--project", "/nonexistent"})

		_ = c.Execute()

		var result binder.OpResult
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatalf("expected JSON OpResult on stdout, got: %q (unmarshal err: %v)", out.String(), err)
		}
		if len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != binder.CodeIOOrParseFailure {
			t.Errorf("expected OPE009 diagnostic, got: %v", result.Diagnostics)
		}
	})

	t.Run("delete emits JSON on ReadBinder error", func(t *testing.T) {
		mock := &mockDeleteIO{binderErr: binderErr}
		c := NewDeleteCmd(mock)
		out := new(bytes.Buffer)
		c.SetOut(out)
		c.SetErr(new(bytes.Buffer))
		c.SetArgs([]string{"--selector", "foo.md", "--yes", "--json", "--project", "/nonexistent"})

		_ = c.Execute()

		var result binder.OpResult
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatalf("expected JSON OpResult on stdout, got: %q (unmarshal err: %v)", out.String(), err)
		}
		if len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != binder.CodeIOOrParseFailure {
			t.Errorf("expected OPE009 diagnostic, got: %v", result.Diagnostics)
		}
	})

	t.Run("move emits JSON on ReadBinder error", func(t *testing.T) {
		mock := &mockMoveIO{binderErr: binderErr}
		c := NewMoveCmd(mock)
		out := new(bytes.Buffer)
		c.SetOut(out)
		c.SetErr(new(bytes.Buffer))
		c.SetArgs([]string{"--source", "foo.md", "--dest", ".", "--yes", "--json", "--project", "/nonexistent"})

		_ = c.Execute()

		var result binder.OpResult
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatalf("expected JSON OpResult on stdout, got: %q (unmarshal err: %v)", out.String(), err)
		}
		if len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != binder.CodeIOOrParseFailure {
			t.Errorf("expected OPE009 diagnostic, got: %v", result.Diagnostics)
		}
	})

	t.Run("doctor emits JSON on ReadBinder error", func(t *testing.T) {
		mock := &mockDoctorIO{binderErr: errors.New("permission denied")}
		c := NewDoctorCmd(mock)
		out := new(bytes.Buffer)
		c.SetOut(out)
		c.SetErr(new(bytes.Buffer))
		c.SetArgs([]string{"--json", "--project", "/nonexistent"})

		_ = c.Execute()

		var result doctorOutput
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatalf("expected JSON doctorOutput on stdout, got: %q (unmarshal err: %v)", out.String(), err)
		}
		if len(result.Diagnostics) == 0 {
			t.Fatal("expected at least one diagnostic")
		}
		var foundError bool
		for _, d := range result.Diagnostics {
			if d.Severity == "error" {
				foundError = true
				break
			}
		}
		if !foundError {
			t.Errorf("expected error-severity diagnostic, got: %v", result.Diagnostics)
		}
	})
}
