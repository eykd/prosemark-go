package binder

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	pragmaRE        = regexp.MustCompile(`<\\?!--\s*prosemark-binder:v1\s*-->`)
	linkRE          = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)
	listItemRE      = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+(.+)`)
	inlineLinkRE    = regexp.MustCompile(`^\[([^\]]*)\]\(([^)\s"]+)(?:\s+"[^"]*")?\s*\)`)
	fullRefLinkRE   = regexp.MustCompile(`^\[([^\]]*)\]\[([^\]]+)\]`)
	collapsedRefRE  = regexp.MustCompile(`^\[([^\]]*)\]\[\]`)
	wikilinkRE      = regexp.MustCompile(`^!?\[\[([^\]|]+)(?:\|([^\]]*))?\]\]`)
	shortcutRefRE   = regexp.MustCompile(`^\[([^\]]+)\]$`)
	refDefRE        = regexp.MustCompile(`^\[([^\]]+)\]:\s+(\S+)(?:\s+"([^"]*)")?`)
	mdInlineLinkRE  = regexp.MustCompile(`\[[^\]]*\]\([^)]*\.md[^)]*\)`)
	checkboxRE      = regexp.MustCompile(`^\[[xX ]\]\s+`)
	strikethroughRE = regexp.MustCompile(`~~[^~]*~~`)
	// allInlineLinkRE finds all inline links anywhere in content.
	allInlineLinkRE = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s"]+)(?:\s+"[^"]*")?\s*\)`)
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
	}

	// Reject non-UTF-8 content before any processing.
	if !utf8.Valid(src) {
		return result, nil, fmt.Errorf("binder file contains invalid UTF-8 content")
	}

	// Strip UTF-8 BOM if present.
	if bytes.HasPrefix(src, []byte(utf8BOM)) {
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
	p1 := pass1Scan(result.Lines)
	result.HasPragma = p1.hasPragma
	result.PragmaLine = p1.pragmaLine
	result.RefDefs = p1.refDefs
	diags = append(diags, p1.diags...)

	// Emit BNDW001 if no effective pragma found and file has content (or no project context).
	if !result.HasPragma && (project == nil || len(result.Lines) > 0) {
		diags = append(diags, Diagnostic{
			Severity: "warning",
			Code:     CodeMissingPragma,
			Message:  "Missing binder pragma: file has content but does not begin with <!-- prosemark-binder:v1 -->",
		})
	}

	// Build O(1) wikilink basename index and project file set.
	wikiIndex := buildWikilinkIndex(project)
	projectFileSet := buildProjectFileSet(project)
	projectFilesLower := buildProjectFilesLower(project)

	binderDir := ""
	if project != nil {
		binderDir = project.BinderDir
	}

	// Pass 2: scan list items, build node tree, emit structural diagnostics.
	type stackEntry struct {
		indent int
		node   *Node
	}
	stack := []stackEntry{{indent: -1, node: result.Root}}
	seenTargets := make(map[string]bool)

	inFence := false
	fenceMarker := ""

	// consumed tracks lines already processed as list item continuations.
	consumed := make(map[int]bool)

	for i, line := range result.Lines {
		lineNum := i + 1

		if consumed[i] {
			continue
		}

		if !inFence {
			if marker := openFenceMarker(line); marker != "" {
				inFence, fenceMarker = true, marker
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
		rawContent := m[3]
		content := strings.TrimSpace(rawContent)

		// Column of the content start within the line (1-based).
		listItemColumn := len(m[0]) - len(rawContent) + 1

		content = normalizeListContent(content)

		target, title, linkDiags := parseLink(content, result.RefDefs, wikiIndex, binderDir, lineNum, listItemColumn)

		// If no link found in content, check the immediately following continuation line.
		if target == "" && i+1 < len(result.Lines) {
			nextLine := result.Lines[i+1]
			// A continuation line has more indentation than the list marker level.
			if countLeadingWhitespace(nextLine) > indent && !listItemRE.MatchString(nextLine) {
				contContent := normalizeListContent(strings.TrimSpace(nextLine))
				t, ti, ld := parseLink(contContent, result.RefDefs, wikiIndex, binderDir, i+2, 0)
				consumed[i+1] = true
				if t != "" {
					target, title = t, ti
					linkDiags = ld
				}
			}
		}

		// Emit link-resolution diagnostics (BNDE003, BNDW009).
		diags = append(diags, linkDiags...)

		// Skip items with no resolved target.
		if target == "" {
			continue
		}

		// Handle non-md first link: if target is non-md, look for an md link elsewhere.
		if !isMarkdownTarget(target) && !hasIllegalPathChars(target) && !escapesRoot(target) {
			// Emit BNDW007 for this non-md link and try to find an md link in content.
			diags = append(diags, Diagnostic{
				Severity: "warning",
				Code:     CodeNonMarkdownTarget,
				Message:  "Link target is not a .md file",
				Location: &Location{Line: lineNum},
			})
			// Search for an md link elsewhere in the content.
			mdTarget, mdTitle := findFirstMdLink(content)
			if mdTarget == "" {
				continue
			}
			target, title = mdTarget, mdTitle
		}

		// Percent-decode the target before validation.
		decoded, decodeOK := percentDecodeTarget(target)
		if !decodeOK {
			diags = append(diags, Diagnostic{
				Severity: "error",
				Code:     CodeIllegalPathChars,
				Message:  fmt.Sprintf("Illegal path characters in link target: %s", target),
				Location: &Location{Line: lineNum, Column: listItemColumn},
			})
			continue
		}
		target = decoded

		// Validate target path.
		if diag := validateTarget(target, lineNum, listItemColumn); diag != nil {
			diags = append(diags, *diag)
			continue
		}

		// Check for self-referential link (BNDW008).
		if target == "_binder.md" {
			diags = append(diags, Diagnostic{
				Severity: "warning",
				Code:     CodeSelfReferentialLink,
				Message:  "link targets the binder file itself",
				Location: &Location{Line: lineNum},
			})
			continue // skip node creation for self-referential links
		}

		// Check for duplicate file reference (BNDW003).
		if seenTargets[target] {
			diags = append(diags, Diagnostic{
				Severity: "warning",
				Code:     CodeDuplicateFileRef,
				Message:  fmt.Sprintf("Duplicate file reference: %s appears as more than one node in the binder tree", target),
				Location: &Location{Line: lineNum},
			})
		}
		seenTargets[target] = true

		// Check for missing/case-mismatch target file when project context is available.
		if project != nil && !projectFileSet[target] {
			// Check for case-insensitive match (BNDW009).
			if lowerMatch := projectFilesLower[strings.ToLower(target)]; lowerMatch != "" {
				diags = append(diags, Diagnostic{
					Severity: "warning",
					Code:     CodeCaseInsensitiveMatch,
					Message:  fmt.Sprintf("case-insensitive match found: %s → %s", target, lowerMatch),
					Location: &Location{Line: lineNum},
				})
			} else {
				diags = append(diags, Diagnostic{
					Severity: "warning",
					Code:     CodeMissingTargetFile,
					Message:  fmt.Sprintf("Target file %s is not present in the project", target),
					Location: &Location{Line: lineNum},
				})
			}
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

// pass1Data holds the results of the first-pass scan over source lines.
type pass1Data struct {
	hasPragma  bool
	pragmaLine int
	refDefs    map[string]RefDef
	diags      []Diagnostic // fence-link diagnostics only
}

// pass1Scan scans lines to detect the pragma, collect reference definitions, and warn
// on structural links inside fenced code blocks.
func pass1Scan(lines []string) pass1Data {
	result := pass1Data{refDefs: make(map[string]RefDef)}
	inFence := false
	fenceMarker := ""

	for i, line := range lines {
		lineNum := i + 1
		if !inFence {
			if marker := openFenceMarker(line); marker != "" {
				inFence, fenceMarker = true, marker
			} else {
				if !result.hasPragma && pragmaRE.MatchString(line) {
					result.hasPragma = true
					result.pragmaLine = lineNum
				}
				if m := refDefRE.FindStringSubmatch(line); m != nil {
					label := strings.ToLower(m[1])
					result.refDefs[label] = RefDef{
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
				result.diags = append(result.diags, Diagnostic{
					Severity: "warning",
					Code:     CodeLinkInCodeFence,
					Message:  "Structural link found inside a fenced code block",
					Location: &Location{Line: lineNum},
				})
			}
		}
	}
	return result
}

// parseLink parses the content portion of a list item and returns (target, title, diags).
// Returns ("", "", nil) if no link can be resolved.
func parseLink(content string, refDefs map[string]RefDef, wikiIndex map[string][]wikilinkEntry, binderDir string, lineNum, column int) (target, title string, diags []Diagnostic) {
	if m := inlineLinkRE.FindStringSubmatch(content); m != nil {
		target, title = m[2], m[1]
		if title == "" {
			title = stemFromPath(target)
		}
	} else if m := wikilinkRE.FindStringSubmatch(content); m != nil {
		rawStem := m[1]
		alias := m[2]
		target, title, diags = resolveWikilink(rawStem, alias, wikiIndex, binderDir, lineNum, column)
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

// findFirstMdLink scans content for the first inline .md link after a non-md link.
// Returns ("", "") if none found.
func findFirstMdLink(content string) (target, title string) {
	for _, m := range allInlineLinkRE.FindAllStringSubmatch(content, -1) {
		if isMarkdownTarget(m[2]) {
			target, title = m[2], m[1]
			return
		}
	}
	return
}

// resolveWikilink resolves a wikilink stem and alias using the pre-built index.
// Caller must ensure project is non-nil.
func resolveWikilink(rawStem, alias string, wikiIndex map[string][]wikilinkEntry, binderDir string, lineNum, column int) (target, title string, diags []Diagnostic) {
	// Strip fragment from stem (e.g., "foo#section" → "foo").
	stem := rawStem
	if idx := strings.Index(stem, "#"); idx >= 0 {
		stem = stem[:idx]
	}

	// Fragment-only wikilink [[#heading]] → illegal (BNDE001).
	if stem == "" {
		diags = append(diags, Diagnostic{
			Severity: "error",
			Code:     CodeIllegalPathChars,
			Message:  fmt.Sprintf("Illegal path characters in link target: #%s", strings.TrimPrefix(rawStem, "#")),
			Location: &Location{Line: lineNum},
		})
		return
	}

	stemFile := stem + ".md"
	entries := wikiIndex[strings.ToLower(stem)]

	// Case-sensitive: exact path match OR basename match.
	var exactEntries, basenameEntries []wikilinkEntry
	for _, e := range entries {
		if e.file == stemFile {
			exactEntries = append(exactEntries, e)
		} else if baseName(e.file) == stemFile {
			basenameEntries = append(basenameEntries, e)
		}
	}
	csEntries := append(exactEntries, basenameEntries...) //nolint:gocritic

	caseInsensitive := false
	matchEntries := csEntries
	if len(csEntries) == 0 {
		matchEntries = entries
		caseInsensitive = true
	}

	// Ambiguity check: when an exact-path match coexists with subdirectory basename matches
	// and no binderDir context is available, the wikilink is ambiguous (BNDE003).
	if len(exactEntries) > 0 && len(basenameEntries) > 0 && binderDir == "" {
		diags = append(diags, Diagnostic{
			Severity: "error",
			Code:     CodeAmbiguousWikilink,
			Message:  fmt.Sprintf("Ambiguous wikilink: [[%s]] matches %s", rawStem, joinWikilinkFiles(csEntries)),
			Location: &Location{Line: lineNum, Column: column},
		})
		return
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

	switch {
	case len(atMinDepth) == 1:
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
	case len(atMinDepth) > 1:
		// Multiple at same depth even after proximity tiebreak.
		diags = append(diags, Diagnostic{
			Severity: "error",
			Code:     CodeAmbiguousWikilink,
			Message:  fmt.Sprintf("Ambiguous wikilink: [[%s]] matches %s", rawStem, joinWikilinkFiles(atMinDepth)),
			Location: &Location{Line: lineNum, Column: column},
		})
	default:
		// Zero matches: create node with derived filename (BNDW004 emitted by caller).
		target = stemFile
		if alias == "" {
			alias = stem
		}
		title = alias
	}
	return
}

// joinWikilinkFiles formats a list of wikilink entries as "file1 and file2".
func joinWikilinkFiles(entries []wikilinkEntry) string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.file
	}
	return strings.Join(names, " and ")
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

// buildProjectFilesLower builds a lowercase→original map for case-insensitive matching.
func buildProjectFilesLower(project *Project) map[string]string {
	m := make(map[string]string)
	if project == nil {
		return m
	}
	for _, f := range project.Files {
		m[strings.ToLower(f)] = f
	}
	return m
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
// Checks are applied in order: illegal chars, trailing-dot segment, root escape.
// Non-markdown targets are handled before this function is called.
func validateTarget(target string, lineNum, column int) *Diagnostic {
	switch {
	case hasIllegalPathChars(target):
		return &Diagnostic{
			Severity: "error",
			Code:     CodeIllegalPathChars,
			Message:  fmt.Sprintf("Illegal path characters in link target: %s", target),
			Location: &Location{Line: lineNum, Column: column},
		}
	case hasTrailingDotSegment(target):
		return &Diagnostic{
			Severity: "error",
			Code:     CodeIllegalPathChars,
			Message:  fmt.Sprintf("Illegal path characters in link target: %s", target),
			Location: &Location{Line: lineNum, Column: column},
		}
	case escapesRoot(target):
		return &Diagnostic{
			Severity: "error",
			Code:     CodePathEscapesRoot,
			Message:  "Link target resolves outside the project root",
			Location: &Location{Line: lineNum, Column: column},
		}
	}
	return nil
}

// stemFromPath returns the filename stem (basename without last extension).
func stemFromPath(p string) string {
	if idx := strings.LastIndex(p, "/"); idx >= 0 {
		p = p[idx+1:]
	}
	if idx := strings.LastIndex(p, "."); idx >= 0 {
		p = p[:idx]
	}
	return p
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

// hasTrailingDotSegment reports whether any path segment (other than "." and "..") ends with ".".
func hasTrailingDotSegment(path string) bool {
	for _, seg := range strings.Split(path, "/") {
		if len(seg) > 1 && strings.HasSuffix(seg, ".") && seg != ".." {
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

// normalizeListContent strips GFM task-list checkbox prefixes and strikethrough markup
// from list item content, trimming surrounding whitespace.
func normalizeListContent(s string) string {
	s = checkboxRE.ReplaceAllString(s, "")
	return strings.TrimSpace(strikethroughRE.ReplaceAllString(s, ""))
}

// openFenceMarker returns the fence marker string ("```" or "~~~") if line opens a
// fenced code block, or "" if the line is not a fence opener.
func openFenceMarker(line string) string {
	if strings.HasPrefix(line, "```") {
		return "```"
	}
	if strings.HasPrefix(line, "~~~") {
		return "~~~"
	}
	return ""
}

// countLeadingWhitespace returns the number of leading space/tab characters.
func countLeadingWhitespace(s string) int {
	n := 0
	for _, c := range s {
		if c != ' ' && c != '\t' {
			break
		}
		n++
	}
	return n
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
