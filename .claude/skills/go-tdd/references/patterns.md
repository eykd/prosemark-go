# Detailed Testing Patterns

## Table of Contents
1. [Test Helpers](#test-helpers)
2. [Mocking Patterns](#mocking-patterns)
3. [Test Fixtures](#test-fixtures)
4. [Error Testing](#error-testing)
5. [Concurrent Testing](#concurrent-testing)
6. [CLI-Specific Patterns](#cli-specific-patterns)

## Test Helpers

### Assertion Helpers

```go
func assertNoError(t *testing.T, err error) {
    t.Helper()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

func assertEqual[T comparable](t *testing.T, got, want T) {
    t.Helper()
    if got != want {
        t.Errorf("got %v, want %v", got, want)
    }
}

func assertContains(t *testing.T, s, substr string) {
    t.Helper()
    if !strings.Contains(s, substr) {
        t.Errorf("%q does not contain %q", s, substr)
    }
}
```

### Package-Local Test Helpers

When you need a `Must*` helper (e.g., `mustParseMP`), define it as an unexported function in the test file. Do NOT assume the source package exports one — verify with grep first. Each test package may define its own local helpers.

### Setup/Teardown

```go
func TestWithCleanup(t *testing.T) {
    // Setup
    tmpDir := t.TempDir() // Auto-cleaned
    
    // Or manual cleanup
    resource := createResource()
    t.Cleanup(func() {
        resource.Close()
    })
    
    // Test code...
}
```

## Mocking Patterns

### Spy (Records Calls)

```go
type spyNotifier struct {
    calls []Notification
}

func (s *spyNotifier) Notify(n Notification) error {
    s.calls = append(s.calls, n)
    return nil
}

// In test
func TestService_Notifies(t *testing.T) {
    spy := &spyNotifier{}
    svc := NewService(WithNotifier(spy))
    
    svc.DoAction()
    
    if len(spy.calls) != 1 {
        t.Errorf("expected 1 notification, got %d", len(spy.calls))
    }
}
```

### Stub (Returns Canned Values)

```go
type stubRepo struct {
    data map[string]*Entity
}

func (s *stubRepo) Find(id string) (*Entity, error) {
    e, ok := s.data[id]
    if !ok {
        return nil, ErrNotFound
    }
    return e, nil
}
```

### Fake (Working Implementation)

```go
type inMemoryStore struct {
    data map[string][]byte
    mu   sync.Mutex
}

func (s *inMemoryStore) Get(key string) ([]byte, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    v, ok := s.data[key]
    if !ok {
        return nil, ErrNotFound
    }
    return v, nil
}

func (s *inMemoryStore) Set(key string, value []byte) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.data[key] = value
    return nil
}
```

### Error Injection

```go
type errorOnNthCall struct {
    callCount int
    failOn    int
    err       error
}

func (e *errorOnNthCall) Do() error {
    e.callCount++
    if e.callCount == e.failOn {
        return e.err
    }
    return nil
}
```

## Test Fixtures

### Test Data Builder

```go
type ConfigBuilder struct {
    config Config
}

func NewConfigBuilder() *ConfigBuilder {
    return &ConfigBuilder{
        config: Config{
            Timeout: 30 * time.Second, // sensible default
            Retries: 3,
        },
    }
}

func (b *ConfigBuilder) WithTimeout(d time.Duration) *ConfigBuilder {
    b.config.Timeout = d
    return b
}

func (b *ConfigBuilder) Build() Config {
    return b.config
}

// Usage
cfg := NewConfigBuilder().WithTimeout(5 * time.Second).Build()
```

### Golden Files

```go
var update = flag.Bool("update", false, "update golden files")

func TestOutput(t *testing.T) {
    got := GenerateOutput()
    golden := filepath.Join("testdata", t.Name()+".golden")
    
    if *update {
        os.WriteFile(golden, []byte(got), 0644)
        return
    }
    
    want, _ := os.ReadFile(golden)
    if got != string(want) {
        t.Errorf("output differs from golden file")
    }
}
```

## Error Testing

### Testing Error Types

```go
func TestService_ReturnsNotFoundError(t *testing.T) {
    svc := NewService(&emptyRepo{})
    
    _, err := svc.Get("missing")
    
    // Check error type
    var notFound *NotFoundError
    if !errors.As(err, &notFound) {
        t.Errorf("error type = %T, want *NotFoundError", err)
    }
    
    // Check sentinel error
    if !errors.Is(err, ErrNotFound) {
        t.Errorf("error = %v, want ErrNotFound", err)
    }
}
```

### Testing Error Messages

```go
func TestValidation_ErrorMessage(t *testing.T) {
    err := Validate(invalidInput)
    
    if err == nil {
        t.Fatal("expected error")
    }
    if !strings.Contains(err.Error(), "invalid") {
        t.Errorf("error = %q, want to contain 'invalid'", err)
    }
}
```

## Concurrent Testing

### Race Detection

```bash
go test -race ./...
```

### Testing Thread Safety

```go
func TestCounter_Concurrent(t *testing.T) {
    counter := NewCounter()
    const n = 1000
    
    var wg sync.WaitGroup
    wg.Add(n)
    
    for i := 0; i < n; i++ {
        go func() {
            defer wg.Done()
            counter.Increment()
        }()
    }
    
    wg.Wait()
    
    if counter.Value() != n {
        t.Errorf("Value() = %d, want %d", counter.Value(), n)
    }
}
```

## CLI-Specific Patterns

### Testing Subcommands

```go
func TestInitCmd(t *testing.T) {
    root := NewRootCmd()
    root.AddCommand(NewInitCmd())
    
    buf := new(bytes.Buffer)
    root.SetOut(buf)
    root.SetArgs([]string{"init", "--name", "myproject"})
    
    err := root.Execute()
    
    assertNoError(t, err)
    assertContains(t, buf.String(), "Initialized myproject")
}
```

### Testing Flags

```go
func TestCmd_Flags(t *testing.T) {
    tests := []struct {
        name    string
        args    []string
        wantErr bool
    }{
        {"valid flags", []string{"--config", "test.yaml"}, false},
        {"missing required", []string{}, true},
        {"unknown flag", []string{"--unknown"}, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cmd := NewCmd()
            cmd.SetArgs(tt.args)
            
            err := cmd.Execute()
            
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Testing Exit Codes

```go
func TestCmd_ExitCode(t *testing.T) {
    // Capture os.Exit by using a wrapper pattern
    cmd := NewRootCmd()
    cmd.SetArgs([]string{"invalid"})
    
    err := cmd.Execute()
    
    // Cobra wraps errors, check underlying
    if err == nil {
        t.Error("expected error for invalid command")
    }
}
```

### Testing Interactive Input

```go
func TestCmd_Prompt(t *testing.T) {
    in := strings.NewReader("yes\n")
    out := new(bytes.Buffer)
    
    cmd := NewCmd()
    cmd.SetIn(in)
    cmd.SetOut(out)
    cmd.SetArgs([]string{"delete", "--confirm"})
    
    err := cmd.Execute()
    
    assertNoError(t, err)
}
```
