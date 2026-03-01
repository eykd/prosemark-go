package node

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/eykd/prosemark-go/internal/binder"
)

// DoctorData holds pre-loaded file data for a doctor audit pass.
// FileContents maps filenames to their raw bytes; a nil value means the file
// does not exist on disk.
type DoctorData struct {
	// BinderSrc is the raw content of the project's _binder.md file.
	BinderSrc []byte
	// UUIDFiles is the list of UUID-pattern .md filenames found in the project root.
	UUIDFiles []string
	// FileContents maps each filename to its raw bytes.
	// A nil value indicates the file does not exist on disk.
	FileContents map[string][]byte
}

// RunDoctor performs all audit checks on the provided pre-loaded project data
// and returns diagnostics sorted by severity (errors first) then path.
// It is a pure function and performs no IO.
func RunDoctor(ctx context.Context, data DoctorData) []AuditDiagnostic {
	var diags []AuditDiagnostic

	// Step 1: Parse the binder.
	parseResult, _, _ := binder.Parse(ctx, data.BinderSrc, nil)

	// Step 2: Walk binder tree to collect references and detect duplicates (AUD003).
	visited := make(map[string]bool)
	duplicated := make(map[string]bool)
	var refs []string

	var walkNodes func(nodes []*binder.Node)
	walkNodes = func(nodes []*binder.Node) {
		for _, n := range nodes {
			if n.Target != "" {
				if !visited[n.Target] {
					visited[n.Target] = true
					refs = append(refs, n.Target)
				} else if !duplicated[n.Target] {
					duplicated[n.Target] = true
					diags = append(diags, AuditDiagnostic{
						Code:     AUD003,
						Severity: SeverityError,
						Message:  fmt.Sprintf("file appears more than once in binder: %s", n.Target),
						Path:     n.Target,
					})
				}
			}
			walkNodes(n.Children)
		}
	}
	walkNodes(parseResult.Root.Children)

	// Step 3: Check each uniquely referenced file.
	for _, ref := range refs {
		isUUID := IsUUIDFilename(ref)

		// AUDW001: non-UUID filename linked in binder.
		if !isUUID {
			diags = append(diags, AuditDiagnostic{
				Code:     AUDW001,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("non-UUID filename linked in binder: %s", ref),
				Path:     ref,
			})
		}

		// AUD001: referenced file does not exist.
		content, ok := data.FileContents[ref]
		if !ok || content == nil {
			diags = append(diags, AuditDiagnostic{
				Code:     AUD001,
				Severity: SeverityError,
				Message:  fmt.Sprintf("referenced file does not exist: %s", ref),
				Path:     ref,
			})
			continue
		}

		// No frontmatter checks for non-UUID files.
		if !isUUID {
			continue
		}

		// AUD007: parse frontmatter.
		stem := strings.TrimSuffix(ref, ".md")
		fm, body, err := ParseFrontmatter(content)
		if err != nil {
			diags = append(diags, AuditDiagnostic{
				Code:     AUD007,
				Severity: SeverityError,
				Message:  fmt.Sprintf("frontmatter YAML is syntactically invalid: %v", err),
				Path:     ref,
			})
			continue
		}

		// AUD004, AUD005, AUD006 via ValidateNode.
		for _, d := range ValidateNode(stem, fm, body) {
			d.Path = ref
			diags = append(diags, d)
		}
	}

	// Step 4: Detect orphaned UUID files (AUD002).
	for _, uuidFile := range data.UUIDFiles {
		if !visited[uuidFile] {
			diags = append(diags, AuditDiagnostic{
				Code:     AUD002,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("orphaned UUID file not referenced in binder: %s", uuidFile),
				Path:     uuidFile,
			})
		}
	}

	// Sort: errors before warnings, then alphabetically by path within each tier.
	sort.SliceStable(diags, func(i, j int) bool {
		si := severityRank(diags[i].Severity)
		sj := severityRank(diags[j].Severity)
		if si != sj {
			return si < sj
		}
		return diags[i].Path < diags[j].Path
	})

	return diags
}

// severityRank returns a numeric rank for sorting: errors (0) sort before warnings (1).
func severityRank(s AuditSeverity) int {
	if s == SeverityError {
		return 0
	}
	return 1
}
