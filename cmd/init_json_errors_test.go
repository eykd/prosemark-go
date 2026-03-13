package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestInitCmd_JSON_ErrorResponses verifies that when --json is specified,
// error conditions produce JSON-formatted output (OpResult with error
// diagnostics) rather than plain text.
func TestInitCmd_JSON_ErrorResponses(t *testing.T) {
	tests := []struct {
		name          string
		binderExists  bool
		binderStatErr error
		configStatErr error
		writeErrFor   map[string]error
		force         bool
		wantErrCode   string // expected diagnostic code, empty = any error diagnostic
	}{
		{
			name:         "binder already exists emits JSON error",
			binderExists: true,
			wantErrCode:  "",
		},
		{
			name:          "binder stat error emits JSON error",
			binderStatErr: errors.New("permission denied"),
			wantErrCode:   binder.CodeIOOrParseFailure,
		},
		{
			name:        "binder write error emits JSON error",
			writeErrFor: map[string]error{"_binder.md": errors.New("disk full")},
			wantErrCode: binder.CodeIOOrParseFailure,
		},
		{
			name:          "config stat error emits JSON error",
			configStatErr: errors.New("permission denied"),
			wantErrCode:   binder.CodeIOOrParseFailure,
		},
		{
			name:        "config write error emits JSON error",
			writeErrFor: map[string]error{".prosemark.yml": errors.New("disk full")},
			wantErrCode: binder.CodeIOOrParseFailure,
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

			projectDir := t.TempDir()
			sub := newInitCmdWithGetCWD(mock, func() (string, error) { return projectDir, nil })
			root := withDryRunFlag(sub)
			out := new(bytes.Buffer)
			sub.SetOut(out)
			sub.SetErr(new(bytes.Buffer))

			args := []string{"init", "--project", projectDir, "--json"}
			if tt.force {
				args = append(args, "--force")
			}
			root.SetArgs(args)

			err := root.Execute()
			if err == nil {
				t.Fatal("expected error from init command")
			}

			// The critical assertion: stdout must contain valid JSON OpResult.
			var result binder.OpResult
			if jsonErr := json.Unmarshal(out.Bytes(), &result); jsonErr != nil {
				t.Fatalf("expected JSON OpResult on stdout, got: %q (unmarshal err: %v)", out.String(), jsonErr)
			}

			if result.Version != "1" {
				t.Errorf("Version = %q, want %q", result.Version, "1")
			}
			if result.Changed {
				t.Error("expected Changed=false on error")
			}

			// Must contain at least one error-severity diagnostic.
			var foundError bool
			for _, d := range result.Diagnostics {
				if d.Severity == "error" {
					foundError = true
					if tt.wantErrCode != "" && d.Code != tt.wantErrCode {
						t.Errorf("diagnostic code = %q, want %q", d.Code, tt.wantErrCode)
					}
					break
				}
			}
			if !foundError {
				t.Errorf("expected at least one error diagnostic, got: %v", result.Diagnostics)
			}
		})
	}
}
