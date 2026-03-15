package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// mockMoveIO is a test double for MoveIO.
type mockMoveIO struct {
	binderBytes  []byte
	project      *binder.Project
	binderErr    error
	projectErr   error
	writeErr     error
	writtenBytes []byte
	writtenPath  string
}

func (m *mockMoveIO) ReadBinder(_ context.Context, _ string) ([]byte, error) {
	return m.binderBytes, m.binderErr
}

func (m *mockMoveIO) ScanProject(_ context.Context, _ string) (*binder.Project, error) {
	if m.project != nil {
		return m.project, m.projectErr
	}
	return &binder.Project{Files: []string{}, BinderDir: "."}, m.projectErr
}

func (m *mockMoveIO) WriteBinderAtomic(_ context.Context, path string, data []byte) error {
	m.writtenPath = path
	m.writtenBytes = data
	return m.writeErr
}

// moveBinder returns a minimal binder with two child nodes for move tests.
func moveBinder() []byte {
	return []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](chapter-one.md)\n- [Chapter Two](chapter-two.md)\n")
}

func TestNewMoveCmd_HasRequiredFlags(t *testing.T) {
	c := NewMoveCmd(nil)
	required := []string{"project", "source", "dest", "first", "at", "before", "after", "yes", "json"}
	for _, name := range required {
		name := name
		t.Run(name, func(t *testing.T) {
			if c.Flags().Lookup(name) == nil {
				t.Errorf("expected --%s flag on move command", name)
			}
		})
	}
}

func TestNewMoveCmd_DefaultsToCWD(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes"})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected success with no --project (CWD default): %v", err)
	}
}

func TestNewMoveCmd_AcceptsProjectFlag(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--project", "/some/dir"})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected success with --project flag: %v", err)
	}
}

func TestNewMoveCmd_GetCWDError(t *testing.T) {
	mock := &mockMoveIO{binderBytes: moveBinder()}
	c := newMoveCmdWithGetCWD(mock, func() (string, error) {
		return "", errors.New("getwd failed")
	})
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when getwd fails")
	}
}

func TestNewMoveCmd_PrintsConfirmationOnSuccess(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Moved chapter-two.md in _binder.md") {
		t.Errorf("expected stdout to contain confirmation, got: %s", out.String())
	}
}

func TestNewMoveCmd_FirstFlagSetsPositionFirst(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--first", "--yes", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --first flag: %v", err)
	}
}

func TestNewMoveCmd_WritesModifiedBinderOnChange(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.writtenPath != "_binder.md" {
		t.Errorf("written to %q, want \"_binder.md\"", mock.writtenPath)
	}
	if len(mock.writtenBytes) == 0 {
		t.Error("expected non-empty bytes written to binder")
	}
}

func TestNewMoveCmd_ReadBinderError(t *testing.T) {
	mock := &mockMoveIO{binderErr: errors.New("disk error")}
	c := NewMoveCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ReadBinder fails")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout on ReadBinder error, got: %s", out.String())
	}
}

func TestNewMoveCmd_ScanProjectError(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		projectErr:  errors.New("disk error"),
	}
	c := NewMoveCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ScanProject fails")
	}
}

func TestNewMoveCmd_WriteError(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
		writeErr:    errors.New("write failed"),
	}
	c := NewMoveCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when WriteBinderAtomic fails")
	}
}

func TestNewMoveCmd_ExitsNonZeroOnOpErrors(t *testing.T) {
	// Selector that matches no node → OPE001 (error-severity diagnostic)
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
	}
	c := NewMoveCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--source", "nonexistent.md", "--dest", "chapter-one.md", "--yes", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Error("expected non-zero exit when op has error diagnostics")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout on op error, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "OPE001") {
		t.Errorf("expected errOut to contain OPE001, got: %s", errOut.String())
	}
	// Binder must NOT be written when op has error diagnostics
	if mock.writtenPath != "" {
		t.Errorf("binder must not be written when op has error diagnostics, was written to %q", mock.writtenPath)
	}
}

