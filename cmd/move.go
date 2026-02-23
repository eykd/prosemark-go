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

// MoveIO handles I/O for the move command.
type MoveIO interface {
	ReadBinder(ctx context.Context, path string) ([]byte, error)
	ReadProject(ctx context.Context, path string) ([]byte, error)
	WriteBinderAtomic(ctx context.Context, path string, data []byte) error
}

// NewMoveCmd creates the move subcommand.
func NewMoveCmd(io MoveIO) *cobra.Command {
	var (
		projectPath string
		source      string
		dest        string
		first       bool
		at          int
		before      string
		after       string
		yes         bool
	)

	cmd := &cobra.Command{
		Use:          "move <binder-path>",
		Short:        "Move a node within a binder",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			binderPath := args[0]
			if filepath.Base(binderPath) != "_binder.md" {
				return fmt.Errorf("binder path must point to a file named _binder.md")
			}

			ctx := cmd.Context()

			binderBytes, err := io.ReadBinder(ctx, binderPath)
			if err != nil {
				return fmt.Errorf("reading binder: %w", err)
			}

			projectBytes, err := io.ReadProject(ctx, projectPath)
			if err != nil {
				return fmt.Errorf("reading project: %w", err)
			}

			var proj binder.Project
			if err = json.Unmarshal(projectBytes, &proj); err != nil {
				return fmt.Errorf("parsing project JSON: %w", err)
			}

			position := "last"
			if first {
				position = "first"
			}

			params := binder.MoveParams{
				SourceSelector:            source,
				DestinationParentSelector: dest,
				Position:                  position,
				At:                        nil,
				Before:                    before,
				After:                     after,
				Yes:                       yes,
			}
			_ = at

			modifiedBytes, diags, _ := ops.Move(ctx, binderBytes, &proj, params)
			if diags == nil {
				diags = []binder.Diagnostic{}
			}

			changed := !bytes.Equal(binderBytes, modifiedBytes)

			out := binder.OpResult{
				Version:     "1",
				Changed:     changed,
				Diagnostics: diags,
			}
			if encErr := json.NewEncoder(cmd.OutOrStdout()).Encode(out); encErr != nil {
				return fmt.Errorf("encoding output: %w", encErr)
			}

			for _, d := range diags {
				if d.Severity == "error" {
					return fmt.Errorf("move has errors")
				}
			}

			if changed {
				if err = io.WriteBinderAtomic(ctx, binderPath, modifiedBytes); err != nil {
					return fmt.Errorf("writing binder: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().Bool("json", false, "Output result as JSON")
	cmd.Flags().StringVar(&projectPath, "project", "", "Path to project.json")
	cmd.Flags().StringVar(&source, "source", "", "Source selector")
	cmd.Flags().StringVar(&dest, "dest", "", "Destination parent selector")
	cmd.Flags().BoolVar(&first, "first", false, "Insert as first child")
	cmd.Flags().IntVar(&at, "at", 0, "Zero-based insertion index")
	cmd.Flags().StringVar(&before, "before", "", "Insert before selector")
	cmd.Flags().StringVar(&after, "after", "", "Insert after selector")
	cmd.Flags().BoolVar(&yes, "yes", false, "Required confirmation flag")

	return cmd
}

// fileMoveIO implements MoveIO using OS file I/O.
type fileMoveIO struct{}

func newDefaultMoveIO() *fileMoveIO {
	return &fileMoveIO{}
}

// ReadBinder reads the binder file at path.
func (w *fileMoveIO) ReadBinder(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ReadProject reads the project file at path.
func (w *fileMoveIO) ReadProject(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteBinderAtomic writes data to path atomically via a temp file.
func (w *fileMoveIO) WriteBinderAtomic(ctx context.Context, path string, data []byte) error {
	return w.WriteBinderAtomicImpl(ctx, path, data)
}

// WriteBinderAtomicImpl performs the atomic write via OS temp file rename.
func (w *fileMoveIO) WriteBinderAtomicImpl(_ context.Context, path string, data []byte) error {
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
