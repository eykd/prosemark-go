package cmd

var auditSuggestionMap = map[string]string{
	"AUD001": "Check that the referenced file exists on disk. Run 'pmk parse --json' to list binder entries.",
	"AUD002": "Add the orphaned file to the binder with 'pmk add-child', or remove it from the project directory.",
	"AUD003": "Remove the duplicate entry from '_binder.md'. Each file should appear only once.",
	"AUD004": "Rename the file to match its frontmatter id, or update the frontmatter id to match the filename.",
	"AUD005": "Add the missing frontmatter fields (id, created, updated) to the node file.",
	"AUD006": "Add content to the node file body, or remove the empty node from the binder.",
	"AUD007": "Fix the YAML syntax in the frontmatter block. Ensure it starts and ends with '---'.",
	"AUD008": "Create a '.prosemark.yml' config file in the project root. Run 'pmk init' to generate one.",
}

func attachAuditSuggestions(diags []DoctorDiagnosticJSON) {
	for i := range diags {
		if s, ok := auditSuggestionMap[diags[i].Code]; ok {
			diags[i].Suggestion = s
		}
	}
}
