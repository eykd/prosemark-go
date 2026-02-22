package acceptance

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SerializeIR marshals a Feature into indented JSON bytes.
func SerializeIR(feature *Feature) ([]byte, error) {
	return json.MarshalIndent(feature, "", "  ")
}

// DeserializeIR unmarshals JSON bytes into a Feature.
func DeserializeIR(data []byte) (*Feature, error) {
	var feature Feature
	if err := json.Unmarshal(data, &feature); err != nil {
		return nil, err
	}
	return &feature, nil
}

// WriteIRImpl writes IR JSON data to disk, creating directories as needed.
// This is an Impl function exempt from coverage requirements.
func WriteIRImpl(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ReadIRImpl reads IR JSON data from disk.
// This is an Impl function exempt from coverage requirements.
func ReadIRImpl(path string) ([]byte, error) {
	return os.ReadFile(path)
}
