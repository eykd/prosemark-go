package binder

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

var (
	pragmaRE       = regexp.MustCompile(`<!--\s*prosemark-binder:v1\s*-->`)
	linkRE         = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)
	listItemRE     = regexp.MustCompile(`^(\s*)([-*+])\s+(.+)`)
	inlineLinkRE   = regexp.MustCompile(`^\[([^\]]*)\]\(([^)\s"]+)(?:\s+"[^"]*")?\s*\)`)
	fullRefLinkRE  = regexp.MustCompile(`^\[([^\]]*)\]\[([^\]]+)\]`)
	collapsedRefRE = regexp.MustCompile(`^\[([^\]]*)\]\[\]`)
	wikilinkRE     = regexp.MustCompile(`^\[\[([^\]|]+)(?:\|([^\]]*))?\]\]`)
	shortcutRefRE  = regexp.MustCompile(`^\[([^\]]+)\]$`)
	refDefRE       = regexp.MustCompile(`^\[([^\]]+)\]:\s+(\S+)(?:\s+"([^"]*)")?`)
	mdInlineLinkRE = regexp.MustCompile(`\[[^\]]*\]\([^)]*\.md[^)]*\)`)
)

// wikilinkEntry holds a project file path and its directory depth (number of "/" separators).
type wikilinkEntry struct {
	file  string
	depth int
}

// Parse parses a binder file and returns a ParseResult, diagnostics, and any fatal error.
// project may be nil.
func Parse(ctx context.Context, src []byte, project *Project) (*ParseResult, []Diagnostic, error) {
	_ = ctx

	var diags []Diagnostic

	result := &ParseResult{
		Version: "1",
		Root: &Node{
			Type:     "root",
			Children: []*Node{},
		},
		RefDefs: make(map[string]RefDef),
	}

	// Strip UTF-8 BOM if present.
	if len(src) >= 3 && src[0] == 0xef && src[1] == 0xbb && src[2] == 0xbf {
		result.HasBOM = true
		src = src[3:]
		diags = append(diags, Diagnostic{
			Severity: "warning",
			Code:     CodeBOMPresence,
			Message:  "UTF-8 BOM detected",
		})
	}

	// Split into lines, recording endings per line.
	result.Lines, result.LineEnds = splitLines(src)

	// Pass 1: track fences, detect pragma, collect ref defs, warn on links in fences.
	inFence := false
	fenceMarker := ""

	for i, line := range result.Lines {
		lineNum := i + 1

		if !inFence {
			switch {
			case strings.HasPrefix(line, "```"):
				inFence = true
				fenceMarker = "```"
			case strings.HasPrefix(line, "~~~"):
				inFence = true
				fenceMarker = "~~~"
			default:
				if !result.HasPragma && pragmaRE.MatchString(line) {
					result.HasPragma = true
					result.PragmaLine = lineNum
				}
				if m := refDefRE.FindStringSubmatch(line); m != nil {
					label := strings.ToLower(m[1])
					result.RefDefs[label] = RefDef{
						Label:  label,
						Target: m[2],
						Title:  m[3],
						Line:   lineNum,
					}
				}
			}
		} else {
			if strings.HasPrefix(line, fenceMarker) {
				inFence = false
				fenceMarker = ""
			} else if linkRE.MatchString(line) {
				diags = append(diags, Diagnostic{
					Severity: "warning",
					Code:     CodeLinkInCodeFence,
					Message:  "link inside fenced code block will be ignored",
					Location: &Location{Line: lineNum},
				})
			}
		}
	}

	// Emit BNDW001 if no effective pragma found.
	if !result.HasPragma {
		diags = append(diags, Diagnostic{
			Severity: "warning",
			Code:     CodeMissingPragma,
			Message:  "prosemark-binder:v1 pragma not found",
		})
	}

	// Build O(1) wikilink basename index and project file set.
	wikiIndex := buildWikilinkIndex(project)
	projectFileSet := buildProjectFileSet(project)

	// Pass 2: scan list items, build node tree, emit structural diagnostics.
	type stackEntry struct {
		indent int
		node   *Node
	}
	stack := []stackEntry{{indent: -1, node: result.Root}}
	seenTargets := make(map[string]bool)

	inFence = false
	fenceMarker = ""

	for i, line := range result.Lines {
		lineNum := i + 1

		if !inFence {
			if strings.HasPrefix(line, "```") {
				inFence, fenceMarker = true, "```"
				continue
			}
			if strings.HasPrefix(line, "~~~") {
				inFence, fenceMarker = true, "~~~"
				continue
			}
		} else {
			if strings.HasPrefix(line, fenceMarker) {
				inFence, fenceMarker = false, ""
			}
			continue
		}

		m := listItemRE.FindStringSubmatch(line)
		if m == nil {
			// Not a list item: check for .md inline links outside lists (BNDW006).
			if mdInlineLinkRE.MatchString(line) {
				diags = append(diags, Diagnostic{
					Severity: "warning",
					Code:     CodeLinkOutsideList,
					Message:  "markdown link to .md file found outside list item",
					Location: &Location{Line: lineNum},
				})
			}
			continue
		}

		indent := len(m[1])
		marker := m[2]
		content := strings.TrimSpace(m[3])

		target, title, linkDiags := parseLink(content, result.RefDefs, project, wikiIndex, lineNum)

		// Emit link-resolution diagnostics (BNDE003, BNDW009).
		diags = append(diags, linkDiags...)

		// Skip items with no resolved target.
		if target == "" {
			continue
		}

		// Percent-decode the target before validation.
		decoded, decodeOK := percentDecodeTarget(target)
		if !decodeOK {
			diags = append(diags, Diagnostic{
				Severity: "error",
				Code:     CodeIllegalPathChars,
				Message:  "link target contains illegal path characters",
				Location: &Location{Line: lineNum},
			})
			continue
		}
		target = decoded

		// Validate target path.
		if diag := validateTarget(target, lineNum); diag != nil {
			diags = append(diags, *diag)
			continue
		}

		// Check for multiple structural .md links in one list item (BNDW002).
		if allMd := mdInlineLinkRE.FindAllString(content, -1); len(allMd) > 1 {
			diags = append(diags, Diagnostic{
				Severity: "warning",
				Code:     CodeMultipleStructLinks,
				Message:  "list item contains multiple structural links; only the first is used",
				Location: &Location{Line: lineNum},
			})
		}

		// Check for self-referential link (BNDW008).
		if target == "_binder.md" {
			diags = append(diags, Diagnostic{
				Severity: "warning",
				Code:     CodeSelfReferentialLink,
				Message:  "link targets the binder file itself",
				Location: &Location{Line: lineNum},
			})
		}

		// Check for duplicate file reference (BNDW003).
		if seenTargets[target] {
			diags = append(diags, Diagnostic{
				Severity: "warning",
				Code:     CodeDuplicateFileRef,
				Message:  "file referenced more than once in binder",
				Location: &Location{Line: lineNum},
			})
		}
		seenTargets[target] = true

		// Check for missing target file when project context is available (BNDW004).
		if project != nil && !projectFileSet[target] {
			diags = append(diags, Diagnostic{
				Severity: "warning",
				Code:     CodeMissingTargetFile,
				Message:  "link target not found in project files",
				Location: &Location{Line: lineNum},
			})
		}

		node := &Node{
			Type:       "node",
			Children:   []*Node{},
			Target:     target,
			Title:      title,
			Line:       lineNum,
			Indent:     indent,
			ListMarker: marker,
			RawLine:    line,
		}

		// Find parent using indent stack.
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1].node
		parent.Children = append(parent.Children, node)
		stack = append(stack, stackEntry{indent: indent, node: node})
	}

	return result, diags, nil
}

