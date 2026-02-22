// Package acceptance provides a pipeline for transforming GWT (Given-When-Then)
// acceptance specs into executable Go test files.
package acceptance

// Step represents a single GIVEN, WHEN, or THEN statement in a scenario.
type Step struct {
	// Keyword is the step type: "GIVEN", "WHEN", or "THEN".
	Keyword string
	// Text is the step description without the keyword prefix.
	Text string
	// Line is the source line number where this step appears.
	Line int
}

// Scenario represents a named acceptance scenario containing a sequence of steps.
type Scenario struct {
	// Description is the human-readable scenario title from the ;=== header.
	Description string
	// Steps is the ordered sequence of GIVEN/WHEN/THEN steps.
	Steps []Step
	// Line is the source line number of the scenario description header.
	Line int
}

// Feature represents a parsed acceptance spec file containing one or more scenarios.
type Feature struct {
	// SourceFile is the path to the spec file this feature was parsed from.
	SourceFile string
	// Scenarios is the list of scenarios defined in the spec file.
	Scenarios []Scenario
}
