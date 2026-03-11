package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestAddChildCmd_DiagnosticError_ReturnsExitError verifies that when the add
// command encounters a diagnostic error, it wraps the returned error in an
// ExitError with the exit code from ExitCodeForDiagnostics.
func TestAddChildCmd_DiagnosticError_ReturnsExitError(t *testing.T) {
	// Target _binder.md triggers CodeTargetIsBinder → ExitValidation (2).
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"_binder.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--parent", ".", "--target", "_binder.md", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error for diagnostic failure")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitValidation {
		t.Errorf("ExitError.Code = %d, want %d (ExitValidation)", exitErr.Code, ExitValidation)
	}
}

// TestDeleteCmd_DiagnosticError_ReturnsExitError verifies that when the delete
// command encounters a diagnostic error, it wraps the returned error in an
// ExitError with the exit code from ExitCodeForDiagnostics.
func TestDeleteCmd_DiagnosticError_ReturnsExitError(t *testing.T) {
	// Selector that doesn't match anything triggers CodeSelectorNoMatch → ExitNotFound (3).
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "nonexistent.md", "--yes", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error for diagnostic failure")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitNotFound {
		t.Errorf("ExitError.Code = %d, want %d (ExitNotFound)", exitErr.Code, ExitNotFound)
	}
}

// TestMoveCmd_DiagnosticError_ReturnsExitError verifies that when the move
// command encounters a diagnostic error, it wraps the returned error in an
// ExitError with the exit code from ExitCodeForDiagnostics.
func TestMoveCmd_DiagnosticError_ReturnsExitError(t *testing.T) {
	// Source selector that doesn't match anything triggers CodeSelectorNoMatch → ExitNotFound (3).
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := NewMoveCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--source", "nonexistent.md", "--dest", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error for diagnostic failure")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitNotFound {
		t.Errorf("ExitError.Code = %d, want %d (ExitNotFound)", exitErr.Code, ExitNotFound)
	}
}

// TestParseCmd_DiagnosticError_ReturnsExitError verifies that when the parse
// command encounters a binder with parse errors, it wraps the returned error
// in an ExitError with the exit code from ExitCodeForDiagnostics.
func TestParseCmd_DiagnosticError_ReturnsExitError(t *testing.T) {
	// Invalid UTF-8 triggers OPE009 error diagnostic and a parse error.
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n\xff\xfe"),
	}
	c := NewParseCmd(reader)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error for diagnostic failure")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code == 0 {
		t.Error("ExitError.Code = 0, want non-zero exit code for parse errors")
	}
}

// TestDoctorCmd_AuditDiagnosticError_ReturnsExitError verifies that when the
// doctor command encounters an audit diagnostic error, it wraps the returned
// error in an ExitError with the exit code from ExitCodeForAuditDiagnostics.
func TestDoctorCmd_AuditDiagnosticError_ReturnsExitError(t *testing.T) {
	// Missing referenced file triggers AUD001 → ExitValidation (2).
	mock := &mockDoctorIO{
		binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
		nodeFiles: map[string]nodeFileEntry{
			doctorTestNodeUUID + ".md": {content: nil, exists: false},
			".prosemark.yml":           {content: []byte("version: \"1\"\n"), exists: true},
		},
	}
	c := NewDoctorCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error for audit diagnostic failure")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitValidation {
		t.Errorf("ExitError.Code = %d, want %d (ExitValidation)", exitErr.Code, ExitValidation)
	}
}

// TestAddChildCmd_JsonMode_DiagnosticError_ReturnsExitError verifies that the
// add command in --json mode also wraps diagnostic errors in ExitError.
func TestAddChildCmd_JsonMode_DiagnosticError_ReturnsExitError(t *testing.T) {
	// Target _binder.md triggers CodeTargetIsBinder → ExitValidation (2), even in JSON mode.
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"_binder.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--parent", ".", "--target", "_binder.md", "--project", ".", "--json"})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error for diagnostic failure in JSON mode")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitValidation {
		t.Errorf("ExitError.Code = %d, want %d (ExitValidation)", exitErr.Code, ExitValidation)
	}
}

// TestAddChildCmd_NewMode_TitleNewline_ReturnsExitError verifies that in --new
// mode, a --title containing a newline character is rejected with ExitError
// code 2 (ExitValidation).
func TestAddChildCmd_NewMode_TitleNewline_ReturnsExitError(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Line1\nLine2", "--parent", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error for title containing newline in --new mode")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitValidation {
		t.Errorf("ExitError.Code = %d, want %d (ExitValidation)", exitErr.Code, ExitValidation)
	}
}

// TestAddChildCmd_NonNewMode_TitleNewline_ReturnsExitError verifies that in
// non-new mode (binder diagnostic path), a --title containing a newline
// character triggers OPE012 and is rejected with ExitError code 2
// (ExitValidation).
func TestAddChildCmd_NonNewMode_TitleNewline_ReturnsExitError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-one.md", "--title", "Line1\nLine2", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error for title containing newline in non-new mode")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitValidation {
		t.Errorf("ExitError.Code = %d, want %d (ExitValidation)", exitErr.Code, ExitValidation)
	}
}
