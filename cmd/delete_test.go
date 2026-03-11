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

// mockDeleteIO is a test double for DeleteIO.
type mockDeleteIO struct {
	binderBytes  []byte
	project      *binder.Project
	binderErr    error
	projectErr   error
	writeErr     error
	writtenBytes []byte
	writtenPath  string

	// File removal tracking for --rm flag.
	removeFileErr   error
	removedFiles    []string
	removeFileCalls int
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

func (m *mockDeleteIO) RemoveFile(_ context.Context, path string) error {
	m.removeFileCalls++
	m.removedFiles = append(m.removedFiles, path)
	return m.removeFileErr
}

// delBinder returns a minimal binder with one child node for delete tests.
func delBinder() []byte {
	return []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](chapter-one.md)\n")
}

func TestNewDeleteCmd_HasRequiredFlags(t *testing.T) {
	c := NewDeleteCmd(nil)
	required := []string{"project", "selector", "yes", "json"}
	for _, name := range required {
		name := name
		t.Run(name, func(t *testing.T) {
			if c.Flags().Lookup(name) == nil {
				t.Errorf("expected --%s flag on delete command", name)
			}
		})
	}
}

func TestNewDeleteCmd_DefaultsToCWD(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes"})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected success with no --project (CWD default): %v", err)
	}
}

func TestNewDeleteCmd_AcceptsProjectFlag(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "/some/dir"})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected success with --project flag: %v", err)
	}
}

func TestNewDeleteCmd_GetCWDError(t *testing.T) {
	mock := &mockDeleteIO{binderBytes: delBinder()}
	c := newDeleteCmdWithGetCWD(mock, func() (string, error) {
		return "", errors.New("getwd failed")
	})
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when getwd fails")
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
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

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
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

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
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

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
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

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
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

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
	c.SetArgs([]string{"--selector", "nonexistent.md", "--yes", "--project", "."})

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
	c.SetArgs([]string{"--selector", "chapter-one.md", "--project", "."})

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

// TestNewDeleteCmd_MissingYes_ExitCode1_AndSuggestion verifies that
// missing --yes returns exit code 1 (usage) with a suggestion mentioning --yes,
// not exit code 6 (transient) with a misleading _binder.md suggestion.
func TestNewDeleteCmd_MissingYes_ExitCode1_AndSuggestion(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--project", "."})

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

// TestNewDeleteCmd_MissingYes_JSON_EmitsOPE011 verifies that the JSON output
// for missing --yes contains OPE011 (not OPE009) with a --yes suggestion.
func TestNewDeleteCmd_MissingYes_JSON_EmitsOPE011(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one.md", "--json", "--project", "."})

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

func TestNewDeleteCmd_WriteSuccessMessageError(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
	}
	c := NewDeleteCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when writing success message fails")
	}
}

func TestNewDeleteCmd_OutputsOpResultJSONOnSuccess(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--json", "--project", "."})

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
	if !result.Changed {
		t.Error("expected Changed=true when node is deleted")
	}
}

func TestNewDeleteCmd_JSONEncodeError(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--json", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when JSON encoding fails")
	}
}

func TestNewDeleteCmd_ScanProjectErrorWithJSON(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		projectErr:  errors.New("disk error"),
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--json", "--project", "."})

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

// ──────────────────────────────────────────────────────────────────────────────
// --rm flag: delete node file from disk
// ──────────────────────────────────────────────────────────────────────────────

func TestNewDeleteCmd_HasRmFlag(t *testing.T) {
	c := NewDeleteCmd(nil)
	if c.Flags().Lookup("rm") == nil {
		t.Error("expected --rm flag on delete command")
	}
}

func TestNewDeleteCmd_RmRemovesNodeFileOnSuccess(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "/proj"},
	}
	c := NewDeleteCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one", "--yes", "--rm", "--project", "/proj"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.removeFileCalls != 1 {
		t.Errorf("RemoveFile called %d times, want 1", mock.removeFileCalls)
	}
	if len(mock.removedFiles) != 1 || mock.removedFiles[0] != "/proj/chapter-one.md" {
		t.Errorf("removed files = %v, want [\"/proj/chapter-one.md\"]", mock.removedFiles)
	}
}

func TestNewDeleteCmd_NoRmDoesNotRemoveFile(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "/proj"},
	}
	c := NewDeleteCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one", "--yes", "--project", "/proj"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.removeFileCalls != 0 {
		t.Errorf("RemoveFile should not be called without --rm, called %d times", mock.removeFileCalls)
	}
}

func TestNewDeleteCmd_RmWithDryRunDoesNotRemoveFile(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	sub := NewDeleteCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"delete", "--selector", "chapter-one", "--yes", "--rm", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.removeFileCalls != 0 {
		t.Errorf("RemoveFile should not be called with --dry-run, called %d times", mock.removeFileCalls)
	}
}

func TestNewDeleteCmd_RmRemoveFileError(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes:   delBinder(),
		project:       &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "/proj"},
		removeFileErr: errors.New("permission denied"),
	}
	c := NewDeleteCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one", "--yes", "--rm", "--project", "/proj"})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when RemoveFile fails")
	}
	if err != nil && !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error = %v, want to contain 'permission denied'", err)
	}
}

func TestNewDeleteCmd_RmConfirmationMessageIncludesFileRemoval(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "/proj"},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one", "--yes", "--rm", "--project", "/proj"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "removed chapter-one.md") {
		t.Errorf("confirmation should mention file removal, got: %s", out.String())
	}
}

func TestNewDeleteCmd_RmCascadeRemovesAllSubtreeFiles(t *testing.T) {
	// Binder with parent + child nodes.
	src := []byte("<!-- prosemark-binder:v1 -->\n- [Act One](act-one.md)\n  - [Scene One](scene-one.md)\n")
	mock := &mockDeleteIO{
		binderBytes: src,
		project:     &binder.Project{Files: []string{"act-one.md", "scene-one.md"}, BinderDir: "/proj"},
	}
	c := NewDeleteCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "act-one", "--yes", "--rm", "--project", "/proj"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.removeFileCalls != 2 {
		t.Errorf("RemoveFile called %d times, want 2 (parent + child)", mock.removeFileCalls)
	}
}
