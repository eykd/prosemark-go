package node

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/eykd/prosemark-go/internal/binder"
)

// binderLinkTargetRE finds markdown inline link targets in binder source.
var binderLinkTargetRE = regexp.MustCompile(`\]\(([^)]+)\)`)

// CollectBinderRefs parses raw binder source and returns:
//   - refs: the deduplicated list of valid (non-escaping) file references from the parsed binder tree
//   - diags: diagnostics for path-escaping links (AUDW001) and duplicate references (AUD003)
//
// binder.Parse already rejects escaping paths from the parse tree, so the raw-byte
// regex scan is required to surface those targets as diagnostics.
func CollectBinderRefs(ctx context.Context, binderSrc []byte) ([]string, []AuditDiagnostic) {
	var diags []AuditDiagnostic

	// Scan raw bytes for path-escaping links that binder.Parse rejects from the tree.
	for _, m := range binderLinkTargetRE.FindAllSubmatch(binderSrc, -1) {
		target := string(m[1])
		if target == ".." || strings.HasPrefix(target, "../") {
			diags = append(diags, AuditDiagnostic{
				Code:     AUDW001,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("binder link escapes project directory: %s", target),
				Path:     target,
			})
		}
	}

	// Parse binder tree to collect valid (non-escaping) refs and detect duplicates.
	parseResult, _, _ := binder.Parse(ctx, binderSrc, nil)

	visited := make(map[string]bool)
	duplicated := make(map[string]bool)
	var refs []string

	var walk func([]*binder.Node)
	walk = func(nodes []*binder.Node) {
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
			walk(n.Children)
		}
	}
	walk(parseResult.Root.Children)

	if refs == nil {
		refs = []string{}
	}

	return refs, diags
}
