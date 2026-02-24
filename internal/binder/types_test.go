package binder_test

import (
	"encoding/json"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestNodeType verifies Node struct has all required fields with correct types.
func TestNodeType(t *testing.T) {
	n := binder.Node{
		Type:        "root",
		Target:      "docs/index.md",
		Title:       "Index",
		Children:    []*binder.Node{},
		Line:        1,
		Column:      3,
		ByteOffset:  0,
		EndLine:     5,
		SubtreeEnd:  10,
		Indent:      2,
		IndentChar:  ' ',
		ListMarker:  "-",
		RawLine:     "  - [Index](docs/index.md)",
		InCodeFence: false,
	}
	if n.Type != "root" {
		t.Errorf("Node.Type = %q, want %q", n.Type, "root")
	}
	if n.Children == nil {
		t.Error("Node.Children must be non-nil (empty slice, not nil)")
	}
}

// TestNodeChildrenNeverNil documents that Children must default to empty slice.
func TestNodeChildrenNeverNil(t *testing.T) {
	n := binder.Node{Type: "root", Children: []*binder.Node{}}
	if n.Children == nil {
		t.Error("Node.Children must be initialised to non-nil empty slice")
	}
}

// TestDiagnosticType verifies Diagnostic struct fields.
func TestDiagnosticType(t *testing.T) {
	loc := &binder.Location{
		Line:       5,
		Column:     3,
		ByteOffset: 42,
	}
	d := binder.Diagnostic{
		Severity: "error",
		Code:     binder.CodeIllegalPathChars,
		Message:  "illegal path character",
		Location: loc,
	}
	if d.Severity != "error" {
		t.Errorf("Diagnostic.Severity = %q, want %q", d.Severity, "error")
	}
	if d.Code != "BNDE001" {
		t.Errorf("Diagnostic.Code = %q, want %q", d.Code, "BNDE001")
	}
	if d.Location == nil {
		t.Error("Diagnostic.Location must be set")
	}
	if d.Location.Line != 5 {
		t.Errorf("Location.Line = %d, want %d", d.Location.Line, 5)
	}
}

// TestLocationType verifies Location struct fields.
func TestLocationType(t *testing.T) {
	loc := binder.Location{Line: 1, Column: 1, ByteOffset: 0}
	if loc.Line != 1 {
		t.Errorf("Location.Line = %d, want 1", loc.Line)
	}
}

// TestRefDefType verifies RefDef struct fields.
func TestRefDefType(t *testing.T) {
	r := binder.RefDef{
		Label:  "my-link",
		Target: "docs/chapter.md",
		Title:  "Chapter One",
		Line:   12,
	}
	if r.Label == "" {
		t.Error("RefDef.Label must not be empty")
	}
}

// TestParseResultType verifies ParseResult struct fields.
func TestParseResultType(t *testing.T) {
	pr := binder.ParseResult{
		Version: "1",
		Root: &binder.Node{
			Type:     "root",
			Children: []*binder.Node{},
		},
		Lines:      []string{"line one", "line two"},
		LineEnds:   []string{"\n", "\n"},
		RefDefs:    map[string]binder.RefDef{},
		HasBOM:     false,
		HasPragma:  true,
		PragmaLine: 1,
	}
	if pr.Version != "1" {
		t.Errorf("ParseResult.Version = %q, want %q", pr.Version, "1")
	}
	if pr.Root == nil {
		t.Error("ParseResult.Root must not be nil")
	}
}

// TestProjectType verifies Project struct fields.
func TestProjectType(t *testing.T) {
	p := binder.Project{
		Files: []string{"docs/index.md", "docs/chapter.md"},
	}
	if len(p.Files) != 2 {
		t.Errorf("Project.Files len = %d, want 2", len(p.Files))
	}
}

// TestOpSpecType verifies OpSpec struct fields.
func TestOpSpecType(t *testing.T) {
	raw := json.RawMessage(`{"parentSelector":".","target":"docs/new.md"}`)
	op := binder.OpSpec{
		Version:   "1",
		Operation: "add",
		Params:    raw,
	}
	if op.Operation != "add" {
		t.Errorf("OpSpec.Operation = %q, want %q", op.Operation, "add")
	}
}

// TestAddChildParamsType verifies AddChildParams struct fields.
func TestAddChildParamsType(t *testing.T) {
	idx := 2
	p := binder.AddChildParams{
		ParentSelector: ".",
		Target:         "docs/new.md",
		Title:          "New Doc",
		Position:       "last",
		At:             &idx,
		Before:         "docs/other.md",
		After:          "",
		Force:          false,
	}
	if p.ParentSelector != "." {
		t.Errorf("AddChildParams.ParentSelector = %q, want %q", p.ParentSelector, ".")
	}
	if p.At == nil {
		t.Error("AddChildParams.At must not be nil when set")
	}
}

// TestDeleteParamsType verifies DeleteParams struct fields.
func TestDeleteParamsType(t *testing.T) {
	p := binder.DeleteParams{
		Selector: "docs/chapter.md",
		Yes:      true,
	}
	if !p.Yes {
		t.Error("DeleteParams.Yes must be true when set")
	}
}

// TestMoveParamsType verifies MoveParams struct fields.
func TestMoveParamsType(t *testing.T) {
	idx := 0
	p := binder.MoveParams{
		SourceSelector:            "docs/chapter.md",
		DestinationParentSelector: ".",
		Position:                  "first",
		At:                        &idx,
		Before:                    "",
		After:                     "",
		Yes:                       true,
	}
	if p.SourceSelector == "" {
		t.Error("MoveParams.SourceSelector must not be empty")
	}
}

// TestOpResultType verifies OpResult struct fields.
func TestOpResultType(t *testing.T) {
	r := binder.OpResult{
		Version: "1",
		Changed: true,
		Diagnostics: []binder.Diagnostic{
			{
				Severity: "warning",
				Code:     binder.CodeMissingPragma,
				Message:  "missing pragma",
			},
		},
	}
	if r.Version != "1" {
		t.Errorf("OpResult.Version = %q, want %q", r.Version, "1")
	}
	if !r.Changed {
		t.Error("OpResult.Changed must be true when set")
	}
}

// TestSelectorResultType verifies SelectorResult struct fields.
func TestSelectorResultType(t *testing.T) {
	sr := binder.SelectorResult{
		Nodes:    []*binder.Node{},
		Warnings: []binder.Diagnostic{},
	}
	if sr.Nodes == nil {
		t.Error("SelectorResult.Nodes must not be nil")
	}
}

// TestDiagnosticCodeConstants verifies all BNDE error code constants.
func TestDiagnosticCodeConstants_BNDE(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{binder.CodeIllegalPathChars, "BNDE001"},
		{binder.CodePathEscapesRoot, "BNDE002"},
		{binder.CodeAmbiguousWikilink, "BNDE003"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("constant = %q, want %q", tt.got, tt.want)
		}
	}
}

