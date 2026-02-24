package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	required := []string{"json", "source", "dest", "first", "at", "before", "after", "yes"}
	for _, name := range required {
		name := name
		t.Run(name, func(t *testing.T) {
			if c.Flags().Lookup(name) == nil {
				t.Errorf("expected --%s flag on move command", name)
			}
		})
	}
}

func TestNewMoveCmd_RejectsNon_binderMdFilename(t *testing.T) {
	tests := []struct {
		name       string
		binderPath string
		wantErr    bool
	}{
		{"valid _binder.md", "_binder.md", false},
		{"valid nested path", "project/_binder.md", false},
		{"invalid notes.md", "notes.md", true},
		{"invalid binder.md without underscore", "binder.md", true},
		{"invalid README.md", "README.md", true},
	}
	for _, tt := range tests {
		tt := tt
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
			c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", tt.binderPath})

			err := c.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && out.Len() > 0 {
				t.Errorf("expected no stdout output on filename validation error, got: %s", out.String())
			}
		})
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
	c.SetArgs([]string{"--json", "--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["version"] != "1" {
		t.Errorf("version = %v, want \"1\"", result["version"])
	}
	if _, ok := result["changed"]; !ok {
		t.Error("expected \"changed\" field in JSON output")
	}
	if _, ok := result["diagnostics"]; !ok {
		t.Error("expected \"diagnostics\" field in JSON output")
	}
}

func TestNewMoveCmd_FirstFlagSetsPositionFirst(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--json", "--source", "chapter-two.md", "--dest", "chapter-one.md", "--first", "--yes", "_binder.md"})

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
	c.SetArgs([]string{"--json", "--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "_binder.md"})

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
	c.SetArgs([]string{"--json", "--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "_binder.md"})

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
	c.SetArgs([]string{"--json", "--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "_binder.md"})

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
	c.SetArgs([]string{"--json", "--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "_binder.md"})

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
	c.SetArgs([]string{"--json", "--source", "nonexistent.md", "--dest", "chapter-one.md", "--yes", "_binder.md"})

	err := c.Execute()
	if err == nil {
		t.Error("expected non-zero exit when op has error diagnostics")
	}
	// JSON must still be written so callers can inspect diagnostics
	if out.Len() == 0 {
		t.Error("expected JSON written to stdout even when op has error diagnostics")
	}
	var result map[string]interface{}
	if jsonErr := json.Unmarshal(out.Bytes(), &result); jsonErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", jsonErr, out.String())
	}
	diags, ok := result["diagnostics"].([]interface{})
	if !ok || len(diags) == 0 {
		t.Error("expected non-empty diagnostics array for op error case")
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
	c.SetOut(out)
	c.SetArgs([]string{"--json", "--source", "chapter-two.md", "--dest", "chapter-one.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --yes flag is missing (OPE009)")
	}
	// JSON output with OPE009 must still be written
	if out.Len() == 0 {
		t.Error("expected JSON written to stdout even for OPE009 error")
	}
}

func TestNewMoveCmd_EncodeError(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
	}
	c := NewMoveCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--json", "--source", "nonexistent.md", "--dest", "chapter-one.md", "--yes", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when JSON encoding fails")
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
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", ".", "--at", "0", "--yes", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --at flag: %v", err)
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
