package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// mockAddChildIOFailSecondWrite is like mockAddChildIOWithNew but fails on the
// second call to WriteNodeFileAtomic, allowing the first (create) to succeed
// while simulating a disk error on the post-editor updated-refresh write.
type mockAddChildIOFailSecondWrite struct {
	mockAddChildIOWithNew
	writeCallCount int
	secondWriteErr error
}

func (m *mockAddChildIOFailSecondWrite) WriteNodeFileAtomic(path string, content []byte) error {
	m.writeCallCount++
	if m.writeCallCount == 1 {
		// First call: delegate to parent (succeeds).
		return m.mockAddChildIOWithNew.WriteNodeFileAtomic(path, content)
	}
	// Second call: simulate failure.
	return m.secondWriteErr
}

// TestNewAddChildCmd_NewMode_RefreshWriteError verifies that a failure on the
// second WriteNodeFileAtomic (post-editor updated-refresh) propagates as an
// error. The node and binder are already committed, so no rollback occurs.
func TestNewAddChildCmd_NewMode_RefreshWriteError(t *testing.T) {
	t.Setenv("EDITOR", "vi")

	mock := &mockAddChildIOFailSecondWrite{
		mockAddChildIOWithNew: mockAddChildIOWithNew{
			mockAddChildIO: mockAddChildIO{
				binderBytes: emptyBinder(),
				project:     &binder.Project{Files: []string{}, BinderDir: "."},
			},
		},
		secondWriteErr: errors.New("disk full on refresh"),
	}

	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when post-editor updated-refresh write fails")
	}
	// Node file was created (first write succeeded).
	if mock.nodeWrittenPath == "" {
		t.Error("expected node file to be created before refresh failure")
	}
	// No rollback: the node is already in a valid committed state.
	if mock.deletedPath != "" {
		t.Error("expected no rollback when refresh write fails")
	}
}
