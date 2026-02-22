# Binding Patterns

## Architecture

The acceptance pipeline generates test stubs in `generated-acceptance-tests/`.
Edit generated files directly. The pipeline preserves functions that don't
contain the unbound sentinel (`t.Fatal("acceptance test not yet bound")`).

### Stub → Bound Test Workflow

1. Run `just acceptance-generate` to create stubs from specs (or let the pipeline run)
2. Read the generated stub to see scenario structure and step comments
3. Edit the generated file directly: replace the `t.Fatal(...)` sentinel with real test code
4. Add necessary imports (`"os"`, `"os/exec"`, `"path/filepath"`, `"strings"`)
5. The pipeline preserves your bound implementations on subsequent runs
6. Use `just acceptance-regen` to force-regenerate all stubs if needed (destroys bound code)

## CLI Execution Pattern

All acceptance tests exercise the CLI as a black box. The user interacts with
`lmk` via the command line, so tests must do the same.

```go
func Test_User_creates_new_project(t *testing.T) {
    // GIVEN a directory with no project
    dir := t.TempDir()

    // WHEN the user runs lmk init
    cmd := exec.Command("go", "run", ".", "init")
    cmd.Dir = dir
    output, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("lmk init failed: %v\n%s", err, output)
    }

    // THEN the output confirms the project was created
    if !strings.Contains(string(output), "Project created") {
        t.Errorf("expected creation confirmation, got: %s", output)
    }
}
```

Key points:
- `exec.Command("go", "run", ".", subcommand, args...)` — runs lmk from source
- `cmd.Dir = dir` — sets working directory to the test's temp dir
- `cmd.CombinedOutput()` — captures both stdout and stderr
- Always check `err` and include output in failure message for debugging

## File System Setup Patterns

### Simple: Single File

```go
dir := t.TempDir()
content := []byte("# Chapter One\n\nSome prose.\n")
if err := os.WriteFile(filepath.Join(dir, "chapter-one.md"), content, 0644); err != nil {
    t.Fatal(err)
}
```

### Multi-File Project

```go
dir := t.TempDir()

files := map[string]string{
    "chapter-01.md": "# Introduction\n\nOpening text.\n",
    "chapter-02.md": "# Rising Action\n\nConflict begins.\n",
    "chapter-03.md": "# Climax\n\nThe peak.\n",
}

for name, content := range files {
    path := filepath.Join(dir, name)
    if err := os.WriteFile(path, []byte(content), 0644); err != nil {
        t.Fatal(err)
    }
}
```

### Nested Directory Structure

```go
dir := t.TempDir()

subdir := filepath.Join(dir, "chapters")
if err := os.MkdirAll(subdir, 0755); err != nil {
    t.Fatal(err)
}
// Then write files into subdir
```

## Output Assertion Patterns

### String Contains (most common)

```go
got := string(output)
if !strings.Contains(got, "3 chapters") {
    t.Errorf("expected chapter count, got: %s", got)
}
```

### Multiple Assertions on One Output

```go
got := string(output)

checks := []string{
    "chapter-01",
    "chapter-02",
    "chapter-03",
}
for _, want := range checks {
    if !strings.Contains(got, want) {
        t.Errorf("expected %q in output, got: %s", want, got)
    }
}
```

### Order-Sensitive Assertions

```go
got := string(output)
idx1 := strings.Index(got, "Introduction")
idx2 := strings.Index(got, "Rising Action")
if idx1 == -1 || idx2 == -1 {
    t.Fatalf("missing expected sections in output: %s", got)
}
if idx1 >= idx2 {
    t.Errorf("expected Introduction before Rising Action, got: %s", got)
}
```

### Error Case Assertions

```go
cmd := exec.Command("go", "run", ".", "status")
cmd.Dir = t.TempDir() // empty dir, no project
output, err := cmd.CombinedOutput()

if err == nil {
    t.Fatal("expected command to fail, but it succeeded")
}
if !strings.Contains(string(output), "no project") {
    t.Errorf("expected error about missing project, got: %s", output)
}
```

## Multi-Step Scenario Pattern

Some scenarios have multiple GIVEN steps requiring sequential setup,
or WHEN steps that build on prior state.

```go
func Test_User_adds_chapter_to_existing_project(t *testing.T) {
    dir := t.TempDir()

    // GIVEN a project exists
    cmd := exec.Command("go", "run", ".", "init")
    cmd.Dir = dir
    if out, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("init failed: %v\n%s", err, out)
    }

    // GIVEN the project has one chapter
    content := []byte("# First Draft\n\nSome text.\n")
    if err := os.WriteFile(filepath.Join(dir, "draft.md"), content, 0644); err != nil {
        t.Fatal(err)
    }

    // WHEN the user runs lmk status
    cmd = exec.Command("go", "run", ".", "status")
    cmd.Dir = dir
    output, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("status failed: %v\n%s", err, output)
    }

    // THEN the output lists one chapter
    if !strings.Contains(string(output), "1 chapter") {
        t.Errorf("expected one chapter in output, got: %s", output)
    }
}
```

Each GIVEN step builds on the previous one. If a setup step fails,
`t.Fatalf` stops the test immediately with a clear message.