func TestNewMoveCmd_MissingYesFlagReturnsError(t *testing.T) {
	// Omitting --yes → ops.Move returns OPE009 (missing confirmation)
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --yes flag is missing (OPE011)")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout for OPE011 error, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "OPE011") {
		t.Errorf("expected errOut to contain OPE011, got: %s", errOut.String())
	}
}

// TestNewMoveCmd_MissingYes_ExitCode1_AndSuggestion verifies that
// missing --yes returns exit code 1 (usage) with a suggestion mentioning --yes,
// not exit code 6 (transient) with a misleading _binder.md suggestion.
func TestNewMoveCmd_MissingYes_ExitCode1_AndSuggestion(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when --yes flag is missing")
	}

	// Exit code must be 1 (usage), not 6 (transient).
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitUsage {
		t.Errorf("exit code = %d, want %d (ExitUsage)", exitErr.Code, ExitUsage)
	}

	// Stderr must mention --yes, not _binder.md.
	stderr := errOut.String()
	if !strings.Contains(stderr, "--yes") {
		t.Errorf("expected stderr to mention --yes, got: %s", stderr)
	}
	if strings.Contains(stderr, "_binder.md") {
		t.Errorf("stderr must NOT suggest checking _binder.md for missing --yes: %s", stderr)
	}
}

// TestNewMoveCmd_MissingYes_JSON_EmitsOPE011 verifies that the JSON output
// for missing --yes contains OPE011 (not OPE009) with a --yes suggestion.
func TestNewMoveCmd_MissingYes_JSON_EmitsOPE011(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--json", "--project", "."})

	_ = c.Execute()

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %s", out.String())
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic")
	}
	d := result.Diagnostics[0]
	if d.Code != binder.CodeMissingConfirmation {
		t.Errorf("diagnostic code = %q, want %q (CodeMissingConfirmation)", d.Code, binder.CodeMissingConfirmation)
	}
	if !strings.Contains(d.Suggestion, "--yes") {
		t.Errorf("suggestion should mention --yes, got: %q", d.Suggestion)
	}
}

func TestNewMoveCmd_WriteSuccessMessageError(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
	}
	c := NewMoveCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when writing success message fails")
	}
}

func TestNewMoveCmd_AtFlag(t *testing.T) {
	// --at 0 moves the source as the first child of dest.
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", ".", "--at", "0", "--yes", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --at flag: %v", err)
	}
}

func TestNewMoveCmd_OutputsOpResultJSONOnSuccess(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--json", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result.Version != "1" {
		t.Errorf("version = %q, want \"1\"", result.Version)
	}
}

func TestNewMoveCmd_JSONEncodeError(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--json", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when JSON encoding fails")
	}
}

func TestNewMoveCmd_ScanProjectErrorWithJSON(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		projectErr:  errors.New("disk error"),
	}
	c := NewMoveCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--json", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ScanProject fails")
	}

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("expected OPE009 JSON on stdout, got: %s", out.String())
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != binder.CodeIOOrParseFailure {
		t.Errorf("expected OPE009 diagnostic, got: %v", result.Diagnostics)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// --source omitted: should report missing flag, not OPE001
// ──────────────────────────────────────────────────────────────────────────────

func TestNewMoveCmd_MissingRequiredFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantFlag string
		wantCode int
	}{
		{
			name:     "missing --source",
			args:     []string{"--dest", "chapter-one.md", "--yes", "--project", "."},
			wantFlag: "--source",
			wantCode: ExitUsage,
		},
		{
			name:     "missing --dest",
			args:     []string{"--source", "chapter-two.md", "--yes", "--project", "."},
			wantFlag: "--dest",
			wantCode: ExitUsage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMoveIO{
				binderBytes: moveBinder(),
				project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
			}
			c := NewMoveCmd(mock)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs(tt.args)

			err := c.Execute()
			if err == nil {
				t.Fatalf("expected error when %s is omitted", tt.wantFlag)
			}

			// Exit code must be ExitUsage (1), not ExitNotFound (3).
			var exitErr *ExitError
			if errors.As(err, &exitErr) {
				if exitErr.Code != tt.wantCode {
					t.Errorf("exit code = %d, want %d (ExitUsage)", exitErr.Code, tt.wantCode)
				}
			}

			// Stderr must mention the missing flag.
			stderr := errOut.String()
			if !strings.Contains(stderr, tt.wantFlag) {
				t.Errorf("expected stderr to contain %q, got: %s", tt.wantFlag, stderr)
			}

			// Must NOT contain OPE001 (selector-not-found) — that's the misleading error.
			if strings.Contains(stderr, binder.CodeSelectorNoMatch) {
				t.Errorf("stderr must NOT contain %s when %s is omitted: %s", binder.CodeSelectorNoMatch, tt.wantFlag, stderr)
			}
		})
	}
}

