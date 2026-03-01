package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/eykd/prosemark-go/internal/binder"
	"github.com/eykd/prosemark-go/internal/binder/ops"
)

// AddChildIO handles I/O for the add command.
type AddChildIO interface {
	ReadBinder(ctx context.Context, path string) ([]byte, error)
	ScanProject(ctx context.Context, binderPath string) (*binder.Project, error)
	WriteBinderAtomic(ctx context.Context, path string, data []byte) error
}

// newNodeIO is an optional extension of AddChildIO for --new mode.
// Commands implementing this interface support atomic node file creation.
type newNodeIO interface {
	WriteNodeFileAtomic(path string, content []byte) error
	DeleteFile(path string) error
	OpenEditor(editor, path string) error
}

// uuidFilenameRe matches a valid lowercase UUIDv7 filename (UUID.md).
var uuidFilenameRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.md$`)

// nodeIDGenerator generates a new UUIDv7-based node filename.
// Override in tests to inject specific values or simulate errors.
var nodeIDGenerator = nodeIDv7Impl

// nodeIDv7Impl calls uuid.NewV7 to produce a UUIDv7-based node filename.
// Excluded from coverage because it wraps an external entropy source.
func nodeIDv7Impl() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String() + ".md", nil
}

// hasControlChars reports whether s contains any C0 control character
// (U+0000–U+001F) or DEL (U+007F).
func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7F {
			return true
		}
	}
	return false
}

// buildNodeContent returns YAML frontmatter content for a new node file.
func buildNodeContent(uuidStem, title, synopsis, timestamp string) []byte {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("id: " + uuidStem + "\n")
	sb.WriteString("title: " + title + "\n")
	sb.WriteString("synopsis: " + synopsis + "\n")
	sb.WriteString("created: " + timestamp + "\n")
	sb.WriteString("updated: " + timestamp + "\n")
	sb.WriteString("---\n")
	return []byte(sb.String())
}

// NewAddChildCmd creates the add subcommand.
func NewAddChildCmd(io AddChildIO) *cobra.Command {
	return newAddChildCmdWithGetCWD(io, os.Getwd)
}

func newAddChildCmdWithGetCWD(io AddChildIO, getwd func() (string, error)) *cobra.Command {
	var (
		parent   string
		target   string
		title    string
		first    bool
		at       int
		before   string
		after    string
		force    bool
		jsonMode bool
		newMode  bool
		synopsis string
		editMode bool
	)

	cmd := &cobra.Command{
		Use:          "add",
		Short:        "Add a child node to a binder",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			binderPath, err := resolveBinderPath(project, getwd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			binderBytes, err := io.ReadBinder(ctx, binderPath)
			if err != nil {
				return fmt.Errorf("reading binder: %w", err)
			}

			proj, err := io.ScanProject(ctx, binderPath)
			if err != nil {
				return emitOPE009AndError(cmd, jsonMode, err)
			}

			positionFlagsSet := 0
			if first {
				positionFlagsSet++
			}
			if cmd.Flags().Changed("at") {
				positionFlagsSet++
			}
			if before != "" {
				positionFlagsSet++
			}
			if after != "" {
				positionFlagsSet++
			}
			if positionFlagsSet > 1 {
				return fmt.Errorf("only one of --first, --at, --before, --after may be specified (%s)", binder.CodeConflictingFlags)
			}

			position := "last"
			if first {
				position = "first"
			}

			if newMode {
				// Validate target (must be UUID filename format; no path separators).
				if target != "" {
					if strings.ContainsRune(target, os.PathSeparator) {
						return fmt.Errorf("target must not contain path separators")
					}
					if !uuidFilenameRe.MatchString(target) {
						return fmt.Errorf("target must be a valid UUID filename when --new is set")
					}
				}

				// Validate title (≤500 chars, no control characters).
				if len(title) > 500 {
					return fmt.Errorf("--title must be 500 characters or fewer")
				}
				if hasControlChars(title) {
					return fmt.Errorf("--title must not contain control characters")
				}

				// Validate synopsis (≤2000 chars, no control characters).
				if len(synopsis) > 2000 {
					return fmt.Errorf("--synopsis must be 2000 characters or fewer")
				}
				if hasControlChars(synopsis) {
					return fmt.Errorf("--synopsis must not contain control characters")
				}

				// Generate UUID filename if not explicitly provided.
				if target == "" {
					id, genErr := nodeIDGenerator()
					if genErr != nil {
						return fmt.Errorf("generating node ID: %w", genErr)
					}
					target = id
				}

				// Require IO that supports node file creation.
				nnIO, ok := io.(newNodeIO)
				if !ok {
					return fmt.Errorf("IO does not support --new mode")
				}

				uuidStem := strings.TrimSuffix(target, ".md")
				binderDir := filepath.Dir(binderPath)
				nodePath := filepath.Join(binderDir, target)
				timestamp := time.Now().UTC().Format(time.RFC3339)
				content := buildNodeContent(uuidStem, title, synopsis, timestamp)

				if err := nnIO.WriteNodeFileAtomic(nodePath, content); err != nil {
					return fmt.Errorf("creating node file: %w", err)
				}

				params := binder.AddChildParams{
					ParentSelector: parent,
					Target:         target,
					Title:          title,
					Position:       position,
					Before:         before,
					After:          after,
					Force:          force,
				}
				if cmd.Flags().Changed("at") {
					params.At = &at
				}

				modifiedBytes, diags, _ := ops.AddChild(ctx, binderBytes, proj, params) //nolint:errcheck
				if diags == nil {
					diags = []binder.Diagnostic{}
				}

				printDiagnostics(cmd, diags)

				changed := !bytes.Equal(binderBytes, modifiedBytes)
				if changed {
					if writeErr := io.WriteBinderAtomic(ctx, binderPath, modifiedBytes); writeErr != nil {
						if rollbackErr := nnIO.DeleteFile(nodePath); rollbackErr != nil {
							return fmt.Errorf("writing binder: %w; rollback also failed: %v", writeErr, rollbackErr)
						}
						return fmt.Errorf("writing binder: %w", writeErr)
					}
				}

				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Created "+sanitizePath(target)+" in "+sanitizePath(binderPath)); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}

				if editMode {
					editor := os.Getenv("EDITOR")
					if editor == "" {
						return fmt.Errorf("$EDITOR is not set")
					}
					if err := nnIO.OpenEditor(editor, nodePath); err != nil {
						return fmt.Errorf("opening editor: %w", err)
					}
				}

				return nil
			}

			params := binder.AddChildParams{
				ParentSelector: parent,
				Target:         target,
				Title:          title,
				Position:       position,
				Before:         before,
				After:          after,
				Force:          force,
			}
			if cmd.Flags().Changed("at") {
				params.At = &at
			}

			modifiedBytes, diags, _ := ops.AddChild(ctx, binderBytes, proj, params) //nolint:errcheck
			if diags == nil {
				diags = []binder.Diagnostic{}
			}

			changed := !bytes.Equal(binderBytes, modifiedBytes)

			hasError := false
			for _, d := range diags {
				if d.Severity == "error" {
					hasError = true
					break
				}
			}

			if jsonMode {
				out := binder.OpResult{Version: "1", Changed: changed, Diagnostics: diags}
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(out); err != nil {
					return fmt.Errorf("encoding output: %w", err)
				}
			} else {
				printDiagnostics(cmd, diags)
			}

			if hasError {
				return fmt.Errorf("add has errors")
			}

			if changed {
				if err = io.WriteBinderAtomic(ctx, binderPath, modifiedBytes); err != nil {
					return fmt.Errorf("writing binder: %w", err)
				}
			}

			if !jsonMode {
				if changed {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Added "+sanitizePath(target)+" to "+sanitizePath(binderPath)); err != nil {
						return fmt.Errorf("writing output: %w", err)
					}
				} else {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), sanitizePath(target)+" already in "+sanitizePath(binderPath)+" (skipped)"); err != nil {
						return fmt.Errorf("writing output: %w", err)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().String("project", "", "project directory containing _binder.md (default: current directory)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent selector")
	cmd.Flags().StringVar(&target, "target", "", "Target path for new child")
	cmd.Flags().StringVar(&title, "title", "", "Display title (empty = derive from stem)")
	cmd.Flags().BoolVar(&first, "first", false, "Insert as first child")
	cmd.Flags().IntVar(&at, "at", 0, "Zero-based insertion index")
	cmd.Flags().StringVar(&before, "before", "", "Insert before selector")
	cmd.Flags().StringVar(&after, "after", "", "Insert after selector")
	cmd.Flags().BoolVar(&force, "force", false, "Allow duplicate target")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Output result as JSON")
	cmd.Flags().BoolVar(&newMode, "new", false, "Create a new UUID node file")
	cmd.Flags().StringVar(&synopsis, "synopsis", "", "Set the synopsis frontmatter field (≤2000 chars)")
	cmd.Flags().BoolVar(&editMode, "edit", false, "Open node file in $EDITOR after creation")

	return cmd
}

// fileAddChildIO implements AddChildIO and newNodeIO using OS file I/O.
type fileAddChildIO struct{}

func newDefaultAddChildIO() *fileAddChildIO {
	return &fileAddChildIO{}
}

// ReadBinder reads the binder file at path.
func (w *fileAddChildIO) ReadBinder(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ScanProject scans the project directory for .md files.
func (w *fileAddChildIO) ScanProject(ctx context.Context, binderPath string) (*binder.Project, error) {
	return ScanProjectImpl(ctx, binderPath)
}

// WriteBinderAtomic writes data to path atomically via a temp file.
func (w *fileAddChildIO) WriteBinderAtomic(ctx context.Context, path string, data []byte) error {
	return w.WriteBinderAtomicImpl(ctx, path, data)
}

// WriteBinderAtomicImpl performs the atomic write via OS temp file rename.
func (w *fileAddChildIO) WriteBinderAtomicImpl(_ context.Context, path string, data []byte) error {
	if fi, statErr := os.Stat(path); statErr == nil {
		if fi.Mode().Perm()&0200 == 0 {
			return fmt.Errorf("binder file is read-only")
		}
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".binder-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// WriteNodeFileAtomic writes content to path atomically (for --new mode).
func (w *fileAddChildIO) WriteNodeFileAtomic(path string, content []byte) error {
	return w.WriteNodeFileAtomicImpl(path, content)
}

// WriteNodeFileAtomicImpl performs the atomic write of a new node file.
func (w *fileAddChildIO) WriteNodeFileAtomicImpl(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".node-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// DeleteFile removes the file at path (used for rollback in --new mode).
func (w *fileAddChildIO) DeleteFile(path string) error {
	return w.DeleteFileImpl(path)
}

// DeleteFileImpl removes the file at path using os.Remove.
func (w *fileAddChildIO) DeleteFileImpl(path string) error {
	return os.Remove(path)
}

// OpenEditor opens the file at path in the named editor.
func (w *fileAddChildIO) OpenEditor(editor, path string) error {
	return w.OpenEditorImpl(editor, path)
}

// OpenEditorImpl launches the editor process and waits for it to exit.
func (w *fileAddChildIO) OpenEditorImpl(editor, path string) error {
	c := exec.Command(editor, path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
