package node_test

import (
	"testing"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// TestUUIDDependency verifies that github.com/google/uuid v1.6.0 is available.
// This test fails (compile error) until the dependency is added to go.mod.
func TestUUIDDependency(t *testing.T) {
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7() error = %v", err)
	}
	if id == (uuid.UUID{}) {
		t.Error("uuid.NewV7() returned zero UUID")
	}
}

// TestYAMLDependency verifies that gopkg.in/yaml.v3 is available.
// This test fails (compile error) until the dependency is added to go.mod.
func TestYAMLDependency(t *testing.T) {
	type stub struct {
		ID string `yaml:"id"`
	}

	src := stub{ID: "test-id"}
	data, err := yaml.Marshal(src)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var dst stub
	if err := yaml.Unmarshal(data, &dst); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if dst.ID != src.ID {
		t.Errorf("round-trip mismatch: got %q, want %q", dst.ID, src.ID)
	}
}
