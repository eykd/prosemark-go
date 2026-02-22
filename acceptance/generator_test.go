package acceptance

import (
	"strings"
	"testing"
)

func TestGenerateTests_SingleScenario(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US1-add-item.txt",
		Scenarios: []Scenario{
			{
				Description: "User can add a new outline item.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "an empty outline.", Line: 4},
					{Keyword: "WHEN", Text: `the user adds an item titled "Chapter 1".`, Line: 6},
					{Keyword: "THEN", Text: "the outline contains 1 item.", Line: 8},
				},
				Line: 2,
			},
		},
	}

	output, err := GenerateTests(feature, "")
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	// Must contain preserved header
	if !strings.Contains(output, "bound implementations are preserved") {
		t.Error("output missing preserved header")
	}

	// Must contain package declaration
	if !strings.Contains(output, "package acceptance_test") {
		t.Error("output missing package declaration")
	}

	// Must import testing
	if !strings.Contains(output, `"testing"`) {
		t.Error("output missing testing import")
	}

	// Must contain a test function
	if !strings.Contains(output, "func Test") {
		t.Error("output missing test function")
	}

	// Must contain t.Fatal stub
	if !strings.Contains(output, "t.Fatal") {
		t.Error("output missing t.Fatal stub")
	}

	// Must reference the scenario description
	if !strings.Contains(output, "User can add a new outline item") {
		t.Error("output missing scenario description")
	}

	// Must contain step comments
	if !strings.Contains(output, "GIVEN") {
		t.Error("output missing GIVEN step comment")
	}
	if !strings.Contains(output, "WHEN") {
		t.Error("output missing WHEN step comment")
	}
	if !strings.Contains(output, "THEN") {
		t.Error("output missing THEN step comment")
	}
}

func TestGenerateTests_MultipleScenarios(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US2-manage.txt",
		Scenarios: []Scenario{
			{
				Description: "User can add an item.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "an empty outline.", Line: 4},
				},
				Line: 2,
			},
			{
				Description: "User can remove an item.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "an outline with items.", Line: 12},
				},
				Line: 10,
			},
		},
	}

	output, err := GenerateTests(feature, "")
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	// Must have two test functions
	count := strings.Count(output, "func Test")
	if count != 2 {
		t.Errorf("test function count = %d, want 2", count)
	}

	if !strings.Contains(output, "User can add an item") {
		t.Error("output missing first scenario description")
	}
	if !strings.Contains(output, "User can remove an item") {
		t.Error("output missing second scenario description")
	}
}

func TestGenerateTests_EmptyFeature(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/empty.txt",
	}

	output, err := GenerateTests(feature, "")
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	// Should still produce valid Go with preserved header
	if !strings.Contains(output, "bound implementations are preserved") {
		t.Error("output missing preserved header")
	}
	if !strings.Contains(output, "package acceptance_test") {
		t.Error("output missing package declaration")
	}

	// Should not contain test functions
	if strings.Contains(output, "func Test") {
		t.Error("output should not contain test functions for empty feature")
	}
}

func TestGenerateTests_FunctionNameSanitization(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US1-test.txt",
		Scenarios: []Scenario{
			{
				Description: `User adds "special" item/thing.`,
				Steps: []Step{
					{Keyword: "GIVEN", Text: "something.", Line: 4},
				},
				Line: 2,
			},
		},
	}

	output, err := GenerateTests(feature, "")
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	// Function name should not contain special chars
	// Should contain a valid Go test function name
	if !strings.Contains(output, "func Test") {
		t.Error("output missing test function")
	}

	// Function name should not have quotes, slashes, or dots
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func Test") {
			// Extract just the function name (before the parenthesis)
			parenIdx := strings.Index(trimmed, "(")
			if parenIdx == -1 {
				continue
			}
			funcName := trimmed[len("func "):parenIdx]
			if strings.ContainsAny(funcName, `"/.`) {
				t.Errorf("function name contains invalid chars: %s", funcName)
			}
		}
	}
}

func TestGenerateTests_SourceFileInComment(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US5-feature.txt",
		Scenarios: []Scenario{
			{
				Description: "A scenario.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "something.", Line: 4},
				},
				Line: 2,
			},
		},
	}

	output, err := GenerateTests(feature, "")
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	if !strings.Contains(output, "specs/US5-feature.txt") {
		t.Error("output missing source file reference")
	}
}

