// Package cmd implements the pmk CLI commands.
package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/eykd/prosemark-go/internal/binder"
)

// NewRootCmd creates the root pmk command with all subcommands registered.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "pmk",
		Short:         "pmk - prosemark CLI for long-form writing projects",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE:          rootRunE,
	}
	root.AddCommand(NewParseCmd(newDefaultParseReader()))
	root.AddCommand(NewAddChildCmd(newDefaultAddChildIO()))
	root.AddCommand(NewDeleteCmd(newDefaultDeleteIO()))
	root.AddCommand(NewMoveCmd(newDefaultMoveIO()))
	root.AddCommand(NewInitCmd(fileInitIO{}))
	root.AddCommand(NewEditCmd(fileEditIO{}))
	return root
}

func rootRunE(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
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
		_ = json.NewEncoder(cmd.OutOrStdout()).Encode(out)
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: I/O or parse failure: %v (OPE009)\n", origErr)
	}
	return fmt.Errorf("operation failed: %w", origErr)
}

// printDiagnostics writes each diagnostic to stderr in human-readable form.
func printDiagnostics(cmd *cobra.Command, diags []binder.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s (%s)\n", d.Severity, d.Message, d.Code)
	}
}
