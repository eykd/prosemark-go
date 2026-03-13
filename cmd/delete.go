package cmd

import (
	"bytes"
	"context"
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
	RemoveFile(ctx context.Context, path string) error
}

// NewDeleteCmd creates the delete subcommand.
func NewDeleteCmd(io DeleteIO) *cobra.Command {
	return newDeleteCmdWithGetCWD(io, os.Getwd)
}

func newDeleteCmdWithGetCWD(io DeleteIO, getwd func() (string, error)) *cobra.Command {
	var (
		selector string
		yes      bool
		jsonMode bool
		rm       bool
	)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a node from a binder",
		Long:  "Delete a node from a binder." + dryRunHelpSuffix,
		Example: `  # Delete a node by its ID
  pmk delete --id abc123

  # Preview deletion without modifying files
  pmk delete --id abc123 --dry-run`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun := isDryRun(cmd)

			binderPath, err := resolveBinderPathFromCmd(cmd, getwd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			binderBytes, err := io.ReadBinder(ctx, binderPath)
			if err != nil {
				return emitOPE009AndError(cmd, jsonMode, err)
			}

			proj, err := io.ScanProject(ctx, binderPath)
			if err != nil {
				return emitOPE009AndError(cmd, jsonMode, err)
			}

			if selector == "" {
				return missingFlagError(cmd, jsonMode, dryRun, "delete", "--selector")
			}

			params := binder.DeleteParams{
				Selector: selector,
				Yes:      yes || dryRun,
			}

			modifiedBytes, diags := ops.Delete(ctx, binderBytes, proj, params)
			diags = prepareDiagnostics(diags)

			changed := !bytes.Equal(binderBytes, modifiedBytes) && !dryRun

			if err := emitOpResult(cmd, jsonMode, changed, dryRun, diags, ""); err != nil {
				return err
			}

			if hasDiagnosticError(diags) {
				return diagnosticExitError("delete", jsonMode, diags)
			}

			if changed {
				if err = io.WriteBinderAtomic(ctx, binderPath, modifiedBytes); err != nil {
					return writeBinderExitError(err)
				}
			}

			// Remove node files from disk when --rm is set and not dry-run.
			var removedTargets []string
			if rm && changed {
				removedTargets, err = deleteRemoveNodeFiles(ctx, io, binderBytes, modifiedBytes, proj)
				if err != nil {
					return err
				}
			}

			if !jsonMode {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), dryRunPrefix(dryRun)+"Deleted "+sanitizePath(selector)+" from "+sanitizePath(binderPath)); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
				for _, target := range removedTargets {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), "removed "+sanitizePath(target)); err != nil {
						return fmt.Errorf("writing output: %w", err)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().String("project", "", "project directory containing _binder.md (default: current directory)")
	cmd.Flags().StringVar(&selector, "selector", "", "Selector for node to delete")
	cmd.Flags().BoolVar(&yes, "yes", false, "Required confirmation flag")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Output result as JSON")
	cmd.Flags().BoolVar(&rm, "rm", false, "Also remove deleted node files from disk")

	return cmd
}

// deleteRemoveNodeFiles identifies node files that were removed by the delete
// operation and removes them from disk. It returns the list of removed target
// paths (relative to binder dir).
func deleteRemoveNodeFiles(ctx context.Context, io DeleteIO, originalBytes, modifiedBytes []byte, proj *binder.Project) ([]string, error) {
	origTargets := deleteCollectTargets(ctx, originalBytes, proj)
	modTargets := deleteCollectTargets(ctx, modifiedBytes, proj)

	modSet := make(map[string]struct{}, len(modTargets))
	for _, t := range modTargets {
		modSet[t] = struct{}{}
	}

	var removed []string
	for _, t := range origTargets {
		if _, ok := modSet[t]; !ok {
			fullPath := filepath.Join(proj.BinderDir, t)
			if err := io.RemoveFile(ctx, fullPath); err != nil {
				return removed, fmt.Errorf("removing node file %s: %w", t, err)
			}
			removed = append(removed, t)
		}
	}
	return removed, nil
}

// deleteCollectTargets parses binder bytes and returns all node targets.
func deleteCollectTargets(ctx context.Context, src []byte, proj *binder.Project) []string {
	result, _, err := binder.Parse(ctx, src, proj)
	if err != nil {
		return nil
	}
	return deleteWalkTargets(result.Root)
}

// deleteWalkTargets recursively collects all node targets in the tree.
func deleteWalkTargets(n *binder.Node) []string {
	var targets []string
	for _, child := range n.Children {
		if child.Target != "" {
			targets = append(targets, child.Target)
		}
		targets = append(targets, deleteWalkTargets(child)...)
	}
	return targets
}

// fileDeleteIO implements DeleteIO using OS file I/O.
type fileDeleteIO struct{ binderLocker }

func newDefaultDeleteIO() *fileDeleteIO {
	return &fileDeleteIO{}
}

// ReadBinder reads the binder file at path.
func (w *fileDeleteIO) ReadBinder(_ context.Context, path string) ([]byte, error) {
	return readBinderSizeLimitedImpl(path)
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
	return writeBinderCheckedImpl(path, data)
}

// RemoveFile removes a file from disk.
func (w *fileDeleteIO) RemoveFile(ctx context.Context, path string) error {
	return w.RemoveFileImpl(ctx, path)
}

// RemoveFileImpl performs the OS file removal.
func (w *fileDeleteIO) RemoveFileImpl(_ context.Context, path string) error {
	return os.Remove(path)
}
