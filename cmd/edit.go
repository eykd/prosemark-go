package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eykd/prosemark-go/internal/binder"
)

// EditIO handles I/O for the edit command.
type EditIO interface {
	ReadBinder(path string) ([]byte, error)
	ReadNodeFile(path string) ([]byte, error)
	WriteNodeFileAtomic(path string, content []byte) error
	CreateNotesFile(path string) error
	OpenEditor(editor, path string) error
}

// editDeleter is an optional extension of EditIO that supports file deletion for rollback.
type editDeleter interface {
	DeleteFile(path string) error
}

// NewEditCmd creates the edit subcommand.
func NewEditCmd(io EditIO) *cobra.Command {
	return newEditCmdWithGetCWD(io, os.Getwd)
}

func newEditCmdWithGetCWD(io EditIO, getwd func() (string, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "edit <id>",
		Short:        "Open a node file in $EDITOR",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeID := args[0]

			editor := os.Getenv("EDITOR")
			if len(strings.Fields(editor)) == 0 {
				return fmt.Errorf("$EDITOR is not set")
			}

			project, _ := cmd.Flags().GetString("project")
			binderPath, err := resolveBinderPath(project, getwd)
			if err != nil {
				return err
			}

			part, _ := cmd.Flags().GetString("part")
			if part != "draft" && part != "notes" {
				return fmt.Errorf("--part must be \"draft\" or \"notes\", got %q", part)
			}

			binderBytes, err := io.ReadBinder(binderPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("project not initialized — run 'pmk init' first")
				}
				return fmt.Errorf("reading binder: %w", err)
			}

			parsed, _, err := binder.Parse(cmd.Context(), binderBytes, nil)
			if err != nil {
				return fmt.Errorf("cannot parse binder: %w", err)
			}

			targetFilename := nodeID + ".md"
			if !findNodeInTree(parsed.Root, targetFilename) {
				return fmt.Errorf("node %q not found in binder", nodeID)
			}

			binderDir := filepath.Dir(binderPath)
			draftPath := filepath.Join(binderDir, nodeID+".md")
			notesPath := filepath.Join(binderDir, nodeID+".notes.md")

			var editPath string
			var notesCreated bool

			if part == "notes" {
				editPath = notesPath
				if _, readErr := io.ReadNodeFile(notesPath); readErr != nil {
					if !errors.Is(readErr, os.ErrNotExist) {
						return fmt.Errorf("reading notes file: %w", readErr)
					}
					if createErr := io.CreateNotesFile(notesPath); createErr != nil {
						return fmt.Errorf("creating notes file: %w", createErr)
					}
					notesCreated = true
				}
			} else {
				editPath = draftPath
				if _, readErr := io.ReadNodeFile(draftPath); readErr != nil {
					return fmt.Errorf("reading node file: %w", readErr)
				}
			}

			if err := io.OpenEditor(editor, editPath); err != nil {
				if notesCreated {
					if deleter, ok := io.(editDeleter); ok {
						_ = deleter.DeleteFile(notesPath)
					}
				}
				return fmt.Errorf("editor: %w", err)
			}

			// Re-read the draft after the editor exits so body text is preserved,
			// then stamp the 'updated' frontmatter field and write back atomically.
			if err := refreshNodeUpdated(io, draftPath); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().String("project", "", "project directory containing _binder.md (default: current directory)")
	cmd.Flags().String("part", "draft", "which part to edit: draft or notes")

	return cmd
}

// findNodeInTree recursively searches the binder tree for a node with the given target filename.
func findNodeInTree(n *binder.Node, target string) bool {
	if n.Target == target {
		return true
	}
	for _, child := range n.Children {
		if findNodeInTree(child, target) {
			return true
		}
	}
	return false
}

// fileEditIO implements EditIO using OS file I/O.
type fileEditIO struct{}

// ReadBinder reads the binder file at path.
func (f fileEditIO) ReadBinder(path string) ([]byte, error) {
	return f.ReadBinderImpl(path)
}

// ReadBinderImpl reads the binder file using os.ReadFile.
func (f fileEditIO) ReadBinderImpl(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ReadNodeFile reads the node file at path.
func (f fileEditIO) ReadNodeFile(path string) ([]byte, error) {
	return f.ReadNodeFileImpl(path)
}

// ReadNodeFileImpl reads the node file using os.ReadFile.
func (f fileEditIO) ReadNodeFileImpl(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteNodeFileAtomic writes content to path atomically via a temp file.
func (f fileEditIO) WriteNodeFileAtomic(path string, content []byte) error {
	return f.WriteNodeFileAtomicImpl(path, content)
}

// WriteNodeFileAtomicImpl performs the atomic write via OS temp file rename.
func (f fileEditIO) WriteNodeFileAtomicImpl(path string, content []byte) error {
	return writeFileAtomicImpl(path, ".node", content)
}

// CreateNotesFile creates a new empty notes file at path using O_CREATE|O_EXCL.
func (f fileEditIO) CreateNotesFile(path string) error {
	return f.CreateNotesFileImpl(path)
}

// CreateNotesFileImpl creates the notes file using os.OpenFile with O_CREATE|O_EXCL.
func (f fileEditIO) CreateNotesFileImpl(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	return file.Close()
}

// OpenEditor opens the file at path in the named editor.
func (f fileEditIO) OpenEditor(editor, path string) error {
	return f.OpenEditorImpl(editor, path)
}

// OpenEditorImpl launches the editor process, splitting $EDITOR with strings.Fields.
func (f fileEditIO) OpenEditorImpl(editor, path string) error {
	return openEditorImpl(editor, path)
}

// openEditorImpl launches the editor process, splitting editor on whitespace.
// The first token is the executable; remaining tokens are prepended to path as args.
func openEditorImpl(editor, path string) error {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return fmt.Errorf("EDITOR is empty")
	}
	c := exec.Command(parts[0], append(parts[1:], path)...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
