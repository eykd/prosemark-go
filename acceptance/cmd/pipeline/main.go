// Package main provides the acceptance test pipeline CLI.
//
// Usage:
//
//	go run ./acceptance/cmd/pipeline -action=parse     # specs/*.txt -> IR JSON
//	go run ./acceptance/cmd/pipeline -action=generate  # IR -> Go test files
//	go run ./acceptance/cmd/pipeline -action=run       # parse + generate + go test
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/eykd/prosemark-go/acceptance"
)

const (
	specsDir    = "specs"
	irDir       = "acceptance-pipeline/ir"
	testDir     = "generated-acceptance-tests"
	specPattern = "*.txt"
)

func main() {
	action := flag.String("action", "", "Pipeline action: parse, generate, or run")
	flag.Parse()

	if *action == "" {
		fmt.Fprintf(os.Stderr, "Usage: pipeline -action=<parse|generate|run>\n")
		os.Exit(1)
	}

	var err error
	switch *action {
	case "parse":
		err = runParse()
	case "generate":
		err = runGenerate()
	case "run":
		err = runAll()
	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s (use parse, generate, or run)\n", *action)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runParse reads all spec files and writes IR JSON.
func runParse() error {
	var specFiles []string
	err := filepath.WalkDir(specsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".txt") {
			specFiles = append(specFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("finding spec files: %w", err)
	}

	if len(specFiles) == 0 {
		fmt.Println("No spec files found in specs/")
		return nil
	}

	if err := os.MkdirAll(irDir, 0o755); err != nil {
		return fmt.Errorf("creating IR directory: %w", err)
	}

	for _, specFile := range specFiles {
		feature, err := acceptance.ParseSpecFileImpl(specFile)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", specFile, err)
		}

		data, err := acceptance.SerializeIR(feature)
		if err != nil {
			return fmt.Errorf("serializing IR for %s: %w", specFile, err)
		}

		// Build a unique IR filename from the relative path under specsDir.
		rel, _ := filepath.Rel(specsDir, specFile)
		irName := strings.ReplaceAll(strings.TrimSuffix(rel, ".txt"), string(filepath.Separator), "-")
		irFile := filepath.Join(irDir, irName+".json")
		if err := acceptance.WriteIRImpl(irFile, data); err != nil {
			return fmt.Errorf("writing IR for %s: %w", specFile, err)
		}

		fmt.Printf("Parsed: %s -> %s\n", specFile, irFile)
	}

	return nil
}

// runGenerate reads IR JSON files and generates Go test files.
func runGenerate() error {
	irFiles, err := filepath.Glob(filepath.Join(irDir, "*.json"))
	if err != nil {
		return fmt.Errorf("finding IR files: %w", err)
	}

	if len(irFiles) == 0 {
		fmt.Println("No IR files found. Run -action=parse first.")
		return nil
	}

	if err := os.MkdirAll(testDir, 0o755); err != nil {
		return fmt.Errorf("creating test directory: %w", err)
	}

	for _, irFile := range irFiles {
		data, err := acceptance.ReadIRImpl(irFile)
		if err != nil {
			return fmt.Errorf("reading %s: %w", irFile, err)
		}

		feature, err := acceptance.DeserializeIR(data)
		if err != nil {
			return fmt.Errorf("deserializing %s: %w", irFile, err)
		}

		testFile := filepath.Join(testDir, strings.TrimSuffix(filepath.Base(irFile), ".json")+"_test.go")

		// Read existing test file to preserve bound implementations
		existingSource := ""
		if data, err := os.ReadFile(testFile); err == nil {
			existingSource = string(data)
		}

		testCode, err := acceptance.GenerateTests(feature, existingSource)
		if err != nil {
			return fmt.Errorf("generating tests for %s: %w", irFile, err)
		}
		if err := acceptance.WriteTestFileImpl(testFile, testCode); err != nil {
			return fmt.Errorf("writing test for %s: %w", irFile, err)
		}

		fmt.Printf("Generated: %s -> %s\n", irFile, testFile)
	}

	return nil
}

// runAll performs the full pipeline: parse, generate, and run tests.
func runAll() error {
	if err := runParse(); err != nil {
		return err
	}

	if err := runGenerate(); err != nil {
		return err
	}

	fmt.Println("\nRunning acceptance tests...")
	cmd := exec.Command("go", "test", "-v", "./"+testDir+"/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
