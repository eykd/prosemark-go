package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
	"github.com/eykd/prosemark-go/internal/node"
)

// mockAddChildIOWithNodeRead extends mockAddChildIO with configurable
// ReadNodeFile behavior for testing frontmatter title derivation.
type mockAddChildIOWithNodeRead struct {
	mockAddChildIO
	nodeFileContent []byte
	nodeFileErr     error
	readNodePath    string // records the path passed to ReadNodeFile
}

func (m *mockAddChildIOWithNodeRead) ReadNodeFile(path string) ([]byte, error) {
	m.readNodePath = path
	return m.nodeFileContent, m.nodeFileErr
}

// nodeFileWithTitle returns a minimal node file with the given frontmatter title.
func nodeFileWithTitle(id, title string) []byte {
	fm := node.Frontmatter{
		ID:      id,
		Title:   title,
		Created: "2026-01-01T00:00:00Z",
		Updated: "2026-01-01T00:00:00Z",
	}
	return node.SerializeFrontmatter(fm)
}

// nodeFileWithoutTitle returns a minimal node file without a title field.
func nodeFileWithoutTitle(id string) []byte {
	fm := node.Frontmatter{
		ID:      id,
		Created: "2026-01-01T00:00:00Z",
		Updated: "2026-01-01T00:00:00Z",
	}
	return node.SerializeFrontmatter(fm)
}

func TestAddChild_FrontmatterTitleDerivation(t *testing.T) {
	const uuid = "019cefe8-4d53-7d0f-bbfa-cb40bee67ffc"
	target := uuid + ".md"

	tests := []struct {
		name            string
		nodeFileContent []byte
		nodeFileErr     error
		titleFlag       string
		wantLinkTitle   string
	}{
		{
			name:            "target file has frontmatter title",
			nodeFileContent: nodeFileWithTitle(uuid, "The Raven"),
			wantLinkTitle:   "The Raven",
		},
		{
			name:            "target file has no frontmatter title falls back to stem",
			nodeFileContent: nodeFileWithoutTitle(uuid),
			wantLinkTitle:   uuid,
		},
		{
			name:          "target file does not exist falls back to stem",
			nodeFileErr:   &notExistError{},
			wantLinkTitle: uuid,
		},
		{
			name:            "target file has invalid frontmatter falls back to stem",
			nodeFileContent: []byte("not valid frontmatter at all"),
			wantLinkTitle:   uuid,
		},
		{
			name:            "explicit title flag overrides frontmatter title",
			nodeFileContent: nodeFileWithTitle(uuid, "The Raven"),
			titleFlag:       "Custom Title",
			wantLinkTitle:   "Custom Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAddChildIOWithNodeRead{
				mockAddChildIO: mockAddChildIO{
					binderBytes: emptyBinder(),
					project:     &binder.Project{Files: []string{target}, BinderDir: "."},
				},
				nodeFileContent: tt.nodeFileContent,
				nodeFileErr:     tt.nodeFileErr,
			}
			cmd := NewAddChildCmd(mock)
			out := new(bytes.Buffer)
			cmd.SetOut(out)
			cmd.SetErr(new(bytes.Buffer))

			args := []string{"--parent", ".", "--target", target}
			if tt.titleFlag != "" {
				args = append(args, "--title", tt.titleFlag)
			}
			cmd.SetArgs(args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// The binder should contain a link with the expected title.
			wantLink := "[" + tt.wantLinkTitle + "](" + uuid + ".md)"
			if !strings.Contains(string(mock.writtenBytes), wantLink) {
				t.Errorf("binder output does not contain expected link %q\ngot:\n%s",
					wantLink, string(mock.writtenBytes))
			}
		})
	}
}

// notExistError satisfies the os.IsNotExist check.
type notExistError struct{}

func (e *notExistError) Error() string { return "file does not exist" }

func TestAddChild_FrontmatterTitleReadsCorrectPath(t *testing.T) {
	const uuid = "019cefe8-4d53-7d0f-bbfa-cb40bee67ffc"
	target := uuid + ".md"

	mock := &mockAddChildIOWithNodeRead{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{target}, BinderDir: "."},
		},
		nodeFileContent: nodeFileWithTitle(uuid, "Chapter One"),
	}
	cmd := newAddChildCmdWithGetCWD(mock, func() (string, error) {
		return "/project/dir", nil
	})
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--parent", ".", "--target", target})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the correct path was used to read the node file.
	// The path should be relative to the binder directory.
	if mock.readNodePath == "" {
		t.Fatal("ReadNodeFile was not called")
	}
	if !strings.HasSuffix(mock.readNodePath, target) {
		t.Errorf("ReadNodeFile called with %q, want path ending in %q", mock.readNodePath, target)
	}
}

func TestAddChild_SubdirTargetFrontmatterTitle(t *testing.T) {
	const uuid = "019cefe8-4d53-7d0f-bbfa-cb40bee67ffc"
	target := "chapters/" + uuid + ".md"

	mock := &mockAddChildIOWithNodeRead{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{target}, BinderDir: "."},
		},
		nodeFileContent: nodeFileWithTitle(uuid, "Nested Chapter"),
	}
	cmd := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--parent", ".", "--target", target})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantLink := "[Nested Chapter](" + target + ")"
	if !strings.Contains(string(mock.writtenBytes), wantLink) {
		t.Errorf("binder output does not contain expected link %q\ngot:\n%s",
			wantLink, string(mock.writtenBytes))
	}
}
