# Coverage Strategies

## Table of Contents
1. [Running Coverage](#running-coverage)
2. [Coverage Analysis](#coverage-analysis)
3. [Common Coverage Gaps](#common-coverage-gaps)
4. [Branch Coverage](#branch-coverage)
5. [Excluding Code](#excluding-code)

## Running Coverage

### Basic Commands

```bash
# Quick coverage percentage
go test -cover ./...

# Generate profile
go test -coverprofile=coverage.out ./...

# View in terminal
go tool cover -func=coverage.out

# Visual HTML report
go tool cover -html=coverage.out

# Project-specific (excludes main)
just test-cover-check
```

### Interpreting Output

```
github.com/turtlebased/turtlebased-go/cmd/root.go:15:    Execute     100.0%
github.com/turtlebased/turtlebased-go/cmd/root.go:23:    init        100.0%
total:                                                   (statements) 100.0%
```

## Coverage Analysis

### Finding Uncovered Lines

```bash
# HTML report highlights uncovered lines in red
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

### Per-Package Coverage

```bash
go test -cover ./cmd/...
go test -cover ./internal/...
```

## Common Coverage Gaps

### 1. Error Paths

**Gap**: Error branches not tested.

```go
func Process(data []byte) error {
    if len(data) == 0 {
        return ErrEmpty  // ← Often untested
    }
    // ...
}
```

**Fix**: Add error case to table-driven tests.

```go
{"empty input", []byte{}, nil, true},
```

### 2. Switch Default Cases

**Gap**: Default branch not exercised.

```go
switch status {
case "active":
    return handleActive()
case "pending":
    return handlePending()
default:
    return ErrUnknownStatus  // ← Test this
}
```

**Fix**: Test with invalid status.

### 3. Early Returns

**Gap**: Guard clauses not tested.

```go
func (s *Service) Update(id string) error {
    if s.readOnly {
        return ErrReadOnly  // ← Test this path
    }
    // ...
}
```

**Fix**: Test with readOnly=true.

### 4. Deferred Functions

**Gap**: Deferred cleanup not triggered in tests.

```go
func ReadFile(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()  // ← Must reach here
    return io.ReadAll(f)
}
```

**Fix**: Test the success path (reaches defer).

## Branch Coverage

### Testing All Branches

```go
func Classify(n int) string {
    if n < 0 {
        return "negative"
    } else if n == 0 {
        return "zero"
    } else {
        return "positive"
    }
}

func TestClassify(t *testing.T) {
    tests := []struct {
        input int
        want  string
    }{
        {-5, "negative"},   // Branch 1
        {0, "zero"},        // Branch 2
        {5, "positive"},    // Branch 3
    }
    // ...
}
```

### Boolean Combinations

```go
func ShouldProcess(enabled, valid bool) bool {
    return enabled && valid
}

// Test all combinations
{"both true", true, true, true},
{"enabled only", true, false, false},
{"valid only", false, true, false},
{"both false", false, false, false},
```

## Excluding Code

### Main Package

The main package is excluded from coverage by convention because `main()` is the entry point that's tested through acceptance tests.

**Pattern**: Keep main.go minimal.

```go
// main.go
package main

import (
    "os"
    "github.com/turtlebased/turtlebased-go/cmd"
)

func main() {
    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

All logic lives in `cmd.Execute()` which IS tested.

### Generated Code

If using code generators:

```bash
# Exclude from coverage in justfile
go test -coverprofile=coverage.out \
    -coverpkg=./cmd/...,./internal/... \
    ./...
```

### Build Tags for Untestable Code

For code that genuinely cannot be unit tested (external system calls, OS-specific behavior):

```go
//go:build !coverage

package mypackage

// This file is excluded when running coverage tests
// Use: go test -tags coverage ...
```

### Impl Functions Pattern

For functions that wrap external system calls (exec.Command, os operations), use the **impl pattern**:

1. Create a mockable package-level variable
2. Put the actual implementation in a small `*Impl` function
3. Test the logic by mocking the variable
4. Accept that the `*Impl` function has 0% unit test coverage

```go
// Mockable variable (tested via mocking)
var runCommandFn = runCommandImpl

// Business logic uses the variable (100% testable)
func DoWork() error {
    return runCommandFn("git", "status")
}

// Impl function wraps external call (exempt from coverage)
func runCommandImpl(name string, args ...string) error {
    return exec.Command(name, args...).Run()
}
```

In tests:
```go
func TestDoWork(t *testing.T) {
    // Save and restore
    orig := runCommandFn
    defer func() { runCommandFn = orig }()

    runCommandFn = func(name string, args ...string) error {
        return nil // Mock success
    }

    err := DoWork()
    // Assert...
}
```

The `runCommandImpl` function will show 0% coverage, but that's acceptable because:
- It's a thin wrapper with no logic
- It's tested via integration tests
- The business logic that uses it IS fully tested

### Excluding Impl Functions from Coverage Check

The coverage check in `justfile` and `lefthook.yml` verifies that all non-Impl functions are at 100% coverage:

```bash
# Filter out *Impl functions and check all others are at 100%
UNCOVERED=$(go tool cover -func=coverage.out | grep -v "Impl" | grep -v "main.go" | grep -v "100.0%" | grep -v "total:" || true)
if [ -n "$UNCOVERED" ]; then
    echo "The following functions need tests:"
    echo "$UNCOVERED"
    exit 1
fi
```

### File-Level Exclusion

For entire files that wrap external systems:

```go
// +build !coverage
// OR for Go 1.17+:
//go:build !coverage

package cmd

// external_commands.go - excluded from coverage
// These functions wrap external commands and are tested via integration tests

func runGitCommandImpl(args ...string) error {
    return exec.Command("git", args...).Run()
}
```

Then run tests with: `go test -tags coverage ./...`


### When to Exempt Code

**Exempt** (0% coverage acceptable):
- Direct `exec.Command` wrappers
- Direct `os.Open/Read/Write` in impl functions
- HTTP client `.Do()` calls in impl functions
- Database driver calls in impl functions

**Do NOT Exempt** (must test):
- Error handling logic
- Business logic around external calls
- Retry logic, timeouts, parsing responses
- Any code with conditional branches

## Maintaining 100% Coverage

### Pre-Commit Check

The project uses lefthook to enforce coverage:

```yaml
# lefthook.yml
pre-commit:
  commands:
    test-coverage:
      run: just test-cover-check
```

### When Coverage Drops

1. Run `just test-cover` to see HTML report
2. Find red (uncovered) lines
3. Add tests for uncovered paths
4. Verify with `just test-cover-check`

### TDD Naturally Achieves Coverage

When following TDD strictly:
- Every line of code exists because a test required it
- No speculative code is written
- Coverage is a natural byproduct, not a goal

**If coverage is hard to achieve, consider:**
- Is there dead code that should be removed?
- Is the design testable? (dependency injection, interfaces)
- Are there hidden dependencies on global state?

### Batch Implementation Anti-Pattern

When implementing multiple similar functions (e.g., adapters, handlers), it is tempting to write all code first and add tests later. This violates TDD and reliably produces coverage gaps in error paths.

**Rule**: Complete the full Red-Green-Refactor cycle for each function before starting the next. Run `just test-cover-check` after each function, not after the batch.

### Scenario Coverage vs Line Coverage

100% line coverage does not guarantee all behavior paths are tested. A function can have every line covered while missing critical *combinations* of inputs.

**Example**: A recursive function that compacts file paths had 100% line coverage. Tests covered:
- Root-level renumbering (parent changes, no children)
- Scoped compacting (children change, parent stays same)

But never tested both simultaneously — parent renumbered AND children compacted in the same call. This combination triggered a bug where the function composed old/new paths incorrectly.

**Rule**: For recursive or multi-step operations, test the *cross-product* of independent axes:
- If step A can change X, and step B can change Y, test the case where both change
- Table-driven tests make this natural — add the combination row
