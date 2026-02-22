package acceptance

import (
	"testing"
)

func TestParseSpec_SingleScenario(t *testing.T) {
	content := `;===============================================================
; User can add a new outline item.
;===============================================================
GIVEN an empty outline.

WHEN the user adds an item titled "Chapter 1".

THEN the outline contains 1 item.
THEN the item is titled "Chapter 1".
`
	feature, err := ParseSpec(content, "specs/US1-add-item.txt")
	if err != nil {
		t.Fatalf("ParseSpec() error = %v", err)
	}
	if feature.SourceFile != "specs/US1-add-item.txt" {
		t.Errorf("SourceFile = %q, want %q", feature.SourceFile, "specs/US1-add-item.txt")
	}
	if len(feature.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(feature.Scenarios))
	}

	scenario := feature.Scenarios[0]
	if scenario.Description != "User can add a new outline item." {
		t.Errorf("Description = %q, want %q", scenario.Description, "User can add a new outline item.")
	}
	if scenario.Line != 2 {
		t.Errorf("Line = %d, want 2", scenario.Line)
	}
	if len(scenario.Steps) != 4 {
		t.Fatalf("len(Steps) = %d, want 4", len(scenario.Steps))
	}

	wantSteps := []Step{
		{Keyword: "GIVEN", Text: "an empty outline.", Line: 4},
		{Keyword: "WHEN", Text: `the user adds an item titled "Chapter 1".`, Line: 6},
		{Keyword: "THEN", Text: "the outline contains 1 item.", Line: 8},
		{Keyword: "THEN", Text: `the item is titled "Chapter 1".`, Line: 9},
	}

	for i, want := range wantSteps {
		got := scenario.Steps[i]
		if got.Keyword != want.Keyword {
			t.Errorf("Steps[%d].Keyword = %q, want %q", i, got.Keyword, want.Keyword)
		}
		if got.Text != want.Text {
			t.Errorf("Steps[%d].Text = %q, want %q", i, got.Text, want.Text)
		}
		if got.Line != want.Line {
			t.Errorf("Steps[%d].Line = %d, want %d", i, got.Line, want.Line)
		}
	}
}

func TestParseSpec_MultipleScenarios(t *testing.T) {
	content := `;===============================================================
; User can add a new outline item.
;===============================================================
GIVEN an empty outline.

WHEN the user adds an item titled "Chapter 1".

THEN the outline contains 1 item.

;===============================================================
; User can remove an outline item.
;===============================================================
GIVEN an outline with 1 item titled "Chapter 1".

WHEN the user removes the item titled "Chapter 1".

THEN the outline contains 0 items.
`
	feature, err := ParseSpec(content, "specs/US2-manage-items.txt")
	if err != nil {
		t.Fatalf("ParseSpec() error = %v", err)
	}
	if len(feature.Scenarios) != 2 {
		t.Fatalf("len(Scenarios) = %d, want 2", len(feature.Scenarios))
	}
	if feature.Scenarios[0].Description != "User can add a new outline item." {
		t.Errorf("Scenarios[0].Description = %q", feature.Scenarios[0].Description)
	}
	if feature.Scenarios[1].Description != "User can remove an outline item." {
		t.Errorf("Scenarios[1].Description = %q", feature.Scenarios[1].Description)
	}
	if len(feature.Scenarios[0].Steps) != 3 {
		t.Errorf("Scenarios[0] len(Steps) = %d, want 3", len(feature.Scenarios[0].Steps))
	}
	if len(feature.Scenarios[1].Steps) != 3 {
		t.Errorf("Scenarios[1] len(Steps) = %d, want 3", len(feature.Scenarios[1].Steps))
	}
}

func TestParseSpec_CommentLinesIgnored(t *testing.T) {
	content := `;===============================================================
; User can view an item.
;===============================================================
; This is a comment explaining setup
GIVEN an outline with items.

; Another comment
WHEN the user selects an item.

THEN the item details are displayed.
`
	feature, err := ParseSpec(content, "specs/US3-view-item.txt")
	if err != nil {
		t.Fatalf("ParseSpec() error = %v", err)
	}
	if len(feature.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(feature.Scenarios))
	}
	if len(feature.Scenarios[0].Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(feature.Scenarios[0].Steps))
	}
}

