package cmd

import (
	"bytes"
	"errors"
	"strings"
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

// mockAddChildIOWithReadNodeErr wraps mockAddChildIOWithNew and returns a
// configured error from ReadNodeFile, simulating a disk failure when re-reading
// the node file after the editor exits.
type mockAddChildIOWithReadNodeErr struct {
	mockAddChildIOWithNew
	readNodeErr error
}

func (m *mockAddChildIOWithReadNodeErr) ReadNodeFile(_ string) ([]byte, error) {
	return nil, m.readNodeErr
}

// mockAddChildIOWithBadNodeContent wraps mockAddChildIOWithNew and returns
// syntactically invalid content from ReadNodeFile, causing ParseFrontmatter
// to fail during the post-editor updated-refresh step.
type mockAddChildIOWithBadNodeContent struct {
	mockAddChildIOWithNew
	badContent []byte
}

func (m *mockAddChildIOWithBadNodeContent) ReadNodeFile(_ string) ([]byte, error) {
	return m.badContent, nil
}

// TestNewAddChildCmd_NewMode_ReadNodeFileError verifies that a failure reading
// the node file after the editor exits propagates as an error whose message
// contains "reading node file after edit:".
func TestNewAddChildCmd_NewMode_ReadNodeFileError(t *testing.T) {
	t.Setenv("EDITOR", "vi")

	mock := &mockAddChildIOWithReadNodeErr{
		mockAddChildIOWithNew: mockAddChildIOWithNew{
			mockAddChildIO: mockAddChildIO{
				binderBytes: emptyBinder(),
				project:     &binder.Project{Files: []string{}, BinderDir: "."},
			},
		},
		readNodeErr: errors.New("file read failed"),
	}

	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when ReadNodeFile fails after editor exits")
	}
	if !strings.Contains(err.Error(), "reading node file after edit:") {
		t.Errorf("error = %q, want to contain \"reading node file after edit:\"", err.Error())
	}
}

// TestNewAddChildCmd_NewMode_ParseFrontmatterError verifies that invalid
// frontmatter content returned by ReadNodeFile (post-editor) causes an error
// whose message contains "parsing node file after edit:".
func TestNewAddChildCmd_NewMode_ParseFrontmatterError(t *testing.T) {
	t.Setenv("EDITOR", "vi")

	mock := &mockAddChildIOWithBadNodeContent{
		mockAddChildIOWithNew: mockAddChildIOWithNew{
			mockAddChildIO: mockAddChildIO{
				binderBytes: emptyBinder(),
				project:     &binder.Project{Files: []string{}, BinderDir: "."},
			},
		},
		badContent: []byte("no frontmatter block here"),
	}

	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when ParseFrontmatter fails after editor exits")
	}
	if !strings.Contains(err.Error(), "parsing node file after edit:") {
		t.Errorf("error = %q, want to contain \"parsing node file after edit:\"", err.Error())
	}
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
