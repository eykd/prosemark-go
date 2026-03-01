package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// mockAddChildIOWithEditorBody is a mock for --new mode that tracks node file
// contents in an in-memory map. OpenEditor simulates user editing by appending
// editorBody to the currently stored file content. ReadNodeFile returns the
// post-edit in-memory content so that the refreshed write can preserve it.
//
// This mock is used to verify that the post-editor refresh reads the current
// file content (not the pre-editor content stored in the local 'content' variable).
type mockAddChildIOWithEditorBody struct {
	mockAddChildIO
	files      map[string][]byte
	editorBody string
	editorErr  error
}

func (m *mockAddChildIOWithEditorBody) WriteNodeFileAtomic(path string, content []byte) error {
	if m.files == nil {
		m.files = make(map[string][]byte)
	}
	m.files[path] = bytes.Clone(content)
	return nil
}

func (m *mockAddChildIOWithEditorBody) OpenEditor(editor, path string) error {
	if m.editorErr != nil {
		return m.editorErr
	}
	if m.files == nil {
		m.files = make(map[string][]byte)
	}
	// Simulate the user adding body content after the frontmatter.
	m.files[path] = append(m.files[path], []byte(m.editorBody)...)
	return nil
}

func (m *mockAddChildIOWithEditorBody) DeleteFile(path string) error {
	if m.files != nil {
		delete(m.files, path)
	}
	return nil
}

// ReadNodeFile returns the current in-memory content for a path.
// This method will be called by the production code after the GREEN fix
// adds ReadNodeFile to the newNodeIO interface and calls it post-editor.
func (m *mockAddChildIOWithEditorBody) ReadNodeFile(path string) ([]byte, error) {
	if m.files == nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	content, ok := m.files[path]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	return bytes.Clone(content), nil
}

// TestNewAddChildCmd_NewMode_EmptyTitleAbsentFromNodeContent verifies that when
// no --title is provided in --new mode, the resulting node content does not
// contain a "title:" field.
//
// node.SerializeFrontmatter omits optional fields (title, synopsis) when empty.
// buildNodeContent (to be deleted) always emitted "title: \n" even when the
// title was the empty string.
//
// RED: fails because buildNodeContent writes "title: " unconditionally.
func TestNewAddChildCmd_NewMode_EmptyTitleAbsentFromNodeContent(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{
		"--new", "--synopsis", "A brief synopsis.",
		"--parent", ".", "--project", ".",
	})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(mock.nodeWrittenContent)
	if strings.Contains(content, "title:") {
		t.Errorf("node content should not contain 'title:' when no title is provided;\ngot:\n%s", content)
	}
}

// TestNewAddChildCmd_NewMode_RefreshPreservesEditorBodyContent verifies that
// the post-editor timestamp refresh reads the current file content (not the
// pre-editor content stored in the local variable) so that body text added by
// the editor is preserved in the final node file.
//
// RED: fails because refreshUpdated operates on the initial 'content' variable
// (built before the editor opens), discarding any body written by the editor.
// After GREEN, runNewMode must call ReadNodeFile after the editor exits, then
// use node.ParseFrontmatter → fm.Updated → node.SerializeFrontmatter + body.
//
// NOTE: when the GREEN implementation adds ReadNodeFile to the newNodeIO
// interface, all existing newNodeIO mocks must also gain a ReadNodeFile method.
func TestNewAddChildCmd_NewMode_RefreshPreservesEditorBodyContent(t *testing.T) {
	t.Setenv("EDITOR", "vi")

	const editorBody = "\nChapter One begins here.\n"

	mock := &mockAddChildIOWithEditorBody{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
		editorBody: editorBody,
	}

	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{
		"--new", "--title", "Chapter One",
		"--edit",
		"--parent", ".", "--project", ".",
	})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The node file should have been written at least once.
	if len(mock.files) == 0 {
		t.Fatal("expected node file to be written")
	}

	// Find the node file — it is the only entry in mock.files.
	var finalContent string
	for _, content := range mock.files {
		finalContent = string(content)
	}

	// The final file content must include the body text added by the editor.
	if !strings.Contains(finalContent, strings.TrimSpace(editorBody)) {
		t.Errorf(
			"expected editor body %q in final node content;\ngot:\n%s",
			strings.TrimSpace(editorBody), finalContent,
		)
	}
}
