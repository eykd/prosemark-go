// Package cmd implements the pmk CLI commands.
package cmd

import (
	"encoding/json"
	"fmt"

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
	return root
}

func rootRunE(_ *cobra.Command, _ []string) error {
	return nil
}

// emitOPE009AndError writes an OPE009 JSON diagnostic to stdout and returns
// a non-nil error so the caller exits with non-zero code.
func emitOPE009AndError(cmd *cobra.Command, _ []byte, origErr error) error {
	out := binder.OpResult{
		Version: "1",
		Changed: false,
		Diagnostics: []binder.Diagnostic{{
			Severity: "error",
			Code:     binder.CodeIOOrParseFailure,
			Message:  fmt.Sprintf("I/O or parse failure: %v", origErr),
		}},
	}
	_ = json.NewEncoder(cmd.OutOrStdout()).Encode(out)
	return fmt.Errorf("operation failed: %w", origErr)
}
