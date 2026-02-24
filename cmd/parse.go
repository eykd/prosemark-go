package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/eykd/prosemark-go/internal/binder"
)

// ParseReader reads the binder file and scans the project directory for the parse command.
type ParseReader interface {
	ReadBinder(ctx context.Context, path string) ([]byte, error)
	ScanProject(ctx context.Context, binderPath string) (*binder.Project, error)
}

// parseOutput is the JSON output schema for the parse command.
type parseOutput struct {
	Version     string              `json:"version"`
	Root        *binder.Node        `json:"root"`
	Diagnostics []binder.Diagnostic `json:"diagnostics"`
}

// NewParseCmd creates the parse subcommand.
func NewParseCmd(reader ParseReader) *cobra.Command {
	return newParseCmdWithGetCWD(reader, os.Getwd)
}

func newParseCmdWithGetCWD(reader ParseReader, getwd func() (string, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "parse",
		Short:        "Parse a binder file and output JSON",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			binderPath, err := resolveBinderPath(project, getwd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			binderBytes, err := reader.ReadBinder(ctx, binderPath)
			if err != nil {
				return fmt.Errorf("reading binder: %w", err)
			}

			proj, err := reader.ScanProject(ctx, binderPath)
			if err != nil {
				return fmt.Errorf("scanning project: %w", err)
			}

			result, diags, parseErr := binder.Parse(ctx, binderBytes, proj)
			if parseErr != nil {
				diags = append(diags, binder.Diagnostic{
					Severity: "error",
					Code:     binder.CodeIOOrParseFailure,
					Message:  fmt.Sprintf("parse error: %v", parseErr),
				})
			}
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

	cmd.Flags().String("project", "", "project directory containing _binder.md (default: current directory)")
	cmd.Flags().Bool("json", false, "Output result as JSON (always enabled for parse)")

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

// ScanProject scans the project directory for .md files.
func (r *fileParseReader) ScanProject(ctx context.Context, binderPath string) (*binder.Project, error) {
	return ScanProjectImpl(ctx, binderPath)
}
