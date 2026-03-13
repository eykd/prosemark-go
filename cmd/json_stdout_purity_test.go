package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/eykd/prosemark-go/internal/binder"
)

// simulateHandleCommandError mirrors main.go's handleCommandError: it writes
// the ExitError message to the provided stderr writer, just as production does.
// This lets cmd-package tests verify the full stdout+stderr picture.
func simulateHandleCommandError(err error, stderr *bytes.Buffer) {
	if err == nil {
		return
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		if exitErr.Err != nil {
			fmt.Fprintln(stderr, exitErr.Err)
		}
		return
	}
	fmt.Fprintln(stderr, err)
}

// withRootFlags creates a root command mirroring production settings
// (SilenceErrors: true) and adds sub as a child.
func withRootFlags(sub *cobra.Command, out, errBuf *bytes.Buffer) *cobra.Command {
	root := &cobra.Command{Use: "pmk", SilenceErrors: true}
	root.PersistentFlags().Bool("dry-run", false, "preview changes without writing to disk")
	root.AddCommand(sub)
	root.SetOut(out)
	root.SetErr(errBuf)
	return root
}

// TestJSONMode_DiagnosticErrors_NoHumanTextOnStderr verifies that when --json
// is set and operations produce diagnostic errors, no human-readable summary
// like "add has errors" is emitted to stderr. In --json mode, all error
// information should be in the JSON output on stdout; stderr text makes the
// combined output unparseable for agents that capture stdout+stderr together.
func TestJSONMode_DiagnosticErrors_NoHumanTextOnStderr(t *testing.T) {
	tests := []struct {
		name  string
		setup func(out, errBuf *bytes.Buffer) (*cobra.Command, []string)
	}{
		{
			name: "add with diagnostic error",
			setup: func(out, errBuf *bytes.Buffer) (*cobra.Command, []string) {
				mock := &mockAddChildIO{
					binderBytes: acBinder(),
					project:     &binder.Project{Files: []string{"_binder.md"}, BinderDir: "."},
				}
				sub := NewAddChildCmd(mock)
				root := withRootFlags(sub, out, errBuf)
				return root, []string{"add", "--parent", ".", "--target", "_binder.md", "--project", ".", "--json"}
			},
		},
		{
			name: "delete with diagnostic error",
			setup: func(out, errBuf *bytes.Buffer) (*cobra.Command, []string) {
				mock := &mockDeleteIO{
					binderBytes: delBinder(),
					project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
				}
				sub := NewDeleteCmd(mock)
				root := withRootFlags(sub, out, errBuf)
				return root, []string{"delete", "--selector", "nonexistent.md", "--yes", "--json", "--project", "."}
			},
		},
		{
			name: "move with diagnostic error",
			setup: func(out, errBuf *bytes.Buffer) (*cobra.Command, []string) {
				mock := &mockMoveIO{
					binderBytes: moveBinder(),
					project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
				}
				sub := NewMoveCmd(mock)
				root := withRootFlags(sub, out, errBuf)
				return root, []string{"move", "--source", "nonexistent.md", "--dest", ".", "--yes", "--json", "--project", "."}
			},
		},
		{
			name: "doctor with audit diagnostic error",
			setup: func(out, errBuf *bytes.Buffer) (*cobra.Command, []string) {
				mock := &mockDoctorIO{
					binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
					nodeFiles: map[string]nodeFileEntry{
						doctorTestNodeUUID + ".md": {content: nil, exists: false},
						".prosemark.yml":           {content: []byte("version: \"1\"\n"), exists: true},
					},
				}
				sub := NewDoctorCmd(mock)
				root := withRootFlags(sub, out, errBuf)
				return root, []string{"doctor", "--json", "--project", "."}
			},
		},
		{
			name: "parse with binder read error",
			setup: func(out, errBuf *bytes.Buffer) (*cobra.Command, []string) {
				mock := &mockParseReader{
					binderErr: errors.New("permission denied"),
				}
				sub := NewParseCmd(mock)
				root := withRootFlags(sub, out, errBuf)
				return root, []string{"parse", "--project", "."}
			},
		},
		{
			name: "parse with diagnostic error from invalid content",
			setup: func(out, errBuf *bytes.Buffer) (*cobra.Command, []string) {
				mock := &mockParseReader{
					// Path escaping root produces BNDE002 error diagnostic
					binderBytes: []byte("<!-- prosemark-binder:v1 -->\n- [Escape](../secret.md)\n"),
				}
				sub := NewParseCmd(mock)
				root := withRootFlags(sub, out, errBuf)
				return root, []string{"parse", "--project", "."}
			},
		},
		{
			name: "doctor with binder read error in JSON mode",
			setup: func(out, errBuf *bytes.Buffer) (*cobra.Command, []string) {
				mock := &mockDoctorIO{
					binderErr: errors.New("permission denied"),
				}
				sub := NewDoctorCmd(mock)
				root := withRootFlags(sub, out, errBuf)
				return root, []string{"doctor", "--json", "--project", "."}
			},
		},
		{
			name: "init with binder-already-exists error",
			setup: func(out, errBuf *bytes.Buffer) (*cobra.Command, []string) {
				mock := newMockInitIO()
				mock.binderExists = true
				sub := newInitCmdWithGetCWD(mock, func() (string, error) { return "/tmp", nil })
				root := withRootFlags(sub, out, errBuf)
				return root, []string{"init", "--project", "/tmp", "--json"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			root, args := tt.setup(out, errBuf)
			root.SetArgs(args)

			err := root.Execute()
			if err == nil {
				t.Fatal("expected an error from the command")
			}

			// Simulate what main.go does with the returned error.
			simulateHandleCommandError(err, errBuf)

			// stdout must contain valid JSON.
			stdout := out.String()
			if stdout == "" {
				t.Fatal("expected JSON output on stdout, got empty string")
			}
			trimmed := strings.TrimSpace(stdout)
			if !json.Valid([]byte(trimmed)) {
				t.Errorf("stdout is not valid JSON: %q", trimmed)
			}

			// stderr must be empty in --json mode: all error information is
			// already encoded in the JSON diagnostics on stdout.
			stderr := errBuf.String()
			if stderr != "" {
				t.Errorf("--json mode must not emit human-readable text to stderr;\nstderr: %q", stderr)
			}
		})
	}
}
