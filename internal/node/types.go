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
	// AUD002 indicates a node is missing required front matter fields.
	AUD002 AuditCode = "AUD002"
	// AUD003 indicates a node has a malformed or non-UUID identifier.
	AUD003 AuditCode = "AUD003"
	// AUD004 indicates a binder entry does not match any known node file.
	AUD004 AuditCode = "AUD004"
	// AUD005 indicates a node file exists on disk but is absent from the binder.
	AUD005 AuditCode = "AUD005"
	// AUD006 indicates a node's front matter ID does not match its filename.
	AUD006 AuditCode = "AUD006"
	// AUD007 indicates a node has a duplicate identifier within the project.
	AUD007 AuditCode = "AUD007"
	// AUDW001 is a warning indicating a node has no title set.
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
