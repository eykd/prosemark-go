package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/eykd/prosemark-go/internal/binder"
	"github.com/spf13/cobra"
)

const (
	// initBinderContent is the initial content for a new _binder.md file.
	initBinderContent = "<!-- prosemark-binder:v1 -->\n"
	// initConfigContent is the initial content for a new .prosemark.yml file.
	initConfigContent = "version: \"1\"\n"

	diagCodeBinderCreated  = "OPI001"
	diagCodeConfigCreated  = "OPI002"
	diagCodeBinderDataLoss = "OPW006"
)

// InitIO handles I/O for the init command.
type InitIO interface {
	StatFile(path string) (bool, error)
	ReadFile(path string) (string, error)
	WriteFileAtomic(path, content string) error
}

// NewInitCmd creates the init subcommand.
func NewInitCmd(io InitIO) *cobra.Command {
	return newInitCmdWithGetCWD(io, os.Getwd)
}

func newInitCmdWithGetCWD(initIO InitIO, getwd func() (string, error)) *cobra.Command {
	var (
		force    bool
		jsonMode bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a prosemark project in the current directory",
		Long:  "Initialize a prosemark project in the current directory." + dryRunHelpSuffix,
		Example: `  # Initialize a new prosemark project
  pmk init

  # Preview initialization without creating files
  pmk init --dry-run`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun := isDryRun(cmd)

			project, err := resolveProjectDirFromCmd(cmd, getwd)
			if err != nil {
				return err
			}

			binderPath := filepath.Join(project, "_binder.md")
			configPath := filepath.Join(project, ".prosemark.yml")

			binderExists, err := initIO.StatFile(binderPath)
			if err != nil {
				return emitOPE009AndError(cmd, jsonMode, fmt.Errorf("checking %s: %w", binderPath, err))
			}
			if binderExists && !force {
				return emitOPE009AndError(cmd, jsonMode,
					fmt.Errorf("_binder.md already exists in %s; use --force to overwrite", project))
			}

			var binderNodeCount int
			if force && binderExists {
				content, err := initIO.ReadFile(binderPath)
				if err != nil {
					return emitOPE009AndError(cmd, jsonMode, fmt.Errorf("reading %s: %w", binderPath, err))
				}
				binderNodeCount = countBinderNodes(content)
			}

			changed := !dryRun

			if changed {
				if err := initIO.WriteFileAtomic(binderPath, initBinderContent); err != nil {
					return emitOPE009AndError(cmd, jsonMode, fmt.Errorf("writing _binder.md: %w", err))
				}
			}

			configExists, err := initIO.StatFile(configPath)
			if err != nil {
				return emitOPE009AndError(cmd, jsonMode, fmt.Errorf("checking %s: %w", configPath, err))
			}

			needsWarning := force && (binderExists || configExists)
			writeConfig := !configExists || force

			diags := []binder.Diagnostic{
				{Severity: "info", Code: diagCodeBinderCreated, Message: "created _binder.md"},
			}
			if writeConfig {
				diags = append(diags, binder.Diagnostic{
					Severity: "info", Code: diagCodeConfigCreated, Message: "created .prosemark.yml",
				})
			}

			if changed && writeConfig {
				if err := initIO.WriteFileAtomic(configPath, initConfigContent); err != nil {
					return emitOPE009AndError(cmd, jsonMode,
						fmt.Errorf("writing .prosemark.yml (partial init; re-run with --force to recover): %w", err))
				}
			}

			if binderNodeCount > 0 {
				diags = append(diags, binder.Diagnostic{
					Severity: "warning",
					Code:     diagCodeBinderDataLoss,
					Message:  fmt.Sprintf("existing binder contains %d node entries that will be lost", binderNodeCount),
				})
			}

			diags = prepareDiagnostics(diags)

			if jsonMode {
				out := binder.OpResult{Version: "1", Changed: changed, DryRun: dryRun, Diagnostics: diags}
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(out); err != nil {
					return fmt.Errorf("encoding output: %w", err)
				}
				return nil
			}

			if binderNodeCount > 0 {
				msg := fmt.Sprintf("warning: existing binder contains %d node entries that will be lost", binderNodeCount)
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), msg); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
			}

			if needsWarning {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "warning: overwriting existing files"); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
			}

			printDiagnostics(cmd, diags)

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), dryRunPrefix(dryRun)+"Initialized "+sanitizePath(project)); err != nil {
				return fmt.Errorf("writing output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().String("project", "", "project directory (default: current directory)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Output result as JSON")

	return cmd
}

// countBinderNodes parses binder content and returns the number of node entries.
func countBinderNodes(content string) int {
	result, _, err := binder.Parse(context.Background(), []byte(content), nil)
	if err != nil || result == nil || result.Root == nil {
		return 0
	}
	return countNodesRecursive(result.Root)
}

// countNodesRecursive counts all non-root nodes in the binder tree.
func countNodesRecursive(n *binder.Node) int {
	count := 0
	for _, child := range n.Children {
		count += 1 + countNodesRecursive(child)
	}
	return count
}

// fileInitIO implements InitIO using OS file I/O.
type fileInitIO struct{}

// StatFile returns true if the file at path exists, false if it does not.
// Returns an error only for unexpected OS errors.
func (f fileInitIO) StatFile(path string) (bool, error) {
	return f.StatFileImpl(path)
}

// StatFileImpl wraps os.Stat to check file existence.
func (f fileInitIO) StatFileImpl(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ReadFileImpl reads the contents of the file at path.
func (f fileInitIO) ReadFileImpl(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadFile reads the contents of the file at path.
func (f fileInitIO) ReadFile(path string) (string, error) {
	return f.ReadFileImpl(path)
}

// WriteFileAtomic writes content to path atomically via a temp file with 0600 permissions.
func (f fileInitIO) WriteFileAtomic(path, content string) error {
	return f.WriteFileAtomicImpl(path, content)
}

// WriteFileAtomicImpl performs the atomic write via OS temp file rename.
func (f fileInitIO) WriteFileAtomicImpl(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".init-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write([]byte(content)); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err = os.Chmod(tmpName, 0600); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
