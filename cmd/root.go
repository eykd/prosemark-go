// Package cmd implements the pmk CLI commands.
package cmd

import (
	"github.com/spf13/cobra"
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
	return root
}

func rootRunE(_ *cobra.Command, _ []string) error {
	return nil
}
