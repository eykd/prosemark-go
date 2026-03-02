// Package node defines core domain types for prosemark node identity.
package node

// NodeId is a type alias for string representing a node's unique identifier (UUID v7).
type NodeId = string

// Frontmatter holds the YAML front matter for a prosemark node file.
type Frontmatter struct {
	// ID is the node's unique identifier (UUID v7).
	ID string `yaml:"id"`
	// Title is the optional human-readable title for the node.
	Title string `yaml:"title,omitempty"`
	// Synopsis is the optional brief summary of the node's content.
	Synopsis string `yaml:"synopsis,omitempty"`
	// Created is the RFC3339 timestamp when the node was first created.
	Created string `yaml:"created"`
	// Updated is the RFC3339 timestamp when the node was last modified.
	Updated string `yaml:"updated"`
}

// NodePart identifies which file part of a node is being referenced.
type NodePart string

const (
	// NodePartDraft refers to the main draft content file ({uuid}.md).
	NodePartDraft NodePart = "draft"
	// NodePartNotes refers to the companion notes file ({uuid}.notes.md).
	NodePartNotes NodePart = "notes"
)

// AuditCode identifies a specific audit rule that was evaluated.
type AuditCode string

const (
	// AUD001 indicates a referenced file does not exist on disk.
	AUD001 AuditCode = "AUD001"
	// AUD002 indicates a UUID-pattern file exists in project root but is not referenced in the binder (orphaned node).
	AUD002 AuditCode = "AUD002"
	// AUD003 indicates the same file appears more than once in the binder (duplicate reference).
	AUD003 AuditCode = "AUD003"
	// AUD004 indicates a node file frontmatter id does not match its filename stem.
	AUD004 AuditCode = "AUD004"
	// AUD005 indicates a required frontmatter field (id, created, or updated) is absent or malformed.
	AUD005 AuditCode = "AUD005"
	// AUD006 indicates a node file has valid frontmatter but empty or whitespace-only body (warning).
	AUD006 AuditCode = "AUD006"
	// AUD007 indicates a node file YAML frontmatter block is syntactically unparseable.
	AUD007 AuditCode = "AUD007"
	// AUDW001 is a warning indicating a non-UUID filename linked in binder (backward-compatibility warning for Feature 001 projects).
	AUDW001 AuditCode = "AUDW001"
)

// AuditSeverity classifies the impact level of an audit diagnostic.
type AuditSeverity string

const (
	// SeverityError indicates a condition that must be resolved.
	SeverityError AuditSeverity = "error"
	// SeverityWarning indicates a condition that should be reviewed.
	SeverityWarning AuditSeverity = "warning"
)

// AuditDiagnostic is a single finding produced by the audit command.
type AuditDiagnostic struct {
	// Code is the rule identifier that produced this diagnostic.
	Code AuditCode
	// Severity indicates whether this is an error or warning.
	Severity AuditSeverity
	// Message is a human-readable description of the finding.
	Message string
	// Path is the file or resource path associated with the finding.
	Path string
}
