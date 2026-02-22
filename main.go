// Package main is the entry point for the pmk CLI application.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information, injected at build time.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "pmk",
		Short: "pmk - prosemark CLI for long-form writing projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	rootCmd.Version = Version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
