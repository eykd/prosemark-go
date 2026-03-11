// Package cmd implements the pmk CLI commands.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/eykd/prosemark-go/internal/node"
)

// DoctorIO handles I/O for the doctor command.
type DoctorIO interface {
	// ReadBinder reads the raw binder file at path.
	ReadBinder(path string) ([]byte, error)
	// ListUUIDFiles returns UUID-pattern .md filenames found in dir (non-recursive).
	ListUUIDFiles(dir string) ([]string, error)
	// ReadNodeFile reads the file at path. The bool reports whether the file exists.
	ReadNodeFile(path string) ([]byte, bool, error)
}

// DoctorDiagnosticJSON is the JSON output type for a single doctor diagnostic.
type DoctorDiagnosticJSON struct {
	Severity   string `json:"severity"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Path       string `json:"path"`
	Suggestion string `json:"suggestion,omitempty"`
}

// doctorOutput is the wrapped JSON output for the doctor command.
type doctorOutput struct {
	Version     string                 `json:"version"`
	Diagnostics []DoctorDiagnosticJSON `json:"diagnostics"`
}

// NewDoctorCmd creates the doctor subcommand using os.Getwd for the working directory.
func NewDoctorCmd(io DoctorIO) *cobra.Command {
	return newDoctorCmdWithGetCWD(io, os.Getwd)
}

// newDoctorCmdWithGetCWD creates the doctor subcommand with an injectable getwd function.
func newDoctorCmdWithGetCWD(io DoctorIO, getwd func() (string, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate project structural integrity and frontmatter contracts",
		Long:  "Validate project structural integrity and frontmatter contracts." + dryRunNoOpHelpSuffix,
		Example: `  # Check the current project for issues
  pmk doctor

  # Output diagnostics as JSON
  pmk doctor --json`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Annotations:  dryRunNoOpAnnotation(),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, _ := cmd.Flags().GetBool("json")

			binderPath, err := resolveBinderPathFromCmd(cmd, getwd)
			if err != nil {
				return err
			}
			projectDir := filepath.Dir(binderPath)

			// Read binder — distinguish not-found from permission errors.
			binderBytes, err := io.ReadBinder(binderPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("project not initialized — run 'pmk init' first")
				}
				return fmt.Errorf("cannot read binder: %w", err)
			}

			// List UUID files in project root.
			var dirScanDiags []node.AuditDiagnostic
			uuidFiles, err := io.ListUUIDFiles(projectDir)
			if err != nil {
				uuidFiles = []string{}
				dirScanDiags = []node.AuditDiagnostic{{
					Code:     "AUDW002",
					Severity: node.SeverityWarning,
					Message:  fmt.Sprintf("directory scan failed: %v", err),
					Path:     projectDir,
				}}
			}

			// Collect binder refs and binder-level diagnostics (escape warnings, duplicates).
			// This is the sole binder.Parse call per doctor invocation.
			refs, refDiags := node.CollectBinderRefs(cmd.Context(), binderBytes)

			// Build FileContents map: one entry per unique referenced filename.
			fileContents := make(map[string][]byte, len(refs))
			for _, ref := range refs {
				fileContents[ref] = doctorReadFile(io, projectDir, ref)
			}

			data := node.DoctorData{
				BinderSrc:      binderBytes,
				UUIDFiles:      uuidFiles,
				FileContents:   fileContents,
				BinderRefs:     refs,
				BinderRefDiags: refDiags,
			}

			configDiags := checkProjectConfig(io, projectDir)
			diags := node.RunDoctor(cmd.Context(), data)
			diags = append(diags, configDiags...)
			diags = append(diags, dirScanDiags...)

			// Emit diagnostics.
			jsonDiags := toDoctorDiagnosticJSON(diags)
			attachAuditSuggestions(jsonDiags)
			if jsonMode {
				out := doctorOutput{Version: "1", Diagnostics: jsonDiags}
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(out); err != nil {
					return fmt.Errorf("encoding output: %w", err)
				}
			} else {
				for _, d := range jsonDiags {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s %-7s %s\n",
						d.Code,
						d.Severity,
						sanitizePath(d.Message),
					)
					if d.Suggestion != "" {
						fmt.Fprintf(cmd.ErrOrStderr(), "  suggestion: %s\n", d.Suggestion)
					}
				}
			}

			if hasAuditDiagnosticError(diags) {
				return &ExitError{Code: ExitCodeForAuditDiagnostics(diags), Err: fmt.Errorf("project has integrity errors")}
			}

			return nil
		},
	}

	cmd.Flags().String("project", "", "project directory to audit (default: current directory)")
	cmd.Flags().Bool("json", false, "output diagnostics as JSON")

	return cmd
}

// checkProjectConfig validates .prosemark.yml existence and YAML integrity.
// Returns an AUD008 error diagnostic if the file is missing, unreadable, or contains invalid YAML.
func checkProjectConfig(io DoctorIO, projectDir string) []node.AuditDiagnostic {
	configPath := filepath.Join(projectDir, ".prosemark.yml")
	content, exists, err := io.ReadNodeFile(configPath)

	var msg string
	if err != nil || !exists {
		msg = ".prosemark.yml is missing or unreadable"
	} else {
		var cfg interface{}
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			msg = ".prosemark.yml contains invalid YAML"
		}
	}

	if msg == "" {
		return nil
	}
	return []node.AuditDiagnostic{{
		Code:     node.AUD008,
		Severity: node.SeverityError,
		Message:  msg,
		Path:     ".prosemark.yml",
	}}
}

// doctorReadFile reads a binder-referenced file for doctor analysis.
// Returns nil if the file does not exist or cannot be read.
// Returns []byte{} (empty, non-nil) for files exceeding 1 MB, causing RunDoctor
// to emit AUD007 (frontmatter parse failure) rather than AUD001 (file not found).
func doctorReadFile(io DoctorIO, projectDir, ref string) []byte {
	resolved := filepath.Join(projectDir, ref)
	content, exists, err := io.ReadNodeFile(resolved)
	if err != nil || !exists {
		return nil
	}
	if len(content) > 1024*1024 {
		return []byte{}
	}
	return content
}

// toDoctorDiagnosticJSON converts audit diagnostics to the JSON output type.
func toDoctorDiagnosticJSON(diags []node.AuditDiagnostic) []DoctorDiagnosticJSON {
	jsonDiags := make([]DoctorDiagnosticJSON, len(diags))
	for i, d := range diags {
		jsonDiags[i] = DoctorDiagnosticJSON{
			Severity: string(d.Severity),
			Code:     string(d.Code),
			Message:  d.Message,
			Path:     d.Path,
		}
	}
	return jsonDiags
}

// fileDoctorIO implements DoctorIO using OS file I/O.
// *Impl methods wrap OS calls and are excluded from coverage requirements.
type fileDoctorIO struct{}

// ReadBinder reads the binder file at path.
func (f fileDoctorIO) ReadBinder(path string) ([]byte, error) {
	return f.ReadBinderImpl(path)
}

// ReadBinderImpl reads the binder file using os.ReadFile.
func (f fileDoctorIO) ReadBinderImpl(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ListUUIDFiles returns UUID-pattern .md filenames found in dir.
func (f fileDoctorIO) ListUUIDFiles(dir string) ([]string, error) {
	return f.ListUUIDFilesImpl(dir)
}

// ListUUIDFilesImpl reads the directory and filters for UUID-pattern .md files.
func (f fileDoctorIO) ListUUIDFilesImpl(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		if !e.IsDir() && node.IsUUIDFilename(e.Name()) {
			result = append(result, e.Name())
		}
	}
	return result, nil
}

// ReadNodeFile reads the node file at path, returning content, existence flag, and error.
func (f fileDoctorIO) ReadNodeFile(path string) ([]byte, bool, error) {
	return f.ReadNodeFileImpl(path)
}

// ReadNodeFileImpl reads the file using os.ReadFile, mapping ErrNotExist to exists=false.
func (f fileDoctorIO) ReadNodeFileImpl(path string) ([]byte, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return content, true, nil
}