func TestGenerateTests_StepTextInComments(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US1-test.txt",
		Scenarios: []Scenario{
			{
				Description: "Test scenario.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "a precondition exists.", Line: 4},
					{Keyword: "WHEN", Text: "the action is performed.", Line: 6},
					{Keyword: "THEN", Text: "the expected outcome occurs.", Line: 8},
				},
				Line: 2,
			},
		},
	}

	output, err := GenerateTests(feature, "")
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	if !strings.Contains(output, "a precondition exists.") {
		t.Error("output missing GIVEN step text")
	}
	if !strings.Contains(output, "the action is performed.") {
		t.Error("output missing WHEN step text")
	}
	if !strings.Contains(output, "the expected outcome occurs.") {
		t.Error("output missing THEN step text")
	}
}

// --- ExtractBoundFunctions tests ---

func TestExtractBoundFunctions_EmptySource(t *testing.T) {
	got := ExtractBoundFunctions("")
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

func TestExtractBoundFunctions_AllUnbound(t *testing.T) {
	source := `package acceptance_test

import ("testing")

// Scenario description
// Source: specs/US1-test.txt:2
func Test_User_can_add_item(t *testing.T) {
	// GIVEN an empty outline.
	// WHEN the user adds an item.
	// THEN the outline has one item.

	t.Fatal("acceptance test not yet bound")
}
`
	got := ExtractBoundFunctions(source)
	if len(got) != 0 {
		t.Errorf("expected empty map for all-unbound source, got %d entries", len(got))
	}
}

func TestExtractBoundFunctions_SingleBound(t *testing.T) {
	source := `package acceptance_test

import ("testing")

// User can add an item.
// Source: specs/US1-test.txt:2
func Test_User_can_add_item(t *testing.T) {
	dir := t.TempDir()
	_ = dir
}
`
	got := ExtractBoundFunctions(source)
	if len(got) != 1 {
		t.Fatalf("expected 1 bound function, got %d", len(got))
	}
	fn, ok := got["Test_User_can_add_item"]
	if !ok {
		t.Fatal("missing expected function Test_User_can_add_item")
	}
	if !strings.Contains(fn, "func Test_User_can_add_item(t *testing.T)") {
		t.Errorf("function body missing signature, got:\n%s", fn)
	}
	if !strings.Contains(fn, "t.TempDir()") {
		t.Errorf("function body missing implementation, got:\n%s", fn)
	}
}

func TestExtractBoundFunctions_MixedBoundAndUnbound(t *testing.T) {
	source := `package acceptance_test

import ("testing")

// Bound scenario
// Source: specs/US1.txt:2
func Test_Bound_one(t *testing.T) {
	dir := t.TempDir()
	_ = dir
}

// Unbound scenario
// Source: specs/US1.txt:10
func Test_Unbound_one(t *testing.T) {
	// GIVEN something.

	t.Fatal("acceptance test not yet bound")
}

// Another bound scenario
// Source: specs/US1.txt:18
func Test_Bound_two(t *testing.T) {
	// real implementation
	if true {
		_ = "nested"
	}
}
`
	got := ExtractBoundFunctions(source)
	if len(got) != 2 {
		t.Fatalf("expected 2 bound functions, got %d", len(got))
	}
	if _, ok := got["Test_Bound_one"]; !ok {
		t.Error("missing Test_Bound_one")
	}
	if _, ok := got["Test_Bound_two"]; !ok {
		t.Error("missing Test_Bound_two")
	}
	if _, ok := got["Test_Unbound_one"]; ok {
		t.Error("should not include unbound Test_Unbound_one")
	}
}

func TestExtractBoundFunctions_NestedBraces(t *testing.T) {
	source := `package acceptance_test

import ("testing")

func Test_Nested_braces(t *testing.T) {
	if true {
		for i := 0; i < 10; i++ {
			if i > 5 {
				_ = i
			}
		}
	}
}
`
	got := ExtractBoundFunctions(source)
	if len(got) != 1 {
		t.Fatalf("expected 1 bound function, got %d", len(got))
	}
	fn := got["Test_Nested_braces"]
	if !strings.Contains(fn, "for i := 0") {
		t.Errorf("nested body not preserved, got:\n%s", fn)
	}
}

func TestExtractBoundFunctions_PreservesComments(t *testing.T) {
	source := `package acceptance_test

import ("testing")

// A scenario description
// Source: specs/US1.txt:5
func Test_With_comments(t *testing.T) {
	_ = "bound"
}
`
	got := ExtractBoundFunctions(source)
	fn, ok := got["Test_With_comments"]
	if !ok {
		t.Fatal("missing Test_With_comments")
	}
	if !strings.Contains(fn, "// A scenario description") {
		t.Errorf("preceding comment not preserved, got:\n%s", fn)
	}
	if !strings.Contains(fn, "// Source: specs/US1.txt:5") {
		t.Errorf("source comment not preserved, got:\n%s", fn)
	}
}

func TestExtractBoundFunctions_MultipleBound(t *testing.T) {
	source := `package acceptance_test

import ("testing")

func Test_First(t *testing.T) {
	_ = "first"
}

func Test_Second(t *testing.T) {
	_ = "second"
}

func Test_Third(t *testing.T) {
	_ = "third"
}
`
	got := ExtractBoundFunctions(source)
	if len(got) != 3 {
		t.Fatalf("expected 3 bound functions, got %d", len(got))
	}
}

// --- ExtractImports tests ---

func TestExtractImports_EmptySource(t *testing.T) {
	got := ExtractImports("")
	if got != "" {
		t.Errorf("expected empty string, got: %q", got)
	}
}

func TestExtractImports_StandardImport(t *testing.T) {
	source := `package acceptance_test

import (
	"testing"
)

func Test_Something(t *testing.T) {}
`
	got := ExtractImports(source)
	if !strings.Contains(got, `"testing"`) {
		t.Errorf("expected testing import, got: %q", got)
	}
}

func TestExtractImports_MultiImport(t *testing.T) {
	source := `package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func Test_Something(t *testing.T) {}
`
	got := ExtractImports(source)
	for _, pkg := range []string{`"os"`, `"os/exec"`, `"path/filepath"`, `"strings"`, `"testing"`} {
		if !strings.Contains(got, pkg) {
			t.Errorf("missing import %s in: %q", pkg, got)
		}
	}
}

func TestExtractImports_NoImport(t *testing.T) {
	source := `package acceptance_test

func Test_Something(t *testing.T) {}
`
	got := ExtractImports(source)
	if got != "" {
		t.Errorf("expected empty string for no import, got: %q", got)
	}
}

// --- GenerateTests merge behavior tests ---

func TestGenerateTests_BoundFunctionPreserved(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US1-test.txt",
		Scenarios: []Scenario{
			{
				Description: "User can add an item.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "an empty outline.", Line: 4},
				},
				Line: 2,
			},
		},
	}

	existingSource := `package acceptance_test

import (
	"testing"
)

// User can add an item.
// Source: specs/US1-test.txt:2
func Test_User_can_add_an_item(t *testing.T) {
	dir := t.TempDir()
	_ = dir
}
`

	output, err := GenerateTests(feature, existingSource)
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	// Bound function should be preserved
	if !strings.Contains(output, "t.TempDir()") {
		t.Error("bound function implementation not preserved")
	}

	// Should NOT contain the unbound sentinel
	if strings.Contains(output, UnboundSentinel) {
		t.Error("bound function was replaced with stub")
	}
}

