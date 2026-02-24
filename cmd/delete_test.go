package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// mockDeleteIO is a test double for DeleteIO.
type mockDeleteIO struct {
	binderBytes  []byte
	project      *binder.Project
	binderErr    error
	projectErr   error
	writeErr     error
	writtenBytes []byte
	writtenPath  string
}

func (m *mockDeleteIO) ReadBinder(_ context.Context, _ string) ([]byte, error) {
	return m.binderBytes, m.binderErr
}

func (m *mockDeleteIO) ScanProject(_ context.Context, _ string) (*binder.Project, error) {
	if m.project != nil {
		return m.project, m.projectErr
	}
	return &binder.Project{Files: []string{}, BinderDir: "."}, m.projectErr
}

func (m *mockDeleteIO) WriteBinderAtomic(_ context.Context, path string, data []byte) error {
	m.writtenPath = path
	m.writtenBytes = data
	return m.writeErr
}

// delBinder returns a minimal binder with one child node for delete tests.
func delBinder() []byte {
	return []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](chapter-one.md)\n")
}

func TestNewDeleteCmd_HasRequiredFlags(t *testing.T) {
	c := NewDeleteCmd(nil)
	required := []string{"selector", "yes"}
	for _, name := range required {
		name := name
		t.Run(name, func(t *testing.T) {
			if c.Flags().Lookup(name) == nil {
				t.Errorf("expected --%s flag on delete command", name)
			}
		})
	}
}

func TestNewDeleteCmd_RejectsNon_binderMdFilename(t *testing.T) {
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
			mock := &mockDeleteIO{
				binderBytes: delBinder(),
				project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
			}
			c := NewDeleteCmd(mock)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", tt.binderPath})

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

func TestNewDeleteCmd_PrintsConfirmationOnSuccess(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Deleted chapter-one.md from _binder.md") {
		t.Errorf("expected stdout to contain confirmation, got: %s", out.String())
	}
}

func TestNewDeleteCmd_WritesModifiedBinderOnChange(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "_binder.md"})

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

func TestNewDeleteCmd_ReadBinderError(t *testing.T) {
	mock := &mockDeleteIO{binderErr: errors.New("disk error")}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ReadBinder fails")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout on ReadBinder error, got: %s", out.String())
	}
}

func TestNewDeleteCmd_ScanProjectError(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		projectErr:  errors.New("disk error"),
	}
	c := NewDeleteCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ScanProject fails")
	}
}

func TestNewDeleteCmd_WriteError(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
		writeErr:    errors.New("write failed"),
	}
	c := NewDeleteCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when WriteBinderAtomic fails")
	}
}

func TestNewDeleteCmd_ExitsNonZeroOnOpErrors(t *testing.T) {
	// Selector that matches no node → OPE001 (error-severity diagnostic)
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--selector", "nonexistent.md", "--yes", "_binder.md"})

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

func TestNewDeleteCmd_MissingYesFlagReturnsError(t *testing.T) {
	// Omitting --yes → ops.Delete returns OPE009 (missing confirmation)
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--selector", "chapter-one.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --yes flag is missing (OPE009)")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout for OPE009 error, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "OPE009") {
		t.Errorf("expected errOut to contain OPE009, got: %s", errOut.String())
	}
}

func TestNewDeleteCmd_WriteSuccessMessageError(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
	}
	c := NewDeleteCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when writing success message fails")
	}
}

func TestNewRootCmd_RegistersDeleteSubcommand(t *testing.T) {
	root := NewRootCmd()
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "delete" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected \"delete\" subcommand registered on root command")
	}
}
