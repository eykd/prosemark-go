package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/eykd/prosemark-go/internal/binder"
	"github.com/eykd/prosemark-go/internal/binder/ops"
)

// MoveIO handles I/O for the move command.
type MoveIO interface {
	ReadBinder(ctx context.Context, path string) ([]byte, error)
	ScanProject(ctx context.Context, binderPath string) (*binder.Project, error)
	WriteBinderAtomic(ctx context.Context, path string, data []byte) error
}

// NewMoveCmd creates the move subcommand.
func NewMoveCmd(io MoveIO) *cobra.Command {
	return newMoveCmdWithGetCWD(io, os.Getwd)
}

func newMoveCmdWithGetCWD(io MoveIO, getwd func() (string, error)) *cobra.Command {
	var (
		source   string
		dest     string
		first    bool
		at       int
		before   string
		after    string
		yes      bool
		jsonMode bool
	)

	cmd := &cobra.Command{
		Use:   "move",
		Short: "Move a node within a binder",
		Long:  "Move a node within a binder." + dryRunHelpSuffix,
		Example: `  # Move a node to a new parent
  pmk move --id abc123 --parent def456

  # Move a node to a specific position under its parent
  pmk move --id abc123 --parent def456 --position 2`,
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
				return fmt.Errorf("reading binder: %w", err)
			}

			proj, err := io.ScanProject(ctx, binderPath)
			if err != nil {
				return emitOPE009AndError(cmd, jsonMode, err)
			}

			if err := checkConflictingPositionFlags(cmd, first, before, after); err != nil {
				return err
			}

			position := "last"
			if first {
				position = "first"
			}

			params := binder.MoveParams{
				SourceSelector:            source,
				DestinationParentSelector: dest,
				Position:                  position,
				Before:                    before,
				After:                     after,
				Yes:                       yes || dryRun,
			}
			if cmd.Flags().Changed("at") {
				params.At = &at
			}

			modifiedBytes, diags := ops.Move(ctx, binderBytes, proj, params)
			diags = prepareDiagnostics(diags)

			changed := !bytes.Equal(binderBytes, modifiedBytes) && !dryRun

			if err := emitOpResult(cmd, jsonMode, changed, dryRun, diags, ""); err != nil {
				return err
			}

			if hasDiagnosticError(diags) {
				return &ExitError{Code: ExitCodeForDiagnostics(diags), Err: fmt.Errorf("move has errors")}
			}

			if changed {
				if err = io.WriteBinderAtomic(ctx, binderPath, modifiedBytes); err != nil {
					return fmt.Errorf("writing binder: %w", err)
				}
			}

			if !jsonMode {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), dryRunPrefix(dryRun)+"Moved "+sanitizePath(source)+" in "+sanitizePath(binderPath)); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().String("project", "", "project directory containing _binder.md (default: current directory)")
	cmd.Flags().StringVar(&source, "source", "", "Source selector")
	cmd.Flags().StringVar(&dest, "dest", "", "Destination parent selector")
	cmd.Flags().BoolVar(&first, "first", false, "Insert as first child")
	cmd.Flags().IntVar(&at, "at", 0, "Zero-based insertion index")
	cmd.Flags().StringVar(&before, "before", "", "Insert before selector")
	cmd.Flags().StringVar(&after, "after", "", "Insert after selector")
	cmd.Flags().BoolVar(&yes, "yes", false, "Required confirmation flag")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Output result as JSON")

	return cmd
}

// fileMoveIO implements MoveIO using OS file I/O.
type fileMoveIO struct{ binderLocker }

func newDefaultMoveIO() *fileMoveIO {
	return &fileMoveIO{}
}

// ReadBinder reads the binder file at path.
func (w *fileMoveIO) ReadBinder(_ context.Context, path string) ([]byte, error) {
	return readBinderSizeLimitedImpl(path)
}

// ScanProject scans the project directory for .md files.
func (w *fileMoveIO) ScanProject(ctx context.Context, binderPath string) (*binder.Project, error) {
	return ScanProjectImpl(ctx, binderPath)
}

// WriteBinderAtomic writes data to path atomically via a temp file.
func (w *fileMoveIO) WriteBinderAtomic(ctx context.Context, path string, data []byte) error {
	return w.WriteBinderAtomicImpl(ctx, path, data)
}

// WriteBinderAtomicImpl performs the atomic write via OS temp file rename.
func (w *fileMoveIO) WriteBinderAtomicImpl(_ context.Context, path string, data []byte) error {
	return writeBinderCheckedImpl(path, data)
}