func TestGenerateTests_UnboundRegeneratedWhenExisting(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US1-test.txt",
		Scenarios: []Scenario{
			{
				Description: "User can add an item.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "an empty outline.", Line: 4},
				},
				Line: 2,
			},
		},
	}

	existingSource := `package acceptance_test

import (
	"testing"
)

// User can add an item.
// Source: specs/US1-test.txt:2
func Test_User_can_add_an_item(t *testing.T) {
	// GIVEN an empty outline.

	t.Fatal("acceptance test not yet bound")
}
`

	output, err := GenerateTests(feature, existingSource)
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	// Unbound function should be regenerated (still contains sentinel)
	if !strings.Contains(output, UnboundSentinel) {
		t.Error("unbound function should still have sentinel")
	}
}

func TestGenerateTests_MixedBoundAndNewScenarios(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US1-test.txt",
		Scenarios: []Scenario{
			{
				Description: "Existing bound scenario.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "something.", Line: 4},
				},
				Line: 2,
			},
			{
				Description: "Brand new scenario.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "something else.", Line: 10},
				},
				Line: 8,
			},
		},
	}

	existingSource := `package acceptance_test

import (
	"testing"
)

// Existing bound scenario.
// Source: specs/US1-test.txt:2
func Test_Existing_bound_scenario(t *testing.T) {
	_ = "I am bound"
}
`

	output, err := GenerateTests(feature, existingSource)
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	// Bound function preserved
	if !strings.Contains(output, `"I am bound"`) {
		t.Error("existing bound function not preserved")
	}

	// New scenario gets stub
	if !strings.Contains(output, "Brand new scenario") {
		t.Error("new scenario missing")
	}
	// Count sentinels - should be exactly 1 (only the new scenario)
	sentinelCount := strings.Count(output, UnboundSentinel)
	if sentinelCount != 1 {
		t.Errorf("expected 1 sentinel (new scenario only), got %d", sentinelCount)
	}
}

