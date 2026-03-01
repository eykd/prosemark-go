// Package cmd implements the pmk CLI commands.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

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
// Contains only code, message, and path — severity is excluded per
// the doctor-diagnostic.json schema (additionalProperties: false).
type DoctorDiagnosticJSON struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

// NewDoctorCmd creates the doctor subcommand using os.Getwd for the working directory.
func NewDoctorCmd(io DoctorIO) *cobra.Command {
	return newDoctorCmdWithGetCWD(io, os.Getwd)
}

// newDoctorCmdWithGetCWD creates the doctor subcommand with an injectable getwd function.
func newDoctorCmdWithGetCWD(io DoctorIO, getwd func() (string, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "doctor",
		Short:        "Validate project structural integrity and frontmatter contracts",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			jsonMode, _ := cmd.Flags().GetBool("json")

			binderPath, err := resolveBinderPath(project, getwd)
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
			uuidFiles, err := io.ListUUIDFiles(projectDir)
			if err != nil {
				uuidFiles = []string{}
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

			diags := node.RunDoctor(cmd.Context(), data)

			// Emit diagnostics.
			if jsonMode {
				jsonDiags := make([]DoctorDiagnosticJSON, len(diags))
				for i, d := range diags {
					jsonDiags[i] = DoctorDiagnosticJSON{
						Code:    string(d.Code),
						Message: d.Message,
						Path:    d.Path,
					}
				}
				_ = json.NewEncoder(cmd.OutOrStdout()).Encode(jsonDiags)
			} else {
				for _, d := range diags {
					fmt.Fprintf(cmd.OutOrStdout(), "%s %-7s %s\n",
						string(d.Code),
						string(d.Severity),
						sanitizePath(d.Message),
					)
				}
			}

			if hasErrorDiagnostic(diags) {
				return fmt.Errorf("project has integrity errors")
			}

			return nil
		},
	}

	cmd.Flags().String("project", "", "project directory to audit (default: current directory)")
	cmd.Flags().Bool("json", false, "output diagnostics as JSON array")

	return cmd
}

// hasErrorDiagnostic reports whether any diagnostic in the slice has error severity.
func hasErrorDiagnostic(diags []node.AuditDiagnostic) bool {
	for _, d := range diags {
		if d.Severity == node.SeverityError {
			return true
		}
	}
	return false
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
