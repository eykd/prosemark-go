package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/eykd/prosemark-go/internal/binder"
)

// ParseReader reads binder and project files for the parse command.
type ParseReader interface {
	ReadBinder(ctx context.Context, path string) ([]byte, error)
	ReadProject(ctx context.Context, path string) ([]byte, error)
}

// parseOutput is the JSON output schema for the parse command.
type parseOutput struct {
	Version     string              `json:"version"`
	Root        *binder.Node        `json:"root"`
	Diagnostics []binder.Diagnostic `json:"diagnostics"`
}

// NewParseCmd creates the parse subcommand.
func NewParseCmd(reader ParseReader) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:          "parse <binder-path>",
		Short:        "Parse a binder file and output JSON",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			binderPath := args[0]
			if filepath.Base(binderPath) != "_binder.md" {
				return fmt.Errorf("binder path must point to a file named _binder.md")
			}

			ctx := cmd.Context()

			binderBytes, err := reader.ReadBinder(ctx, binderPath)
			if err != nil {
				return fmt.Errorf("reading binder: %w", err)
			}

			projectBytes, err := reader.ReadProject(ctx, projectPath)
			if err != nil {
				return fmt.Errorf("reading project: %w", err)
			}

			var proj binder.Project
			if err = json.Unmarshal(projectBytes, &proj); err != nil {
				return fmt.Errorf("parsing project JSON: %w", err)
			}

			// binder.Parse accumulates errors into diagnostics; the error return is always nil.
			result, diags, _ := binder.Parse(ctx, binderBytes, &proj) //nolint:errcheck
			if diags == nil {
				diags = []binder.Diagnostic{}
			}

			out := parseOutput{
				Version:     result.Version,
				Root:        result.Root,
				Diagnostics: diags,
			}
			if err = json.NewEncoder(cmd.OutOrStdout()).Encode(out); err != nil {
				return fmt.Errorf("encoding output: %w", err)
			}

			for _, d := range diags {
				if d.Severity == "error" {
					return fmt.Errorf("binder has parse errors")
				}
			}
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "Output parse result as JSON")
	cmd.Flags().StringVar(&projectPath, "project", "", "Path to project.json")
	return cmd
}

// fileParseReader implements ParseReader using OS file I/O.
type fileParseReader struct{}

func newDefaultParseReader() *fileParseReader {
	return &fileParseReader{}
}

func (r *fileParseReader) ReadBinder(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (r *fileParseReader) ReadProject(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}
