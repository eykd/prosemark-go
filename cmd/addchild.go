package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/eykd/prosemark-go/internal/binder"
	"github.com/eykd/prosemark-go/internal/binder/ops"
	"github.com/eykd/prosemark-go/internal/node"
)

// AddChildIO handles I/O for the add command.
type AddChildIO interface {
	ReadBinder(ctx context.Context, path string) ([]byte, error)
	ScanProject(ctx context.Context, binderPath string) (*binder.Project, error)
	WriteBinderAtomic(ctx context.Context, path string, data []byte) error
}

// newNodeIO defines the I/O capabilities required for --new mode node file management.
type newNodeIO interface {
	WriteNodeFileAtomic(path string, content []byte) error
	DeleteFile(path string) error
	OpenEditor(editor, path string) error
	ReadNodeFile(path string) ([]byte, error)
}

// NewNodeAddChildIO is the combined IO interface for --new mode.
// Embedding both AddChildIO and newNodeIO expresses the --new capability
// in the type system rather than discovering it at runtime via type assertion.
type NewNodeAddChildIO interface {
	AddChildIO
	newNodeIO
}

// nodeIDGenerator generates a new UUIDv7-based node filename.
// Override in tests to inject specific values or simulate errors.
var nodeIDGenerator = nodeIDv7Impl

// nowUTCFunc returns the current UTC time as a string for frontmatter timestamps.
// Override in tests to inject specific values or simulate clock behavior.
var nowUTCFunc = node.NowUTC

// nodeIDv7Impl calls uuid.NewV7 to produce a UUIDv7-based node filename.
// Excluded from coverage because it wraps an external entropy source.
func nodeIDv7Impl() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String() + ".md", nil
}

// NewAddChildCmd creates the add subcommand.
func NewAddChildCmd(io NewNodeAddChildIO) *cobra.Command {
	return newAddChildCmdWithGetCWD(io, os.Getwd)
}

