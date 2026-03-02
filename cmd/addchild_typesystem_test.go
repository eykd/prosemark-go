package cmd

import (
	"github.com/spf13/cobra"
)

// Compile-time assertions: NewAddChildCmd and newAddChildCmdWithGetCWD must
// accept NewNodeAddChildIO (not AddChildIO) so that --new mode capability is
// expressed in the type system rather than discovered at runtime via assertion.
// These serve as regression guards against future signature regressions.
var (
	_ func(NewNodeAddChildIO) *cobra.Command                         = NewAddChildCmd
	_ func(NewNodeAddChildIO, func() (string, error)) *cobra.Command = newAddChildCmdWithGetCWD
)
