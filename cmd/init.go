package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// InitIO handles I/O for the init command.
type InitIO interface {
	StatFile(path string) (bool, error)
	WriteFileAtomic(path, content string) error
}

// NewInitCmd creates the init subcommand.
func NewInitCmd(io InitIO) *cobra.Command {
	return newInitCmdWithGetCWD(io, os.Getwd)
}

func newInitCmdWithGetCWD(io InitIO, getwd func() (string, error)) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Initialize a prosemark project in the current directory",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			if project == "" {
				cwd, err := getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}
				project = cwd
			}

			binderPath := filepath.Join(project, "_binder.md")
			configPath := filepath.Join(project, ".prosemark.yml")

			binderExists, err := io.StatFile(binderPath)
			if err != nil {
				return fmt.Errorf("checking %s: %w", binderPath, err)
			}
			if binderExists && !force {
				return fmt.Errorf("_binder.md already exists in %s; use --force to overwrite", project)
			}

			needsWarning := force && binderExists

			const binderContent = "<!-- prosemark-binder:v1 -->\n"
			if err := io.WriteFileAtomic(binderPath, binderContent); err != nil {
				return fmt.Errorf("writing _binder.md: %w", err)
			}

			configExists, err := io.StatFile(configPath)
			if err != nil {
				return fmt.Errorf("checking %s: %w", configPath, err)
			}

			needsWarning = needsWarning || (force && configExists)

			if !configExists || force {
				const configContent = "# prosemark project configuration\n"
				if err := io.WriteFileAtomic(configPath, configContent); err != nil {
					return fmt.Errorf(
						"writing .prosemark.yml (partial init; re-run with --force to recover): %w", err)
				}
			}

			if needsWarning {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: overwriting existing files")
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Initialized "+project)
			return nil
		},
	}

	cmd.Flags().String("project", "", "project directory (default: current directory)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")

	return cmd
}

// fileInitIO implements InitIO using OS file I/O.
type fileInitIO struct{}

func newDefaultInitIO() *fileInitIO {
	return &fileInitIO{}
}

// StatFile returns true if the file at path exists, false if it does not.
// Returns an error only for unexpected OS errors.
func (f *fileInitIO) StatFile(path string) (bool, error) {
	return f.StatFileImpl(path)
}

// StatFileImpl wraps os.Stat to check file existence.
func (f *fileInitIO) StatFileImpl(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// WriteFileAtomic writes content to path atomically via a temp file with 0600 permissions.
func (f *fileInitIO) WriteFileAtomic(path, content string) error {
	return f.WriteFileAtomicImpl(path, content)
}

// WriteFileAtomicImpl performs the atomic write via OS temp file rename.
func (f *fileInitIO) WriteFileAtomicImpl(path, content string) error {
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
