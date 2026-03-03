package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestNewAddChildCmd_JSONOutput_UsesExactOpResultSchema verifies that the
// add command --json output is strict-schema binder.OpResult with no extra fields.
// This is a regression test: if a private addChildOutput struct is reintroduced,
// this test will catch any schema drift.
func TestNewAddChildCmd_JSONOutput_UsesExactOpResultSchema(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--json", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	decoder.DisallowUnknownFields()
	var result binder.OpResult
	if err := decoder.Decode(&result); err != nil {
		t.Errorf("add --json output does not match binder.OpResult schema exactly: %v\noutput: %s", err, out.String())
	}
}

// TestNewDeleteCmd_JSONOutput_UsesExactOpResultSchema verifies that the
// delete command --json output is strict-schema binder.OpResult with no extra fields.
// This is a regression test: if a private deleteOutput struct is reintroduced,
// this test will catch any schema drift.
func TestNewDeleteCmd_JSONOutput_UsesExactOpResultSchema(t *testing.T) {
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

	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	decoder.DisallowUnknownFields()
	var result binder.OpResult
	if err := decoder.Decode(&result); err != nil {
		t.Errorf("delete --json output does not match binder.OpResult schema exactly: %v\noutput: %s", err, out.String())
	}
}
