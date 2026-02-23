// Package main is the entry point for the pmk CLI application.
package main

import (
	"fmt"
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
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