// parseLink parses the content portion of a list item and returns (target, title, diags).
// Returns ("", "", nil) if no link can be resolved.
func parseLink(content string, refDefs map[string]RefDef, project *Project, wikiIndex map[string][]wikilinkEntry, lineNum int) (target, title string, diags []Diagnostic) {
	if m := inlineLinkRE.FindStringSubmatch(content); m != nil {
		target, title = m[2], m[1]
		if title == "" {
			title = stemFromPath(target)
		}
	} else if m := wikilinkRE.FindStringSubmatch(content); m != nil {
		if project != nil {
			target, title, diags = resolveWikilink(m[1], m[2], wikiIndex, lineNum)
		}
	} else if m := fullRefLinkRE.FindStringSubmatch(content); m != nil {
		if rd, exists := refDefs[strings.ToLower(m[2])]; exists {
			target, title = rd.Target, m[1]
		}
	} else if m := collapsedRefRE.FindStringSubmatch(content); m != nil {
		if rd, exists := refDefs[strings.ToLower(m[1])]; exists {
			target, title = rd.Target, m[1]
		}
	} else if m := shortcutRefRE.FindStringSubmatch(content); m != nil {
		if rd, exists := refDefs[strings.ToLower(m[1])]; exists {
			target, title = rd.Target, m[1]
		}
	}
	return
}

// resolveWikilink resolves a wikilink stem and alias using the pre-built index.
// Caller must ensure project is non-nil.
func resolveWikilink(stem, alias string, wikiIndex map[string][]wikilinkEntry, lineNum int) (target, title string, diags []Diagnostic) {
	stemFile := stem + ".md"
	entries := wikiIndex[strings.ToLower(stem)]

	// Case-sensitive: exact path match OR basename match.
	var csEntries []wikilinkEntry
	for _, e := range entries {
		if e.file == stemFile || baseName(e.file) == stemFile {
			csEntries = append(csEntries, e)
		}
	}

	caseInsensitive := false
	matchEntries := csEntries
	if len(csEntries) == 0 {
		matchEntries = entries
		caseInsensitive = true
	}

	// Proximity tiebreak: prefer shallowest path (fewest "/" separators).
	minDepth := -1
	for _, e := range matchEntries {
		if minDepth == -1 || e.depth < minDepth {
			minDepth = e.depth
		}
	}

	var atMinDepth []wikilinkEntry
	for _, e := range matchEntries {
		if e.depth == minDepth {
			atMinDepth = append(atMinDepth, e)
		}
	}

	switch len(atMinDepth) {
	case 1:
		target = atMinDepth[0].file
		if caseInsensitive {
			diags = append(diags, Diagnostic{
				Severity: "warning",
				Code:     CodeCaseInsensitiveMatch,
				Message:  "wikilink resolved by case-insensitive match",
				Location: &Location{Line: lineNum},
			})
		}
		if alias == "" {
			alias = stemFromPath(target)
		}
		title = alias
	default:
		if len(atMinDepth) > 1 {
			diags = append(diags, Diagnostic{
				Severity: "error",
				Code:     CodeAmbiguousWikilink,
				Message:  "wikilink matches multiple files at the same depth",
				Location: &Location{Line: lineNum},
			})
		}
	}
	return
}

