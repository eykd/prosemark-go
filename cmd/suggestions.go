package cmd

import "github.com/eykd/prosemark-go/internal/binder"

var suggestionMap = map[string]string{
	binder.CodeSelectorNoMatch:   "Run 'pmk parse --json' to list available nodes and their selectors.",
	binder.CodeAmbiguousBareStem: "Use a full path selector (e.g., 'parent/child.md') to disambiguate.",
	binder.CodeCycleDetected:     "The destination is a descendant of the source. Choose a different destination.",
	binder.CodeInvalidTargetPath: "Check that the target path contains only valid filename characters.",
	binder.CodeTargetIsBinder:    "The binder file cannot be added as a node. Choose a different target.",
	binder.CodeNodeInCodeFence:   "The node is inside a code fence. Move it outside the fenced block.",
	binder.CodeSiblingNotFound:   "The sibling selector matched no nodes. Run 'pmk parse --json' to verify.",
	binder.CodeIndexOutOfBounds:  "The index is out of bounds. Run 'pmk parse --json' to check child count.",
	binder.CodeIOOrParseFailure:  "Check that '_binder.md' exists and is readable. Run 'pmk doctor' to diagnose.",
	binder.CodeConflictingFlags:  "Specify only one positioning flag: --first, --at, --before, or --after.",
	binder.CodeIllegalPathChars:  "Remove illegal characters from the file path.",
	binder.CodePathEscapesRoot:   "Paths must not escape the project root with '../'.",
	binder.CodeAmbiguousWikilink: "Use a full path instead of a wikilink to resolve the ambiguity.",
}

func attachSuggestions(diags []binder.Diagnostic) {
	for i := range diags {
		if s, ok := suggestionMap[diags[i].Code]; ok {
			diags[i].Suggestion = s
		}
	}
}
