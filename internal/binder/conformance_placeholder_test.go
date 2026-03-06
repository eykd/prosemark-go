package binder_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConformance_PlaceholderFixtures verifies that conformance fixture directories
// 121–125 exist and produce the expected parse output and diagnostics.
//
// These fixtures cover Phase 2 of the placeholder-parsing feature (spec §FR-001–FR-009).
// This test is RED until the fixture directories are created.
func TestConformance_PlaceholderFixtures(t *testing.T) {
	fixtureNames := []string{
		"121-placeholder-basic",
		"122-placeholder-empty-title",
		"123-placeholder-with-children",
		"124-placeholder-mixed",
		"125-placeholder-list-markers",
	}
	for _, name := range fixtureNames {
		name := name
		t.Run(name, func(t *testing.T) {
			fixturePath := filepath.Join(parseFixturesDir, name)
			if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
				t.Fatalf("fixture directory %q not found; placeholder conformance fixtures (Phase 2) must be created", fixturePath)
			}
			runParseFixture(t, fixturePath)
		})
	}
}
