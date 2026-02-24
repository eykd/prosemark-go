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
	ScanProject(ctx context.Context, binderPath string) (*binder.Project, error)
	WriteBinderAtomic(ctx context.Context, path string, data []byte) error
}

// NewDeleteCmd creates the delete subcommand.
func NewDeleteCmd(io DeleteIO) *cobra.Command {
	var (
		selector string
		yes      bool
		jsonMode bool
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

			proj, err := io.ScanProject(ctx, binderPath)
			if err != nil {
				return emitOPE009AndError(cmd, jsonMode, err)
			}

			params := binder.DeleteParams{
				Selector: selector,
				Yes:      yes,
			}

			modifiedBytes, diags, _ := ops.Delete(ctx, binderBytes, proj, params) //nolint:errcheck
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
				return fmt.Errorf("delete has errors")
			}

			if changed {
				if err = io.WriteBinderAtomic(ctx, binderPath, modifiedBytes); err != nil {
					return fmt.Errorf("writing binder: %w", err)
				}
			}

			if !jsonMode {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Deleted "+selector+" from "+binderPath); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&selector, "selector", "", "Selector for node to delete")
	cmd.Flags().BoolVar(&yes, "yes", false, "Required confirmation flag")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Output result as JSON")

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

// ScanProject scans the project directory for .md files.
func (w *fileDeleteIO) ScanProject(ctx context.Context, binderPath string) (*binder.Project, error) {
	return ScanProjectImpl(ctx, binderPath)
}

// WriteBinderAtomic writes data to path atomically via a temp file.
func (w *fileDeleteIO) WriteBinderAtomic(ctx context.Context, path string, data []byte) error {
	return w.WriteBinderAtomicImpl(ctx, path, data)
}

// WriteBinderAtomicImpl performs the atomic write via OS temp file rename.
func (w *fileDeleteIO) WriteBinderAtomicImpl(_ context.Context, path string, data []byte) error {
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