// buildWikilinkIndex builds a lowercase-stem → []wikilinkEntry map for O(1) lookup.
// Each file is indexed by its basename stem; files in subdirectories are also indexed
// by their full path stem so that [[subdir/file]] wikilinks resolve via the index too.
func buildWikilinkIndex(project *Project) map[string][]wikilinkEntry {
	index := make(map[string][]wikilinkEntry)
	if project == nil {
		return index
	}
	for _, f := range project.Files {
		depth := strings.Count(f, "/")
		e := wikilinkEntry{file: f, depth: depth}
		lowBase := strings.ToLower(strings.TrimSuffix(baseName(f), ".md"))
		index[lowBase] = append(index[lowBase], e)
		if depth > 0 {
			lowPath := strings.ToLower(strings.TrimSuffix(f, ".md"))
			index[lowPath] = append(index[lowPath], e)
		}
	}
	return index
}

// buildProjectFileSet builds a set of project file paths for O(1) membership checks.
func buildProjectFileSet(project *Project) map[string]bool {
	set := make(map[string]bool)
	if project == nil {
		return set
	}
	for _, f := range project.Files {
		set[f] = true
	}
	return set
}

// percentDecodeTarget URL-decodes a percent-encoded path target.
// Returns (decoded, true) on success, ("", false) if the encoding is malformed.
func percentDecodeTarget(target string) (string, bool) {
	decoded, err := url.PathUnescape(target)
	if err != nil {
		return "", false
	}
	return decoded, true
}

// baseName returns the final slash-separated component of a path.
func baseName(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// validateTarget returns a Diagnostic if target fails path validation, or nil if valid.
// Checks are applied in order: illegal chars, root escape, non-markdown extension.
func validateTarget(target string, lineNum int) *Diagnostic {
	switch {
	case hasIllegalPathChars(target):
		return &Diagnostic{
			Severity: "error",
			Code:     CodeIllegalPathChars,
			Message:  "link target contains illegal path characters",
			Location: &Location{Line: lineNum},
		}
	case escapesRoot(target):
		return &Diagnostic{
			Severity: "error",
			Code:     CodePathEscapesRoot,
			Message:  "link target escapes project root",
			Location: &Location{Line: lineNum},
		}
	case !isMarkdownTarget(target):
		return &Diagnostic{
			Severity: "warning",
			Code:     CodeNonMarkdownTarget,
			Message:  "link target is not a .md file",
			Location: &Location{Line: lineNum},
		}
	}
	return nil
}

// stemFromPath returns the filename stem (basename without extension).
func stemFromPath(p string) string {
	if idx := strings.LastIndex(p, "/"); idx >= 0 {
		p = p[idx+1:]
	}
	return strings.SplitN(p, ".", 2)[0]
}

// hasIllegalPathChars reports whether path contains illegal file path characters.
func hasIllegalPathChars(path string) bool {
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c < 0x20 || c == '>' || c == '<' || c == '|' || c == '?' || c == '*' || c == ':' {
			return true
		}
	}
	return false
}

// escapesRoot reports whether path escapes the project root via "..".
func escapesRoot(path string) bool {
	return path == ".." || strings.HasPrefix(path, "../")
}

// isMarkdownTarget reports whether path has a .md extension.
func isMarkdownTarget(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".md")
}

// splitLines splits src into lines and their corresponding line endings.
// Lines do not include the ending characters.
// A trailing newline does not produce an extra empty line.
func splitLines(src []byte) ([]string, []string) {
	if len(src) == 0 {
		return []string{}, []string{}
	}

	var lines []string
	var ends []string
	start := 0

	for i := 0; i < len(src); {
		switch src[i] {
		case '\n':
			lines = append(lines, string(src[start:i]))
			ends = append(ends, "\n")
			i++
			start = i
		case '\r':
			end := "\r"
			advance := 1
			if i+1 < len(src) && src[i+1] == '\n' {
				end = "\r\n"
				advance = 2
			}
			lines = append(lines, string(src[start:i]))
			ends = append(ends, end)
			i += advance
			start = i
		default:
			i++
		}
	}

	// Handle remaining content (file does not end with a newline).
	if start < len(src) {
		lines = append(lines, string(src[start:]))
		ends = append(ends, "")
	}

	return lines, ends
}
