package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// validGetwd is a stub getwd that returns a plausible directory, used in
// tests that want to confirm validation fires before CWD resolution.
func validGetwd() (string, error) { return "/some/valid/dir", nil }

func TestNewParseCmd_RejectsExplicitlyEmptyProject(t *testing.T) {
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n"),
		project:     &binder.Project{Files: []string{}, BinderDir: "."},
	}
	c := newParseCmdWithGetCWD(reader, validGetwd)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--project", ""})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --project is explicitly set to empty string")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout before validation error, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "project") {
		t.Errorf("expected error message to mention --project, got: %s", errOut.String())
	}
}

func TestNewAddChildCmd_RejectsExplicitlyEmptyProject(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := newAddChildCmdWithGetCWD(mock, validGetwd)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--project", ""})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --project is explicitly set to empty string")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout before validation error, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "project") {
		t.Errorf("expected error message to mention --project, got: %s", errOut.String())
	}
}

func TestNewDeleteCmd_RejectsExplicitlyEmptyProject(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := newDeleteCmdWithGetCWD(mock, validGetwd)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", ""})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --project is explicitly set to empty string")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout before validation error, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "project") {
		t.Errorf("expected error message to mention --project, got: %s", errOut.String())
	}
}

func TestNewMoveCmd_RejectsExplicitlyEmptyProject(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	c := newMoveCmdWithGetCWD(mock, validGetwd)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--project", ""})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --project is explicitly set to empty string")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout before validation error, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "project") {
		t.Errorf("expected error message to mention --project, got: %s", errOut.String())
	}
}
