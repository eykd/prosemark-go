package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Added "+target+" to "+binderPath); err != nil {
						return fmt.Errorf("writing output: %w", err)
					}
				} else {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), target+" already in "+binderPath+" (skipped)"); err != nil {
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

	return cmd
}

// fileAddChildIO implements AddChildIO using OS file I/O.
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
