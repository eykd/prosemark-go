package acceptance

import (
	"testing"
)

func TestStepFields(t *testing.T) {
	tests := []struct {
		name    string
		step    Step
		keyword string
		text    string
		line    int
	}{
		{
			name:    "GIVEN step",
			step:    Step{Keyword: "GIVEN", Text: "an empty outline.", Line: 3},
			keyword: "GIVEN",
			text:    "an empty outline.",
			line:    3,
		},
		{
			name:    "WHEN step",
			step:    Step{Keyword: "WHEN", Text: "the user adds an item.", Line: 5},
			keyword: "WHEN",
			text:    "the user adds an item.",
			line:    5,
		},
		{
			name:    "THEN step",
			step:    Step{Keyword: "THEN", Text: "the outline contains 1 item.", Line: 7},
			keyword: "THEN",
			text:    "the outline contains 1 item.",
			line:    7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.step.Keyword != tt.keyword {
				t.Errorf("Keyword = %q, want %q", tt.step.Keyword, tt.keyword)
			}
			if tt.step.Text != tt.text {
				t.Errorf("Text = %q, want %q", tt.step.Text, tt.text)
			}
			if tt.step.Line != tt.line {
				t.Errorf("Line = %d, want %d", tt.step.Line, tt.line)
			}
		})
	}
}

func TestScenarioFields(t *testing.T) {
	steps := []Step{
		{Keyword: "GIVEN", Text: "an empty outline.", Line: 3},
		{Keyword: "WHEN", Text: "the user adds an item.", Line: 5},
		{Keyword: "THEN", Text: "the outline contains 1 item.", Line: 7},
	}

	scenario := Scenario{
		Description: "User can add a new outline item.",
		Steps:       steps,
		Line:        1,
	}

	if scenario.Description != "User can add a new outline item." {
		t.Errorf("Description = %q, want %q", scenario.Description, "User can add a new outline item.")
	}
	if scenario.Line != 1 {
		t.Errorf("Line = %d, want %d", scenario.Line, 1)
	}
	if len(scenario.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want %d", len(scenario.Steps), 3)
	}
	if scenario.Steps[0].Keyword != "GIVEN" {
		t.Errorf("Steps[0].Keyword = %q, want %q", scenario.Steps[0].Keyword, "GIVEN")
	}
}

func TestFeatureFields(t *testing.T) {
	feature := Feature{
		SourceFile: "specs/US1-add-item.txt",
		Scenarios: []Scenario{
			{
				Description: "User can add a new outline item.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "an empty outline.", Line: 3},
				},
				Line: 1,
			},
		},
	}

	if feature.SourceFile != "specs/US1-add-item.txt" {
		t.Errorf("SourceFile = %q, want %q", feature.SourceFile, "specs/US1-add-item.txt")
	}
	if len(feature.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want %d", len(feature.Scenarios), 1)
	}
	if feature.Scenarios[0].Description != "User can add a new outline item." {
		t.Errorf("Scenarios[0].Description = %q, want %q", feature.Scenarios[0].Description, "User can add a new outline item.")
	}
}

func TestFeatureMultipleScenarios(t *testing.T) {
	feature := Feature{
		SourceFile: "specs/US2-manage-items.txt",
		Scenarios: []Scenario{
			{Description: "Add item", Steps: []Step{}, Line: 1},
			{Description: "Remove item", Steps: []Step{}, Line: 10},
		},
	}

	if len(feature.Scenarios) != 2 {
		t.Fatalf("len(Scenarios) = %d, want %d", len(feature.Scenarios), 2)
	}
	if feature.Scenarios[1].Description != "Remove item" {
		t.Errorf("Scenarios[1].Description = %q, want %q", feature.Scenarios[1].Description, "Remove item")
	}
}

func TestStepZeroValue(t *testing.T) {
	var s Step
	if s.Keyword != "" {
		t.Errorf("zero Step.Keyword = %q, want empty", s.Keyword)
	}
	if s.Text != "" {
		t.Errorf("zero Step.Text = %q, want empty", s.Text)
	}
	if s.Line != 0 {
		t.Errorf("zero Step.Line = %d, want 0", s.Line)
	}
}

func TestScenarioZeroValue(t *testing.T) {
	var s Scenario
	if s.Description != "" {
		t.Errorf("zero Scenario.Description = %q, want empty", s.Description)
	}
	if s.Steps != nil {
		t.Errorf("zero Scenario.Steps = %v, want nil", s.Steps)
	}
	if s.Line != 0 {
		t.Errorf("zero Scenario.Line = %d, want 0", s.Line)
	}
}

func TestFeatureZeroValue(t *testing.T) {
	var f Feature
	if f.SourceFile != "" {
		t.Errorf("zero Feature.SourceFile = %q, want empty", f.SourceFile)
	}
	if f.Scenarios != nil {
		t.Errorf("zero Feature.Scenarios = %v, want nil", f.Scenarios)
	}
}