func newAddChildCmdWithGetCWD(io NewNodeAddChildIO, getwd func() (string, error)) *cobra.Command {
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
			binderPath, err := resolveBinderPathFromCmd(cmd, getwd)
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

			if newMode {
				if err := node.ValidateNewNodeInput(target, title, synopsis); err != nil {
					return err
				}
				if target == "" {
					id, genErr := nodeIDGenerator()
					if genErr != nil {
						return fmt.Errorf("generating node ID: %w", genErr)
					}
					params.Target = id
				}
				return runNewMode(ctx, cmd, io, binderPath, binderBytes, proj, params, synopsis, editMode)
			}

			modifiedBytes, diags, _ := ops.AddChild(ctx, binderBytes, proj, params) //nolint:errcheck
			if diags == nil {
				diags = []binder.Diagnostic{}
			}

			changed := !bytes.Equal(binderBytes, modifiedBytes)

			if jsonMode {
				out := binder.OpResult{Version: "1", Changed: changed, Diagnostics: diags}
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(out); err != nil {
					return fmt.Errorf("encoding output: %w", err)
				}
			} else {
				printDiagnostics(cmd, diags)
			}

			if hasDiagnosticError(diags) {
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

// nodeRefresher is the minimal IO surface needed to refresh a node file's
// 'updated' frontmatter timestamp after an editor session.
type nodeRefresher interface {
	ReadNodeFile(path string) ([]byte, error)
	WriteNodeFileAtomic(path string, content []byte) error
}

// refreshNodeUpdated reads the node file at path, stamps the 'updated'
// frontmatter field with the current UTC time, and writes it back atomically.
func refreshNodeUpdated(io nodeRefresher, path string) error {
	content, err := io.ReadNodeFile(path)
	if err != nil {
		return fmt.Errorf("reading node file after edit: %w", err)
	}
	fm, body, err := node.ParseFrontmatter(content)
	if err != nil {
		return fmt.Errorf("parsing node file after edit: %w", err)
	}
	fm.Updated = nowUTCFunc()
	refreshed := append(node.SerializeFrontmatter(fm), body...)
	if err := io.WriteNodeFileAtomic(path, refreshed); err != nil {
		return fmt.Errorf("refreshing node file after edit: %w", err)
	}
	return nil
}

// runNewMode handles the --new flag workflow: creates a UUID node file, updates
// the binder, and optionally opens an editor to populate the file.
// params.Target must already be set to a valid UUID filename before calling.
func runNewMode(ctx context.Context, cmd *cobra.Command, io NewNodeAddChildIO, binderPath string, binderBytes []byte, proj *binder.Project, params binder.AddChildParams, synopsis string, editMode bool) error {
	uuidStem := strings.TrimSuffix(params.Target, ".md")
	binderDir := filepath.Dir(binderPath)
	nodePath := filepath.Join(binderDir, params.Target)
	now := nowUTCFunc()
	fm := node.Frontmatter{
		ID:       uuidStem,
		Title:    params.Title,
		Synopsis: synopsis,
		Created:  now,
		Updated:  now,
	}
	content := node.SerializeFrontmatter(fm)

	if err := io.WriteNodeFileAtomic(nodePath, content); err != nil {
		return fmt.Errorf("creating node file: %w", err)
	}

	modifiedBytes, diags, _ := ops.AddChild(ctx, binderBytes, proj, params) //nolint:errcheck
	if diags == nil {
		diags = []binder.Diagnostic{}
	}

	printDiagnostics(cmd, diags)

	if hasDiagnosticError(diags) {
		rollbackErr := io.DeleteFile(nodePath)
		return errors.Join(fmt.Errorf("add has errors"), rollbackErr)
	}

	changed := !bytes.Equal(binderBytes, modifiedBytes)
	if changed {
		if writeErr := io.WriteBinderAtomic(ctx, binderPath, modifiedBytes); writeErr != nil {
			if rollbackErr := io.DeleteFile(nodePath); rollbackErr != nil {
				return fmt.Errorf("writing binder: %w; rollback also failed: %v", writeErr, rollbackErr)
			}
			return fmt.Errorf("writing binder: %w", writeErr)
		}
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Created "+sanitizePath(params.Target)+" in "+sanitizePath(binderPath)); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	if editMode {
		editor := os.Getenv("EDITOR")
		if strings.TrimSpace(editor) == "" {
			return fmt.Errorf("$EDITOR is not set")
		}
		if err := io.OpenEditor(editor, nodePath); err != nil {
			return fmt.Errorf("opening editor: %w", err)
		}
		// Re-read the file after the editor exits so body text is preserved,
		// then stamp the 'updated' frontmatter field and write back atomically.
		if err := refreshNodeUpdated(io, nodePath); err != nil {
			return err
		}
	}

	return nil
}

// fileAddChildIO implements NewNodeAddChildIO using OS file I/O.
type fileAddChildIO struct{}

func newDefaultAddChildIO() *fileAddChildIO {
	return &fileAddChildIO{}
}

// ReadBinder reads the binder file at path.
func (w *fileAddChildIO) ReadBinder(_ context.Context, path string) ([]byte, error) {
	return readBinderSizeLimitedImpl(path)
}

// ScanProject scans the project directory for .md files.
func (w *fileAddChildIO) ScanProject(ctx context.Context, binderPath string) (*binder.Project, error) {
	return ScanProjectImpl(ctx, binderPath)
}

// WriteBinderAtomic writes data to path atomically via a temp file.
func (w *fileAddChildIO) WriteBinderAtomic(ctx context.Context, path string, data []byte) error {
	return w.WriteBinderAtomicImpl(ctx, path, data)
}

// maxBinderFileSize is the maximum allowed size for _binder.md files (10 MB).
const maxBinderFileSize = 10 * 1024 * 1024

// readBinderSizeLimitedImpl reads the binder file at path, rejecting files that
// exceed maxBinderFileSize. Excluded from coverage because it wraps OS calls.
func readBinderSizeLimitedImpl(path string) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.Size() > maxBinderFileSize {
		return nil, fmt.Errorf("binder file exceeds the 10 MB size limit")
	}
	return os.ReadFile(path)
}

// writeFileAtomicImpl writes data to path atomically via a temp file and rename.
// tmpPrefix is the leading label used for the temp file name (e.g. ".binder" or ".node").
func writeFileAtomicImpl(path, tmpPrefix string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, tmpPrefix+"-*.tmp")
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

// WriteBinderAtomicImpl performs the atomic write via OS temp file rename.
func (w *fileAddChildIO) WriteBinderAtomicImpl(_ context.Context, path string, data []byte) error {
	if fi, statErr := os.Stat(path); statErr == nil {
		if fi.Mode().Perm()&0200 == 0 {
			return fmt.Errorf("binder file is read-only")
		}
	}
	return writeFileAtomicImpl(path, ".binder", data)
}

// WriteNodeFileAtomic writes content to path atomically (for --new mode).
func (w *fileAddChildIO) WriteNodeFileAtomic(path string, content []byte) error {
	return w.WriteNodeFileAtomicImpl(path, content)
}

// WriteNodeFileAtomicImpl performs the atomic write of a new node file.
func (w *fileAddChildIO) WriteNodeFileAtomicImpl(path string, content []byte) error {
	return writeFileAtomicImpl(path, ".node", content)
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

// OpenEditorImpl launches the editor process, splitting $EDITOR with strings.Fields.
func (w *fileAddChildIO) OpenEditorImpl(editor, path string) error {
	return openEditorImpl(editor, path)
}

// ReadNodeFile reads the node file at path.
func (w *fileAddChildIO) ReadNodeFile(path string) ([]byte, error) {
	return w.ReadNodeFileImpl(path)
}

// ReadNodeFileImpl reads the node file at path using os.ReadFile.
func (w *fileAddChildIO) ReadNodeFileImpl(path string) ([]byte, error) {
	return os.ReadFile(path)
}
