// Package cmd implements the pmk CLI commands.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/eykd/prosemark-go/internal/binder"
)

// maxBinderFileSize is the maximum allowed size for _binder.md files (10 MB).
const maxBinderFileSize = 10 * 1024 * 1024

// dryRunAnnotationKey is the cobra annotation key declaring a command's dry-run policy.
const dryRunAnnotationKey = "dry-run"

// dryRunNoOp is the annotation value indicating --dry-run is accepted but has no effect.
const dryRunNoOp = "no-op"

// dryRunNoOpAnnotation returns the annotation map for read-only commands
// where --dry-run is accepted as a no-op (FR-018).
func dryRunNoOpAnnotation() map[string]string {
	return map[string]string{dryRunAnnotationKey: dryRunNoOp}
}

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

// NewRootCmd creates the root pmk command with all subcommands registered.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pmk",
		Short: "pmk - prosemark CLI for long-form writing projects",
		Long: `pmk - prosemark CLI for long-form writing projects

Exit Codes:
  0  Success
  1  Usage error
  2  Validation error
  3  Not found
  5  Conflict
  6  Transient

State Model:
  .prosemark.yml  project configuration
  _binder.md      outline and node registry

Environment Variables:
  EDITOR       editor for the edit command
  PMK_PROJECT  default project directory`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE:          rootRunE,
	}
	root.PersistentFlags().Bool(dryRunAnnotationKey, false, "preview changes without writing to disk")
	root.AddCommand(NewParseCmd(newDefaultParseReader()))
	root.AddCommand(NewAddChildCmd(newDefaultAddChildIO()))
	root.AddCommand(NewDeleteCmd(newDefaultDeleteIO()))
	root.AddCommand(NewMoveCmd(newDefaultMoveIO()))
	root.AddCommand(NewInitCmd(fileInitIO{}))
	root.AddCommand(NewEditCmd(fileEditIO{}))
	root.AddCommand(NewDoctorCmd(fileDoctorIO{}))
	return root
}

func rootRunE(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

// isDryRun returns true when the --dry-run persistent flag is set on cmd.
// Returns false if the flag is not defined (e.g. in unit tests without a root parent).
func isDryRun(cmd *cobra.Command) bool {
	f := cmd.Flags().Lookup(dryRunAnnotationKey)
	if f == nil {
		return false
	}
	return f.Value.String() == "true"
}

// resolveProjectDirFromCmd validates the --project flag and resolves the project directory.
// It returns an error if the flag was explicitly set to an empty string.
func resolveProjectDirFromCmd(cmd *cobra.Command, getwd func() (string, error)) (string, error) {
	project, _ := cmd.Flags().GetString("project")
	if cmd.Flags().Changed("project") && project == "" {
		return "", fmt.Errorf("--project flag cannot be empty")
	}
	if project == "" {
		project = os.Getenv("PMK_PROJECT")
	}
	if project == "" {
		cwd, err := getwd()
		if err != nil {
			return "", fmt.Errorf("getting working directory: %w", err)
		}
		return cwd, nil
	}
	return project, nil
}

// resolveBinderPathFromCmd validates the --project flag and resolves the binder path.
// It returns an error if the flag was explicitly set to an empty string.
func resolveBinderPathFromCmd(cmd *cobra.Command, getwd func() (string, error)) (string, error) {
	project, err := resolveProjectDirFromCmd(cmd, getwd)
	if err != nil {
		return "", err
	}
	return filepath.Join(project, "_binder.md"), nil
}

// resolveBinderPath derives the binder path from a project directory.
// If project is empty, getwd is called to determine the current directory.
func resolveBinderPath(project string, getwd func() (string, error)) (string, error) {
	if project == "" {
		cwd, err := getwd()
		if err != nil {
			return "", fmt.Errorf("getting working directory: %w", err)
		}
		project = cwd
	}
	return filepath.Join(project, "_binder.md"), nil
}

// emitOPE009AndError writes an OPE009 error diagnostic and returns a non-nil
// error so the caller exits with non-zero code. When jsonMode is true the
// diagnostic is written as a binder.OpResult JSON object to stdout; otherwise
// it is written as a human-readable message to stderr.
func emitOPE009AndError(cmd *cobra.Command, jsonMode bool, origErr error) error {
	if jsonMode {
		diags := []binder.Diagnostic{{Severity: "error", Code: binder.CodeIOOrParseFailure, Message: origErr.Error()}}
		out := binder.OpResult{Version: "1", Changed: false, Diagnostics: diags}
		if encErr := json.NewEncoder(cmd.OutOrStdout()).Encode(out); encErr != nil {
			return fmt.Errorf("encoding OPE009 diagnostic: %w: %w", encErr, origErr)
		}
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: I/O or parse failure: %v (OPE009)\n", origErr)
	}
	return fmt.Errorf("operation failed: %w", origErr)
}

// dryRunHelpSuffix is the help text appended to mutation subcommand Long
// descriptions to document --dry-run support.
const dryRunHelpSuffix = "\n\nUse --dry-run to preview changes without modifying any files."

// dryRunNoOpHelpSuffix is the help text appended to read-only subcommand Long
// descriptions to document that --dry-run is accepted but has no effect.
const dryRunNoOpHelpSuffix = "\n\nThe --dry-run flag is accepted but has no effect (no-op) on this read-only command."

// dryRunPrefix returns "dry-run: " when dryRun is true, or "" otherwise.
// Used by mutation commands to prefix human-readable output in dry-run mode.
func dryRunPrefix(dryRun bool) string {
	if dryRun {
		return "dry-run: "
	}
	return ""
}

// emitOpResult writes the operation result as JSON (when jsonMode is true) or
// prints diagnostics to stderr (when jsonMode is false). It is the shared
// output path for mutation commands (add, delete, move).
func emitOpResult(cmd *cobra.Command, jsonMode, changed, dryRun bool, diags []binder.Diagnostic) error {
	if jsonMode {
		out := binder.OpResult{Version: "1", Changed: changed, DryRun: dryRun, Diagnostics: diags}
		if err := json.NewEncoder(cmd.OutOrStdout()).Encode(out); err != nil {
			return fmt.Errorf("encoding output: %w", err)
		}
	} else {
		printDiagnostics(cmd, diags)
	}
	return nil
}

// printDiagnostics writes each diagnostic to stderr in human-readable form.
func printDiagnostics(cmd *cobra.Command, diags []binder.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s (%s)\n", d.Severity, d.Message, d.Code)
		if d.Suggestion != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "  suggestion: %s\n", d.Suggestion)
		}
	}
}

// checkConflictingPositionFlags returns an error if more than one of the
// mutually-exclusive positioning flags (--first, --at, --before, --after)
// is set. Both add and move commands share this validation.
func checkConflictingPositionFlags(cmd *cobra.Command, first bool, before, after string) error {
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
	return nil
}