func TestNewMoveCmd_MissingRequiredFlag_JSON(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantFlag string
	}{
		{
			name:     "missing --source",
			args:     []string{"--dest", "chapter-one.md", "--yes", "--json", "--project", "."},
			wantFlag: "--source",
		},
		{
			name:     "missing --dest",
			args:     []string{"--source", "chapter-two.md", "--yes", "--json", "--project", "."},
			wantFlag: "--dest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMoveIO{
				binderBytes: moveBinder(),
				project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
			}
			c := NewMoveCmd(mock)
			out := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(new(bytes.Buffer))
			c.SetArgs(tt.args)

			_ = c.Execute()

			var result binder.OpResult
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("expected valid JSON, got: %s", out.String())
			}
			if len(result.Diagnostics) == 0 {
				t.Fatal("expected at least one diagnostic")
			}
			d := result.Diagnostics[0]
			// Must NOT be OPE001 (selector-not-found).
			if d.Code == binder.CodeSelectorNoMatch {
				t.Errorf("diagnostic code must NOT be %q when %s is omitted", binder.CodeSelectorNoMatch, tt.wantFlag)
			}
			// Message should mention the missing flag.
			if !strings.Contains(d.Message, tt.wantFlag) {
				t.Errorf("diagnostic message should mention %s, got: %q", tt.wantFlag, d.Message)
			}
		})
	}
}

func TestNewMoveCmd_ConflictingPositionFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "first and at conflict",
			args: []string{"--source", "chapter-two.md", "--dest", ".", "--first", "--at", "0", "--yes", "--project", "."},
		},
		{
			name: "first and before conflict",
			args: []string{"--source", "chapter-two.md", "--dest", ".", "--first", "--before", "chapter-one.md", "--yes", "--project", "."},
		},
		{
			name: "first and after conflict",
			args: []string{"--source", "chapter-two.md", "--dest", ".", "--first", "--after", "chapter-one.md", "--yes", "--project", "."},
		},
		{
			name: "before and after conflict",
			args: []string{"--source", "chapter-two.md", "--dest", ".", "--before", "chapter-one.md", "--after", "chapter-one.md", "--yes", "--project", "."},
		},
		{
			name: "at and before conflict",
			args: []string{"--source", "chapter-two.md", "--dest", ".", "--at", "0", "--before", "chapter-one.md", "--yes", "--project", "."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMoveIO{
				binderBytes: moveBinder(),
				project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
			}
			c := NewMoveCmd(mock)
			c.SetOut(new(bytes.Buffer))
			c.SetErr(new(bytes.Buffer))
			c.SetArgs(tt.args)

			err := c.Execute()
			if err == nil {
				t.Errorf("expected error for conflicting position flags (%s)", tt.name)
				return
			}
			if !strings.Contains(err.Error(), binder.CodeConflictingFlags) {
				t.Errorf("expected error to contain %s, got: %v", binder.CodeConflictingFlags, err)
			}
		})
	}
}

func TestNewRootCmd_RegistersMoveSubcommand(t *testing.T) {
	root := NewRootCmd()
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "move" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected \"move\" subcommand registered on root command")
	}
}
