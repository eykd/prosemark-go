---
name: go-cli-ddd
description: Build Go CLI applications using Domain-Driven Design with TDD. Use when creating CLI tools with Cobra that need clean architecture, business logic separation, or testable layered design. Triggers on requests involving Go CLI structure, domain modeling for CLIs, separating business logic from commands, or achieving 100% test coverage in CLI apps.
---

# Go CLI with Domain-Driven Design

## Decision Tree

**What do you need?**
├── New CLI feature → § Outside-In Workflow
├── Domain modeling → [domain-layer.md](references/domain-layer.md)
├── Application services → [application-layer.md](references/application-layer.md)
├── Repository/adapter → [infrastructure-layer.md](references/infrastructure-layer.md)
├── Cobra command → [cli-layer.md](references/cli-layer.md)
└── Advanced patterns → [patterns.md](references/patterns.md)

## Architecture

Dependencies flow inward. Domain knows nothing about outer layers.

```
CLI (cmd/)
    ↓ calls
Application (internal/app/)
    ↓ uses
Domain (internal/domain/)
    ↑ implements
Infrastructure (internal/infra/)
```

### Project Structure

```
project/
├── cmd/                      # Cobra commands (thin adapters)
│   ├── root.go
│   └── <command>.go
├── internal/
│   ├── domain/<context>/     # Entities, value objects, interfaces
│   ├── app/                  # Application services (use cases)
│   └── infra/
│       ├── repository/       # Storage implementations
│       └── api/              # External API clients
├── test/acceptance/          # End-to-end command tests
└── main.go
```

## Layer Summary

| Layer | Contains | Tests With | Mocks |
|-------|----------|------------|-------|
| Domain | Entities, value objects, repository interfaces | Unit tests | Nothing (pure) |
| Application | Services orchestrating use cases | Unit tests | Infrastructure |
| Infrastructure | DB repos, API clients, file I/O | Integration tests | External systems |
| CLI | Cobra commands, flag parsing, output | Unit + acceptance | Application services |

## Outside-In Workflow

Build features as vertical slices through all layers:

### 1. Acceptance Test (test/acceptance/)
```go
func TestCommand_Feature(t *testing.T) {
    tmpDir := t.TempDir()
    cmd := exec.Command(binaryPath(t), "command", "arg")
    cmd.Env = append(os.Environ(), "CONFIG_DIR="+tmpDir)
    output, err := cmd.CombinedOutput()
    
    // Assert command succeeds
    if err != nil {
        t.Fatalf("command failed: %v\noutput: %s", err, output)
    }
    // Assert output/side effects
}
```

### 2. Application Service (internal/app/)
```go
func TestService_UseCase(t *testing.T) {
    repo := &mockRepository{...}
    svc := NewService(WithRepository(repo))
    
    result, err := svc.DoSomething(ctx, input)
    
    // Assert behavior
}
```

### 3. Domain Objects (internal/domain/)
```go
func TestEntity_Behavior(t *testing.T) {
    entity := NewEntity(validInput)
    
    err := entity.DoAction()
    
    // Assert state change or error
}
```

### 4. Infrastructure (internal/infra/)
```go
func TestRepository_Persistence(t *testing.T) {
    tmpDir := t.TempDir()
    repo := NewFileRepository(tmpDir)
    
    // Save and retrieve
}
```

### 5. CLI Command (cmd/)
```go
func TestCommand_ParsesFlags(t *testing.T) {
    mockSvc := &mockService{...}
    // Inject mock, execute command, assert calls
}
```

### 6. Wire and Run Acceptance Test

## Quick Patterns

### Value Object with Validation
```go
type Name string

func NewName(s string) (Name, error) {
    s = strings.TrimSpace(s)
    if s == "" {
        return "", ErrEmptyName
    }
    return Name(s), nil
}
```

### Repository Interface (domain defines, infra implements)
```go
// internal/domain/project/repository.go
type Repository interface {
    FindByID(ctx context.Context, id ID) (*Project, error)
    Save(ctx context.Context, p *Project) error
}
```

### Application Service with Options
```go
type Service struct {
    repo Repository
}

type Option func(*Service)

func WithRepository(r Repository) Option {
    return func(s *Service) { s.repo = r }
}

func NewService(opts ...Option) *Service {
    s := &Service{}
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

### Thin Cobra Command
```go
func runCommand(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()
    svc := getService() // from container
    
    result, err := svc.DoSomething(ctx, args[0])
    if err != nil {
        return formatError(err)
    }
    
    fmt.Fprintf(cmd.OutOrStdout(), "Done: %s\n", result)
    return nil
}
```

## Import Rules

```go
// ✅ Allowed
app → domain
infra → domain (implements interfaces)
cmd → app

// ❌ Forbidden
domain → infra
domain → app
domain → cmd
```

## References

| Topic | File |
|-------|------|
| Entities, value objects, aggregates | [domain-layer.md](references/domain-layer.md) |
| Application services, use cases | [application-layer.md](references/application-layer.md) |
| Repositories, API clients | [infrastructure-layer.md](references/infrastructure-layer.md) |
| Cobra commands, testing | [cli-layer.md](references/cli-layer.md) |
| Events, errors, DI, context | [patterns.md](references/patterns.md) |