func TestParseSpec_EmptyContent(t *testing.T) {
	feature, err := ParseSpec("", "specs/empty.txt")
	if err != nil {
		t.Fatalf("ParseSpec() error = %v", err)
	}
	if len(feature.Scenarios) != 0 {
		t.Errorf("len(Scenarios) = %d, want 0", len(feature.Scenarios))
	}
	if feature.SourceFile != "specs/empty.txt" {
		t.Errorf("SourceFile = %q, want %q", feature.SourceFile, "specs/empty.txt")
	}
}

func TestParseSpec_OnlyComments(t *testing.T) {
	content := `; Just comments
; Nothing else here
`
	feature, err := ParseSpec(content, "specs/comments.txt")
	if err != nil {
		t.Fatalf("ParseSpec() error = %v", err)
	}
	if len(feature.Scenarios) != 0 {
		t.Errorf("len(Scenarios) = %d, want 0", len(feature.Scenarios))
	}
}

func TestParseSpec_StepLineNumbers(t *testing.T) {
	content := `;===============================================================
; Scenario one.
;===============================================================
GIVEN a precondition.
WHEN an action.
THEN an outcome.
`
	feature, err := ParseSpec(content, "test.txt")
	if err != nil {
		t.Fatalf("ParseSpec() error = %v", err)
	}
	if len(feature.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(feature.Scenarios))
	}
	steps := feature.Scenarios[0].Steps
	if len(steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(steps))
	}
	// Lines: 1=separator, 2=description, 3=separator, 4=GIVEN, 5=WHEN, 6=THEN
	if steps[0].Line != 4 {
		t.Errorf("Steps[0].Line = %d, want 4", steps[0].Line)
	}
	if steps[1].Line != 5 {
		t.Errorf("Steps[1].Line = %d, want 5", steps[1].Line)
	}
	if steps[2].Line != 6 {
		t.Errorf("Steps[2].Line = %d, want 6", steps[2].Line)
	}
}

func TestParseSpec_DescriptionLine(t *testing.T) {
	content := `;===============================================================
; My scenario.
;===============================================================
GIVEN something.
`
	feature, err := ParseSpec(content, "test.txt")
	if err != nil {
		t.Fatalf("ParseSpec() error = %v", err)
	}
	if feature.Scenarios[0].Line != 2 {
		t.Errorf("Scenario.Line = %d, want 2", feature.Scenarios[0].Line)
	}
}

func TestParseSpec_TrimsWhitespace(t *testing.T) {
	content := `;===============================================================
;   Scenario with spaces.
;===============================================================
GIVEN   a precondition with spaces.
`
	feature, err := ParseSpec(content, "test.txt")
	if err != nil {
		t.Fatalf("ParseSpec() error = %v", err)
	}
	if feature.Scenarios[0].Description != "Scenario with spaces." {
		t.Errorf("Description = %q, want %q", feature.Scenarios[0].Description, "Scenario with spaces.")
	}
	if feature.Scenarios[0].Steps[0].Text != "a precondition with spaces." {
		t.Errorf("Step.Text = %q, want %q", feature.Scenarios[0].Steps[0].Text, "a precondition with spaces.")
	}
}

func TestParseSpec_NoSeparators(t *testing.T) {
	// Steps without a scenario header are still collected
	content := `GIVEN something.
WHEN action.
THEN result.
`
	feature, err := ParseSpec(content, "test.txt")
	if err != nil {
		t.Fatalf("ParseSpec() error = %v", err)
	}
	if len(feature.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(feature.Scenarios))
	}
	if feature.Scenarios[0].Description != "" {
		t.Errorf("Description = %q, want empty", feature.Scenarios[0].Description)
	}
	if len(feature.Scenarios[0].Steps) != 3 {
		t.Errorf("len(Steps) = %d, want 3", len(feature.Scenarios[0].Steps))
	}
}

func TestParseSpec_WindowsLineEndings(t *testing.T) {
	content := ";===============================================================\r\n; Windows scenario.\r\n;===============================================================\r\nGIVEN a windows file.\r\nTHEN it still parses.\r\n"
	feature, err := ParseSpec(content, "test.txt")
	if err != nil {
		t.Fatalf("ParseSpec() error = %v", err)
	}
	if len(feature.Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(feature.Scenarios))
	}
	if feature.Scenarios[0].Description != "Windows scenario." {
		t.Errorf("Description = %q, want %q", feature.Scenarios[0].Description, "Windows scenario.")
	}
	if feature.Scenarios[0].Steps[0].Text != "a windows file." {
		t.Errorf("Step.Text = %q, want %q", feature.Scenarios[0].Steps[0].Text, "a windows file.")
	}
}
