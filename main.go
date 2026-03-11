// Package main is the entry point for the pmk CLI application.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/eykd/prosemark-go/cmd"
)

// Version information, injected at build time.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	rootCmd.Version = Version
	err := rootCmd.Execute()
	os.Exit(handleCommandError(err, os.Stderr))
}

// handleCommandError extracts exit codes from ExitError or defaults to 1.
func handleCommandError(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	var exitErr *cmd.ExitError
	if errors.As(err, &exitErr) {
		fmt.Fprintln(stderr, exitErr.Err)
		return exitErr.Code
	}
	fmt.Fprintln(stderr, err)
	return 1
}
