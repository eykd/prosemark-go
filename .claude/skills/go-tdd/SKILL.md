---
name: go-tdd
description: TDD-based Go development for turtlebased-go CLI projects. Use when writing new features, adding tests, fixing bugs, or refactoring Go code. Triggers on requests involving Go code changes, test writing, coverage improvement, or TDD workflow guidance.
---

# Go TDD Development

## Workflow Decision Tree

**What are you doing?**
├── Adding a feature → § Outside-In TDD
├── Writing tests → § Test Patterns
├── Fixing a bug → § Bug Fix TDD
├── Refactoring → § Safe Refactoring
└── Improving coverage → See [references/coverage.md](references/coverage.md)

## Outside-In TDD (GOOS Style)

### The Two Loops

**Macro Loop** (feature-level):
1. Write failing acceptance test (end-to-end behavior)
2. Identify first missing piece from failure
3. Drop to unit tests to drive that piece
4. Implement minimum code to pass unit test
5. Integrate upward, re-run acceptance test
6. Repeat until acceptance test passes
7. Refactor

**Micro Loop** (each unit):
1. **Red**: Write failing test (must fail for right reason)
2. **Green**: Minimum code to pass (no more)
3. **Refactor**: Improve design, tests stay green
4. **Verify**: Run `just test-cover-check` before moving to the next unit

### Before Writing Test Code

Before defining test doubles or helpers in a new `*_test.go` file:

1. **Search for existing test doubles**: `grep -rn "^type.*struct" <package>/*_test.go`
2. **Reuse what exists** — Go compiles all `*_test.go` in a package together; duplicate type names cause compile errors
3. **Verify functions exist before calling them** — don't assume `Must*` variants exist; check with grep first

### CLI Error Visibility Checklist

Before marking a CLI wiring task complete, verify:

1. **Entry point prints errors**: If root command has `SilenceErrors: true`, the caller (`main.go`) MUST print errors to stderr. Test with `RunCLI()`.
2. **No silent nil runners**: Every command with a nil runner guard must be covered in `TestBuildCommandTree_AllCommandsHandleNilService`.
3. **Build-and-run smoke test**: `just smoke` passes.

### Feature Implementation Template

```go
// 1. Start with acceptance test (cmd level for CLI)
func TestCommand_EndToEnd(t *testing.T) {
    // Given: system state
    // When: command executes
    // Then: observable outcome
}

// 2. Unit test the component (drives design)
func TestService_Method_Scenario(t *testing.T) {
    // Given: dependencies (use interfaces)
    // When: method call
    // Then: result or interaction
}
```

## Test Patterns

### Table-Driven Tests (default pattern)

```go
func TestParse(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Result
        wantErr bool
    }{
        {"valid input", "abc", Result{Value: "abc"}, false},
        {"empty input", "", Result{}, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Parse(tt.input)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Interface-Based Mocking

```go
// Define interface by consumer need
type Repository interface {
    Find(id string) (*Entity, error)
}

// Test double
type mockRepo struct {
    entity *Entity
    err    error
}

func (m *mockRepo) Find(id string) (*Entity, error) {
    return m.entity, m.err
}
```

### Cobra Command Testing

```go
func TestRootCmd_Execute(t *testing.T) {
    cmd := NewRootCmd()
    buf := new(bytes.Buffer)
    cmd.SetOut(buf)
    cmd.SetErr(buf)
    cmd.SetArgs([]string{"--help"})
    
    err := cmd.Execute()
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !strings.Contains(buf.String(), "expected output") {
        t.Errorf("output = %q, want to contain 'expected output'", buf.String())
    }
}
```

## Bug Fix TDD

1. **Write failing test** that reproduces the bug
2. **Verify it fails** for the right reason
3. **Fix the code** until test passes
4. **Refactor** if needed
5. **Confirm** no regressions (`just test`)

## Safe Refactoring

1. **Ensure tests pass** before starting (`just test`)
2. **Make one change** at a time
3. **Run tests** after each change
4. **If tests break**: Was behavior preserved? If yes → fix test. If no → revert.

## Project Commands

```bash
just test              # Run all tests
just test-cover        # Tests with coverage report
just test-cover-check  # Verify 100% coverage of non-Impl functions
just check             # All quality gates before commit
just vet               # Static analysis
just lint              # Linting (staticcheck)
```

## Quality Gates (Enforced)

| Gate | Command | Requirement |
|------|---------|-------------|
| Coverage | `just test-cover-check` | 100% (excluding main) |
| Formatting | `just fmt-check` | gofmt compliant |
| Vet | `just vet` | Zero warnings |
| Lint | `just lint` | staticcheck clean |

### Run Gates Incrementally

After writing or modifying each file:
- Run `gofmt -w <file>` immediately after writing
- Run `go vet ./<package>/...` after each Red-Green cycle
- Run `just test-cover-check` after completing each unit (not just at the end)

Do NOT batch all quality checks to the end of a task.

## Quick Reference

**Test file naming**: `*_test.go` next to source
**Test function naming**: `Test<Type>_<Method>_<Scenario>`
**Error handling**: Always check errors, never use `_`
**Panics**: Never (except unavoidable in main)

## References

- [Detailed testing patterns](references/patterns.md) — Mocking, helpers, fixtures, edge cases
- [Coverage strategies](references/coverage.md) — Coverage guidelines and exemption patterns