func TestGenerateTests_OrphanedFunctionAppended(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US1-test.txt",
		Scenarios: []Scenario{
			{
				Description: "Current scenario.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "something.", Line: 4},
				},
				Line: 2,
			},
		},
	}

	// Existing source has a bound function that no longer matches any scenario
	existingSource := `package acceptance_test

import (
	"testing"
)

// Renamed scenario (was "Old scenario").
// Source: specs/US1-test.txt:2
func Test_Old_scenario(t *testing.T) {
	_ = "orphaned but bound"
}
`

	output, err := GenerateTests(feature, existingSource)
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	// Orphaned bound function should be appended
	if !strings.Contains(output, `"orphaned but bound"`) {
		t.Error("orphaned bound function not preserved")
	}

	// Should have a warning comment about orphaned functions
	if !strings.Contains(output, "orphaned") || !strings.Contains(output, "WARNING") {
		t.Error("missing orphaned function warning comment")
	}

	// Current scenario should still get a stub
	if !strings.Contains(output, "Current scenario") {
		t.Error("current scenario missing")
	}
}

func TestGenerateTests_ImportsPreservedFromExisting(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US1-test.txt",
		Scenarios: []Scenario{
			{
				Description: "A scenario.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "something.", Line: 4},
				},
				Line: 2,
			},
		},
	}

	existingSource := `package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A scenario.
// Source: specs/US1-test.txt:2
func Test_A_scenario(t *testing.T) {
	dir := t.TempDir()
	_ = dir
}
`

	output, err := GenerateTests(feature, existingSource)
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	// Existing imports should be preserved
	for _, pkg := range []string{`"os"`, `"os/exec"`, `"path/filepath"`, `"strings"`, `"testing"`} {
		if !strings.Contains(output, pkg) {
			t.Errorf("missing preserved import %s", pkg)
		}
	}
}

func TestGenerateTests_BackwardCompatibleWithEmptyExisting(t *testing.T) {
	feature := &Feature{
		SourceFile: "specs/US1-test.txt",
		Scenarios: []Scenario{
			{
				Description: "A scenario.",
				Steps: []Step{
					{Keyword: "GIVEN", Text: "something.", Line: 4},
				},
				Line: 2,
			},
		},
	}

	output, err := GenerateTests(feature, "")
	if err != nil {
		t.Fatalf("GenerateTests() error = %v", err)
	}

	// With empty existing source, should produce default import
	if !strings.Contains(output, `"testing"`) {
		t.Error("missing testing import")
	}
	// Should contain stub
	if !strings.Contains(output, UnboundSentinel) {
		t.Error("missing unbound sentinel")
	}
}

// --- UnboundSentinel constant test ---

func TestUnboundSentinel_Value(t *testing.T) {
	want := `t.Fatal("acceptance test not yet bound")`
	if UnboundSentinel != want {
		t.Errorf("UnboundSentinel = %q, want %q", UnboundSentinel, want)
	}
}
