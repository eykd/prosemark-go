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

// DeleteIO handles I/O for the delete command.
type DeleteIO interface {
	ReadBinder(ctx context.Context, path string) ([]byte, error)
	ReadProject(ctx context.Context, path string) ([]byte, error)
	WriteBinderAtomic(ctx context.Context, path string, data []byte) error
}

// deleteOutput is the JSON output schema for the delete command.
type deleteOutput struct {
	Version     string              `json:"version"`
	Changed     bool                `json:"changed"`
	Diagnostics []binder.Diagnostic `json:"diagnostics"`
}

// NewDeleteCmd creates the delete subcommand.
func NewDeleteCmd(io DeleteIO) *cobra.Command {
	var (
		projectPath string
		selector    string
		yes         bool
	)

	cmd := &cobra.Command{
		Use:          "delete <binder-path>",
		Short:        "Delete a node from a binder",
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
				return emitOPE009AndError(cmd, binderBytes, err)
			}

			var proj binder.Project
			if err = json.Unmarshal(projectBytes, &proj); err != nil {
				return emitOPE009AndError(cmd, binderBytes, err)
			}

			params := binder.DeleteParams{
				Selector: selector,
				Yes:      yes,
			}

			modifiedBytes, diags, _ := ops.Delete(ctx, binderBytes, &proj, params) //nolint:errcheck
			if diags == nil {
				diags = []binder.Diagnostic{}
			}

			changed := !bytes.Equal(binderBytes, modifiedBytes)

			out := deleteOutput{
				Version:     "1",
				Changed:     changed,
				Diagnostics: diags,
			}
			if encErr := json.NewEncoder(cmd.OutOrStdout()).Encode(out); encErr != nil {
				return fmt.Errorf("encoding output: %w", encErr)
			}

			for _, d := range diags {
				if d.Severity == "error" {
					return fmt.Errorf("delete has errors")
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
	cmd.Flags().StringVar(&selector, "selector", "", "Selector for node to delete")
	cmd.Flags().BoolVar(&yes, "yes", false, "Required confirmation flag")

	return cmd
}

// fileDeleteIO implements DeleteIO using OS file I/O.
type fileDeleteIO struct{}

func newDefaultDeleteIO() *fileDeleteIO {
	return &fileDeleteIO{}
}

// ReadBinder reads the binder file at path.
func (w *fileDeleteIO) ReadBinder(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ReadProject reads the project file at path.
func (w *fileDeleteIO) ReadProject(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteBinderAtomic writes data to path atomically via a temp file.
func (w *fileDeleteIO) WriteBinderAtomic(ctx context.Context, path string, data []byte) error {
	return w.WriteBinderAtomicImpl(ctx, path, data)
}

// WriteBinderAtomicImpl performs the atomic write via OS temp file rename.
func (w *fileDeleteIO) WriteBinderAtomicImpl(_ context.Context, path string, data []byte) error {
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
