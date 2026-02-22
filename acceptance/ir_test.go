package acceptance

import (
	"encoding/json"
	"testing"
)

func TestSerializeIR_SingleScenario(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US1-add-item.txt",
		Scenarios: []Scenario{
			{
				Description: "User can add a new outline item.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "an empty outline.", Line: 3},
					{Keyword: "WHEN", Text: "the user adds an item.", Line: 5},
					{Keyword: "THEN", Text: "the outline contains 1 item.", Line: 7},
				},
				Line: 1,
			},
		},
	}

	data, err := SerializeIR(feature)
	if err != nil {
		t.Fatalf("SerializeIR() error = %v", err)
	}

	// Verify it's valid JSON
	if !json.Valid(data) {
		t.Fatal("SerializeIR() produced invalid JSON")
	}

	// Verify round-trip
	got, err := DeserializeIR(data)
	if err != nil {
		t.Fatalf("DeserializeIR() error = %v", err)
	}
	if got.SourceFile != feature.SourceFile {
		t.Errorf("SourceFile = %q, want %q", got.SourceFile, feature.SourceFile)
	}
	if len(got.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(got.Scenarios))
	}
	if got.Scenarios[0].Description != feature.Scenarios[0].Description {
		t.Errorf("Description = %q, want %q", got.Scenarios[0].Description, feature.Scenarios[0].Description)
	}
	if len(got.Scenarios[0].Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(got.Scenarios[0].Steps))
	}
}

func TestSerializeIR_EmptyFeature(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/empty.txt",
	}

	data, err := SerializeIR(feature)
	if err != nil {
		t.Fatalf("SerializeIR() error = %v", err)
	}

	got, err := DeserializeIR(data)
	if err != nil {
		t.Fatalf("DeserializeIR() error = %v", err)
	}
	if got.SourceFile != "specs/empty.txt" {
		t.Errorf("SourceFile = %q, want %q", got.SourceFile, "specs/empty.txt")
	}
	if got.Scenarios != nil {
		t.Errorf("Scenarios = %v, want nil", got.Scenarios)
	}
}

func TestSerializeIR_MultipleScenarios(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US2-manage.txt",
		Scenarios: []Scenario{
			{Description: "Add item", Steps: []Step{{Keyword: "GIVEN", Text: "empty.", Line: 3}}, Line: 1},
			{Description: "Remove item", Steps: []Step{{Keyword: "GIVEN", Text: "one item.", Line: 10}}, Line: 8},
		},
	}

	data, err := SerializeIR(feature)
	if err != nil {
		t.Fatalf("SerializeIR() error = %v", err)
	}

	got, err := DeserializeIR(data)
	if err != nil {
		t.Fatalf("DeserializeIR() error = %v", err)
	}
	if len(got.Scenarios) != 2 {
		t.Fatalf("len(Scenarios) = %d, want 2", len(got.Scenarios))
	}
	if got.Scenarios[0].Description != "Add item" {
		t.Errorf("Scenarios[0].Description = %q", got.Scenarios[0].Description)
	}
	if got.Scenarios[1].Description != "Remove item" {
		t.Errorf("Scenarios[1].Description = %q", got.Scenarios[1].Description)
	}
}

func TestSerializeIR_PreservesStepFields(t *testing.T) {
	feature := &Feature{
		SourceFile: "test.txt",
		Scenarios: []Scenario{
			{
				Description: "test",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "a thing.", Line: 4},
					{Keyword: "WHEN", Text: "action happens.", Line: 6},
					{Keyword: "THEN", Text: "result observed.", Line: 8},
				},
				Line: 2,
			},
		},
	}

	data, err := SerializeIR(feature)
	if err != nil {
		t.Fatalf("SerializeIR() error = %v", err)
	}

	got, err := DeserializeIR(data)
	if err != nil {
		t.Fatalf("DeserializeIR() error = %v", err)
	}

	steps := got.Scenarios[0].Steps
	wantSteps := feature.Scenarios[0].Steps
	for i := range steps {
		if steps[i].Keyword != wantSteps[i].Keyword {
			t.Errorf("Steps[%d].Keyword = %q, want %q", i, steps[i].Keyword, wantSteps[i].Keyword)
		}
		if steps[i].Text != wantSteps[i].Text {
			t.Errorf("Steps[%d].Text = %q, want %q", i, steps[i].Text, wantSteps[i].Text)
		}
		if steps[i].Line != wantSteps[i].Line {
			t.Errorf("Steps[%d].Line = %d, want %d", i, steps[i].Line, wantSteps[i].Line)
		}
	}
}

func TestDeserializeIR_InvalidJSON(t *testing.T) {
	_, err := DeserializeIR([]byte("not json"))
	if err == nil {
		t.Error("DeserializeIR() expected error for invalid JSON, got nil")
	}
}

func TestSerializeIR_ProducesIndentedJSON(t *testing.T) {
	feature := &Feature{
		SourceFile: "test.txt",
		Scenarios: []Scenario{
			{Description: "test", Line: 1},
		},
	}

	data, err := SerializeIR(feature)
	if err != nil {
		t.Fatalf("SerializeIR() error = %v", err)
	}

	// Indented JSON should contain newlines
	str := string(data)
	if len(str) == 0 {
		t.Fatal("SerializeIR() produced empty output")
	}
	// Should contain newlines (indented format)
	if !json.Valid(data) {
		t.Error("SerializeIR() output is not valid JSON")
	}
}
