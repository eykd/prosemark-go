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
	// BinderRefs, when non-nil, provides deduplicated refs pre-computed by CollectBinderRefs.
	// RunDoctor uses these directly and skips re-parsing BinderSrc.
	BinderRefs []string
	// BinderRefDiags holds diagnostics produced by CollectBinderRefs (escape warnings, AUD003).
	// Ignored unless BinderRefs is non-nil.
	BinderRefDiags []AuditDiagnostic
}

// RunDoctor performs all audit checks on the provided pre-loaded project data
// and returns diagnostics sorted by severity (errors first) then path.
// It is a pure function and performs no IO.
//
// When DoctorData.BinderRefs is non-nil, RunDoctor uses those pre-computed refs
// and skips re-parsing BinderSrc, ensuring binder.Parse is called at most once
// per doctor invocation.
func RunDoctor(ctx context.Context, data DoctorData) []AuditDiagnostic {
	var diags []AuditDiagnostic
	var refs []string
	visited := make(map[string]bool)

	if data.BinderRefs != nil {
		// Fast path: use pre-computed refs and diags from CollectBinderRefs.
		diags = append(diags, data.BinderRefDiags...)
		refs = data.BinderRefs
		for _, ref := range refs {
			visited[ref] = true
		}
	} else {
		// Legacy path: parse the binder and detect duplicates.
		parseResult, _, _ := binder.Parse(ctx, data.BinderSrc, nil)

		duplicated := make(map[string]bool)
		var walkNodes func(nodes []*binder.Node)
		walkNodes = func(nodes []*binder.Node) {
			for _, n := range nodes {
				if n.Target != "" {
					if !visited[n.Target] {
						visited[n.Target] = true
						refs = append(refs, n.Target)
					} else if !duplicated[n.Target] {
						duplicated[n.Target] = true
						diags = append(diags, errDiag(AUD003, n.Target, fmt.Sprintf("file appears more than once in binder: %s", n.Target)))
					}
				}
				walkNodes(n.Children)
			}
		}
		walkNodes(parseResult.Root.Children)
	}

	// Check each uniquely referenced file.
	for _, ref := range refs {
		isUUID := IsUUIDFilename(ref)

		// AUDW001: non-UUID filename linked in binder.
		if !isUUID {
			diags = append(diags, warnDiag(AUDW001, ref, fmt.Sprintf("non-UUID filename linked in binder: %s", ref)))
		}

		// AUD001: referenced file does not exist.
		content, ok := data.FileContents[ref]
		if !ok || content == nil {
			diags = append(diags, errDiag(AUD001, ref, fmt.Sprintf("referenced file does not exist: %s", ref)))
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
			diags = append(diags, errDiag(AUD007, ref, fmt.Sprintf("frontmatter YAML is syntactically invalid: %v", err)))
			continue
		}

		// AUD004, AUD005, AUD006 via ValidateNode.
		for _, d := range ValidateNode(stem, fm, body) {
			d.Path = ref
			diags = append(diags, d)
		}
	}

	// Detect orphaned UUID files (AUD002).
	for _, uuidFile := range data.UUIDFiles {
		if !visited[uuidFile] {
			diags = append(diags, warnDiag(AUD002, uuidFile, fmt.Sprintf("orphaned UUID file not referenced in binder: %s", uuidFile)))
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

// errDiag constructs an error-severity AuditDiagnostic.
func errDiag(code AuditCode, path, message string) AuditDiagnostic {
	return AuditDiagnostic{Code: code, Severity: SeverityError, Message: message, Path: path}
}

// warnDiag constructs a warning-severity AuditDiagnostic.
func warnDiag(code AuditCode, path, message string) AuditDiagnostic {
	return AuditDiagnostic{Code: code, Severity: SeverityWarning, Message: message, Path: path}
}