// TestDiagnosticCodeConstants_BNDW verifies all BNDW warning code constants.
func TestDiagnosticCodeConstants_BNDW(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{binder.CodeMissingPragma, "BNDW001"},
		{binder.CodeMultipleStructLinks, "BNDW002"},
		{binder.CodeDuplicateFileRef, "BNDW003"},
		{binder.CodeMissingTargetFile, "BNDW004"},
		{binder.CodeLinkInCodeFence, "BNDW005"},
		{binder.CodeLinkOutsideList, "BNDW006"},
		{binder.CodeNonMarkdownTarget, "BNDW007"},
		{binder.CodeSelfReferentialLink, "BNDW008"},
		{binder.CodeCaseInsensitiveMatch, "BNDW009"},
		{binder.CodeBOMPresence, "BNDW010"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("constant = %q, want %q", tt.got, tt.want)
		}
	}
}

// TestDiagnosticCodeConstants_OPE verifies all OPE operation error code constants.
func TestDiagnosticCodeConstants_OPE(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{binder.CodeSelectorNoMatch, "OPE001"},
		{binder.CodeAmbiguousBareStem, "OPE002"},
		{binder.CodeCycleDetected, "OPE003"},
		{binder.CodeInvalidTargetPath, "OPE004"},
		{binder.CodeTargetIsBinder, "OPE005"},
		{binder.CodeNodeInCodeFence, "OPE006"},
		{binder.CodeSiblingNotFound, "OPE007"},
		{binder.CodeIndexOutOfBounds, "OPE008"},
		{binder.CodeIOOrParseFailure, "OPE009"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("constant = %q, want %q", tt.got, tt.want)
		}
	}
}

// TestDiagnosticCodeConstants_OPW verifies all OPW operation warning code constants.
func TestDiagnosticCodeConstants_OPW(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{binder.CodeMultiMatch, "OPW001"},
		{binder.CodeDuplicateSkipped, "OPW002"},
		{binder.CodeNonStructuralDestroyed, "OPW003"},
		{binder.CodeEmptySublistPruned, "OPW004"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("constant = %q, want %q", tt.got, tt.want)
		}
	}
}

// TestNodeJSONSerialization verifies Node JSON output matches expected schema fields.
func TestNodeJSONSerialization(t *testing.T) {
	child := &binder.Node{
		Type:     "node",
		Target:   "docs/child.md",
		Title:    "Child",
		Children: []*binder.Node{},
	}
	root := binder.Node{
		Type:     "root",
		Children: []*binder.Node{child},
	}

	data, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("json.Marshal(Node) error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	// Root type must be serialized
	if out["type"] != "root" {
		t.Errorf("JSON type = %v, want %q", out["type"], "root")
	}
	// target and title must be absent on root (omitempty)
	if _, ok := out["target"]; ok {
		t.Error("JSON must omit 'target' when empty (root node)")
	}
	if _, ok := out["title"]; ok {
		t.Error("JSON must omit 'title' when empty (root node)")
	}
	// children must always be present
	if _, ok := out["children"]; !ok {
		t.Error("JSON must always include 'children'")
	}
	// source metadata fields must NOT appear in JSON
	for _, field := range []string{"line", "column", "byteOffset", "endLine", "subtreeEnd",
		"indent", "indentChar", "listMarker", "rawLine", "inCodeFence"} {
		if _, ok := out[field]; ok {
			t.Errorf("JSON must not serialize source metadata field %q", field)
		}
	}
}

// TestParseResultJSONSerializationExcludesMetadata verifies ParseResult's metadata
// fields are excluded from JSON output.
func TestParseResultJSONSerializationExcludesMetadata(t *testing.T) {
	pr := binder.ParseResult{
		Version:  "1",
		Root:     &binder.Node{Type: "root", Children: []*binder.Node{}},
		Lines:    []string{"hello"},
		LineEnds: []string{"\n"},
		RefDefs:  map[string]binder.RefDef{"key": {Label: "key", Target: "x.md"}},
		HasBOM:   true,
	}

	data, err := json.Marshal(pr)
	if err != nil {
		t.Fatalf("json.Marshal(ParseResult) error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, field := range []string{"lines", "lineEnds", "refDefs", "hasBOM", "hasPragma", "pragmaLine"} {
		if _, ok := out[field]; ok {
			t.Errorf("ParseResult JSON must not include metadata field %q", field)
		}
	}

	if out["version"] != "1" {
		t.Errorf("ParseResult JSON version = %v, want %q", out["version"], "1")
	}
}
