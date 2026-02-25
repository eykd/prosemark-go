// Package binder provides domain types for the prosemark binder file format.
package binder

import "encoding/json"

// Node is a structural node in the binder tree.
// Root nodes have Type "root"; leaf/branch nodes have Type "node".
type Node struct {
	// JSON-exported fields (match parse-result.schema.json)
	Type     string  `json:"type"`             // "root" | "node"
	Target   string  `json:"target,omitempty"` // resolved relative path (absent on root)
	Title    string  `json:"title,omitempty"`  // display text (absent on root)
	Children []*Node `json:"children"`         // ordered children; never nil (use empty slice)

	// Source metadata (not serialized to JSON)
	Line        int    `json:"-"` // 1-based line number of list item
	Column      int    `json:"-"` // 1-based column of link start
	ByteOffset  int    `json:"-"` // byte offset from file start
	EndLine     int    `json:"-"` // last line of this item's content (before children)
	SubtreeEnd  int    `json:"-"` // last line of this item's subtree (inclusive)
	Indent      int    `json:"-"` // number of leading whitespace chars (spaces or tabs)
	IndentChar  byte   `json:"-"` // ' ' or '\t'
	ListMarker  string `json:"-"` // "-", "*", "+", "1.", "2.", "1)", etc.
	RawLine     string `json:"-"` // original source line bytes (excluding line ending)
	InCodeFence bool   `json:"-"` // true if inside a fenced code block (BNDW005)
}

// Diagnostic is a structured error or warning record emitted during parse or operations.
type Diagnostic struct {
	Severity string    `json:"severity"` // "error" | "warning"
	Code     string    `json:"code"`     // e.g. "BNDE001", "OPW002"
	Message  string    `json:"message"`
	Location *Location `json:"location,omitempty"` // nil if no source location
}

// Location identifies a source position within a binder file.
type Location struct {
	Line       int `json:"line"`       // 1-based
	Column     int `json:"column"`     // 1-based
	ByteOffset int `json:"byteOffset"` // 0-based byte offset from file start
}

// RefDef is a parsed reference link definition.
type RefDef struct {
	Label  string // normalized (lowercase) label
	Target string // URL/path
	Title  string // optional tooltip title
	Line   int    // 1-based line number of definition
}

// ParseResult is the structured output of parsing a binder file.
// Matches parse-result.schema.json when marshaled to JSON (diagnostics are merged at CLI layer).
type ParseResult struct {
	Version string `json:"version"` // always "1"
	Root    *Node  `json:"root"`

	// Source metadata (not in JSON schema)
	Lines      []string          `json:"-"` // original source lines (without endings)
	LineEnds   []string          `json:"-"` // line ending sequence per line: "\n", "\r\n", "\r"
	RefDefs    map[string]RefDef `json:"-"` // reference link definitions keyed by lowercase label
	HasBOM     bool              `json:"-"` // true if input had UTF-8 BOM
	HasPragma  bool              `json:"-"` // true if pragma line found
	PragmaLine int               `json:"-"` // 1-based line of pragma (0 if absent)
}

// Project holds the set of .md files in the project, populated by filesystem scanning.
type Project struct {
	Files     []string `json:"files"`     // relative paths to .md files in the project
	BinderDir string   `json:"binderDir"` // directory containing the binder file (enables proximity tiebreak)
}

// OpSpec is the parsed operation specification from op.json.
type OpSpec struct {
	Version   string          `json:"version"`   // "1"
	Operation string          `json:"operation"` // "add" | "delete" | "move"
	Params    json.RawMessage `json:"params"`    // decoded to specific *Params below
}

// AddChildParams are parameters for the add-child operation.
type AddChildParams struct {
	ParentSelector string `json:"parentSelector"`   // selector for the parent node
	Target         string `json:"target"`           // relative path of new child file
	Title          string `json:"title"`            // display title (empty = derive from stem)
	Position       string `json:"position"`         // "last" | "first" (default: "last")
	At             *int   `json:"at,omitempty"`     // zero-based index insertion point
	Before         string `json:"before,omitempty"` // selector of sibling to insert before
	After          string `json:"after,omitempty"`  // selector of sibling to insert after
	Force          bool   `json:"force"`            // allow duplicate target
}

// DeleteParams are parameters for the delete operation.
type DeleteParams struct {
	Selector string `json:"selector"` // selector for node(s) to delete
	Yes      bool   `json:"yes"`      // required confirmation flag
}

// MoveParams are parameters for the move operation.
type MoveParams struct {
	SourceSelector            string `json:"sourceSelector"`
	DestinationParentSelector string `json:"destinationParentSelector"`
	Position                  string `json:"position"` // "last" | "first"
	At                        *int   `json:"at,omitempty"`
	Before                    string `json:"before,omitempty"`
	After                     string `json:"after,omitempty"`
	Yes                       bool   `json:"yes"` // required confirmation flag
}

// OpResult is the CLI JSON output of any mutation operation.
// Matches op-result.schema.json.
type OpResult struct {
	Version     string       `json:"version"`     // "1"
	Changed     bool         `json:"changed"`     // true if binder bytes were modified
	Diagnostics []Diagnostic `json:"diagnostics"` // merged parse + op diagnostics
}

// SelectorResult holds the nodes matched by a selector evaluation.
type SelectorResult struct {
	Nodes    []*Node      // matched nodes (len 0 → OPE001; len > 1 → OPW001)
	Warnings []Diagnostic // OPW001 if multi-match
}

// Parse/lint errors (non-zero exit).
const (
	CodeIllegalPathChars  = "BNDE001"
	CodePathEscapesRoot   = "BNDE002"
	CodeAmbiguousWikilink = "BNDE003"
)

// Parse/lint warnings.
const (
	CodeMissingPragma        = "BNDW001"
	CodeMultipleStructLinks  = "BNDW002"
	CodeDuplicateFileRef     = "BNDW003"
	CodeMissingTargetFile    = "BNDW004"
	CodeLinkInCodeFence      = "BNDW005"
	CodeLinkOutsideList      = "BNDW006"
	CodeNonMarkdownTarget    = "BNDW007"
	CodeSelfReferentialLink  = "BNDW008"
	CodeCaseInsensitiveMatch = "BNDW009"
	CodeBOMPresence          = "BNDW010"
)

// Operation errors (non-zero exit; abort mutation).
const (
	CodeSelectorNoMatch   = "OPE001"
	CodeAmbiguousBareStem = "OPE002"
	CodeCycleDetected     = "OPE003"
	CodeInvalidTargetPath = "OPE004"
	CodeTargetIsBinder    = "OPE005"
	CodeNodeInCodeFence   = "OPE006"
	CodeSiblingNotFound   = "OPE007"
	CodeIndexOutOfBounds  = "OPE008"
	CodeIOOrParseFailure  = "OPE009"
	CodeConflictingFlags  = "OPE010"
)

// Operation warnings (exit 0; mutation proceeds).
const (
	CodeMultiMatch             = "OPW001"
	CodeDuplicateSkipped       = "OPW002"
	CodeNonStructuralDestroyed = "OPW003"
	CodeEmptySublistPruned     = "OPW004"
	CodeCascadeDelete          = "OPW005"
)
