package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// mockAddChildIOWithEditorEdits simulates a user editing the node file in
// an external editor. Before OpenEditor is called, ReadNodeFile returns the
// original template written by WriteNodeFileAtomic. After OpenEditor is called,
// ReadNodeFile returns the original template plus simulated prose body text –
// mimicking a real on-disk file that the editor has modified.
//
// This mock exists to distinguish two code paths:
//   - BUGGY: use the pre-editor in-memory content variable (body lost)
//   - CORRECT: call io.ReadNodeFile after editor exits (body preserved)
//
// With the buggy code the second WriteNodeFileAtomic receives only frontmatter.
// With the correct code it receives frontmatter + the editor-added body.
type mockAddChildIOWithEditorEdits struct {
	mockAddChildIOWithNew
	editorBodyBytes []byte // prose text the "editor" adds to the file
	editorHasOpened bool
}

// OpenEditor marks that the editor ran and records the call.
func (m *mockAddChildIOWithEditorEdits) OpenEditor(editor, path string) error {
	m.editorHasOpened = true
	m.editorCalls = append(m.editorCalls, []string{editor, path})
	return m.editorErr
}

// ReadNodeFile returns the on-disk content. Before the editor runs this is the
// original template; after the editor runs this is template + body prose.
func (m *mockAddChildIOWithEditorEdits) ReadNodeFile(_ string) ([]byte, error) {
	if m.editorHasOpened && len(m.editorBodyBytes) > 0 {
		return append(bytes.Clone(m.nodeWrittenContent), m.editorBodyBytes...), nil
	}
	return m.nodeWrittenContent, nil
}

// TestNewAddChildCmd_NewMode_EditorBodyPreservedInFinalWrite is the RED test
// for the M2 data-loss bug: "pmk add --new --edit discards user edits after
// editor closes."
//
// The bug: the original code called refreshUpdated(content, ...) with the
// in-memory template variable – overwriting the node file with the empty
// template and a new timestamp, discarding everything the user typed.
//
// The fix: call io.ReadNodeFile after the editor exits, parse frontmatter,
// update only the updated field, and write back (preserving the body).
//
// This test fails with the buggy code: nodeWrittenContents[1] contains only
// frontmatter (body absent). It passes once the fix is applied: the second
// write includes the body text the mock editor "added."
func TestNewAddChildCmd_NewMode_EditorBodyPreservedInFinalWrite(t *testing.T) {
	t.Setenv("EDITOR", "vi")

	// Use a sentinel body marker that is NOT a frontmatter field value, so it
	// can only appear in the post-editor write if the code reads from disk.
	const bodyMarker = "PROSE_BODY_SENTINEL_12345"
	editorBody := []byte("\n" + bodyMarker + "\n\nThis is the opening paragraph.\n")

	mock := &mockAddChildIOWithEditorEdits{
		mockAddChildIOWithNew: mockAddChildIOWithNew{
			mockAddChildIO: mockAddChildIO{
				binderBytes: emptyBinder(),
				project:     &binder.Project{Files: []string{}, BinderDir: "."},
			},
		},
		editorBodyBytes: editorBody,
	}

	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Two WriteNodeFileAtomic calls are expected:
	//   call 1 – initial node creation (template only, no body)
	//   call 2 – post-editor refresh (frontmatter + editor-added body)
	if len(mock.nodeWrittenContents) < 2 {
		t.Fatalf("expected at least 2 WriteNodeFileAtomic calls (initial create + post-editor refresh), got %d", len(mock.nodeWrittenContents))
	}

	finalContent := string(mock.nodeWrittenContents[1])

	// The final write must contain the prose body the editor added.
	// With the buggy code (refreshUpdated(content, ...)), the body is lost.
	if !strings.Contains(finalContent, bodyMarker) {
		t.Errorf("post-editor write does not contain body text added by editor:\ngot:  %q\nwant to contain sentinel body marker", finalContent)
	}

	// The initial template write must NOT contain any prose body.
	initialContent := string(mock.nodeWrittenContents[0])
	if strings.Contains(initialContent, bodyMarker) {
		t.Errorf("initial node write unexpectedly contains body sentinel: %q", initialContent)
	}
}

// TestNewAddChildCmd_NewMode_EditorMultipleBodyLinesPreserved verifies that
// multiple lines of prose body content added by the editor all appear in the
// final node write. This ensures the fix handles arbitrary-length user input,
// not just a single appended line.
//
// With the buggy code (refreshUpdated on in-memory content), ALL prose lines
// are lost. The final write contains only frontmatter. This test fails then.
func TestNewAddChildCmd_NewMode_EditorMultipleBodyLinesPreserved(t *testing.T) {
	t.Setenv("EDITOR", "vi")

	const (
		line1 = "SENTINEL_LINE_ONE_ABCDEF"
		line2 = "SENTINEL_LINE_TWO_123456"
		line3 = "SENTINEL_LINE_THREE_GHIJK"
	)
	editorBody := []byte("\n" + line1 + "\n\n" + line2 + "\n\n" + line3 + "\n")

	mock := &mockAddChildIOWithEditorEdits{
		mockAddChildIOWithNew: mockAddChildIOWithNew{
			mockAddChildIO: mockAddChildIO{
				binderBytes: emptyBinder(),
				project:     &binder.Project{Files: []string{}, BinderDir: "."},
			},
		},
		editorBodyBytes: editorBody,
	}

	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Multi", "--edit", "--parent", ".", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.nodeWrittenContents) < 2 {
		t.Fatalf("expected at least 2 WriteNodeFileAtomic calls, got %d", len(mock.nodeWrittenContents))
	}

	finalContent := string(mock.nodeWrittenContents[1])

	for _, sentinel := range []string{line1, line2, line3} {
		if !strings.Contains(finalContent, sentinel) {
			t.Errorf("post-editor write is missing body sentinel %q:\ngot: %q", sentinel, finalContent)
		}
	}
}
