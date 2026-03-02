package cmd

// Tests for injectable clock in --new mode (nowUTCFunc).
//
// RED: These tests fail to compile because nowUTCFunc does not yet exist in
// cmd/addchild.go.  Currently runNewMode calls node.NowUTC() directly, making
// it impossible to assert exact timestamp values.
//
// GREEN: Add `var nowUTCFunc = node.NowUTC` to addchild.go and replace the
// two direct node.NowUTC() calls in runNewMode with nowUTCFunc().
//
// The tests below verify:
//  1. The initial node write uses nowUTCFunc() for both created and updated.
//  2. The post-editor refresh uses a second nowUTCFunc() call for updated only.
//  3. The created timestamp is NOT overwritten by the post-editor refresh.
//
// This mirrors how nodeIDGenerator is injected: a package-level var pointing
// to the real implementation, overridable in tests via defer-restore.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestRunNewMode_CreatedTimestampFromNowUTCFunc verifies that the created and
// updated fields in the initial node write are set using nowUTCFunc().
// Fails to compile until nowUTCFunc is declared in cmd/addchild.go.
func TestRunNewMode_CreatedTimestampFromNowUTCFunc(t *testing.T) {
	orig := nowUTCFunc // RED: nowUTCFunc does not exist yet
	defer func() { nowUTCFunc = orig }()

	const fixedTS = "2024-01-01T10:00:00Z"
	nowUTCFunc = func() string { return fixedTS }

	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--parent", ".", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(mock.nodeWrittenContent)
	if !strings.Contains(content, "created: "+fixedTS) {
		t.Errorf("initial write: created should be %q\ngot:\n%s", fixedTS, content)
	}
	if !strings.Contains(content, "updated: "+fixedTS) {
		t.Errorf("initial write: updated should be %q\ngot:\n%s", fixedTS, content)
	}
}

// TestRunNewMode_UpdatedRefreshedByNowUTCFuncAfterEdit verifies that the
// post-editor refresh writes the timestamp from the second nowUTCFunc() call
// into the updated field, while leaving all other fields unchanged.
// Fails to compile until nowUTCFunc is declared in cmd/addchild.go.
func TestRunNewMode_UpdatedRefreshedByNowUTCFuncAfterEdit(t *testing.T) {
	t.Setenv("EDITOR", "vi")

	orig := nowUTCFunc // RED: nowUTCFunc does not exist yet
	defer func() { nowUTCFunc = orig }()

	callCount := 0
	timestamps := []string{
		"2024-01-01T10:00:00Z", // first call: node creation (created + updated)
		"2024-01-01T10:05:00Z", // second call: post-editor refresh (updated only)
	}
	nowUTCFunc = func() string {
		ts := timestamps[callCount]
		callCount++
		return ts
	}

	mock := &mockAddChildIOWithEditorEdits{
		mockAddChildIOWithNew: mockAddChildIOWithNew{
			mockAddChildIO: mockAddChildIO{
				binderBytes: emptyBinder(),
				project:     &binder.Project{Files: []string{}, BinderDir: "."},
			},
		},
		editorBodyBytes: []byte("\nSome prose content.\n"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.nodeWrittenContents) < 2 {
		t.Fatalf("expected at least 2 WriteNodeFileAtomic calls, got %d", len(mock.nodeWrittenContents))
	}

	// Initial write: both created and updated set from timestamps[0].
	initialContent := string(mock.nodeWrittenContents[0])
	if !strings.Contains(initialContent, "created: "+timestamps[0]) {
		t.Errorf("initial write: created should be %q\ngot:\n%s", timestamps[0], initialContent)
	}
	if !strings.Contains(initialContent, "updated: "+timestamps[0]) {
		t.Errorf("initial write: updated should be %q\ngot:\n%s", timestamps[0], initialContent)
	}

	// Post-editor write: created preserved at timestamps[0], updated refreshed to timestamps[1].
	finalContent := string(mock.nodeWrittenContents[1])
	if !strings.Contains(finalContent, "created: "+timestamps[0]) {
		t.Errorf("post-editor write: created should still be %q\ngot:\n%s", timestamps[0], finalContent)
	}
	if !strings.Contains(finalContent, "updated: "+timestamps[1]) {
		t.Errorf("post-editor write: updated should be refreshed to %q\ngot:\n%s", timestamps[1], finalContent)
	}
}

// TestRunNewMode_CreatedPreservedAfterEditorExit verifies that the created
// timestamp set at node-creation time is NOT overwritten by the post-editor
// updated refresh.  Only the updated field should change.
// Fails to compile until nowUTCFunc is declared in cmd/addchild.go.
func TestRunNewMode_CreatedPreservedAfterEditorExit(t *testing.T) {
	t.Setenv("EDITOR", "vi")

	orig := nowUTCFunc // RED: nowUTCFunc does not exist yet
	defer func() { nowUTCFunc = orig }()

	callCount := 0
	nowUTCFunc = func() string {
		callCount++
		if callCount == 1 {
			return "2024-01-01T10:00:00Z"
		}
		return "2024-01-01T10:05:00Z"
	}

	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Preserved", "--edit", "--parent", ".", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.nodeWrittenContents) < 2 {
		t.Fatalf("expected at least 2 WriteNodeFileAtomic calls, got %d", len(mock.nodeWrittenContents))
	}

	finalContent := string(mock.nodeWrittenContents[1])

	// created must still reflect the node-creation timestamp, not the refresh timestamp.
	if strings.Contains(finalContent, "created: 2024-01-01T10:05:00Z") {
		t.Errorf("created timestamp was incorrectly overwritten by post-editor refresh:\ngot:\n%s", finalContent)
	}
	if !strings.Contains(finalContent, "created: 2024-01-01T10:00:00Z") {
		t.Errorf("created timestamp missing from post-editor write:\ngot:\n%s", finalContent)
	}
}
