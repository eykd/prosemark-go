---
name: clean-architecture-validator
description: 'Use when: (1) reviewing code for architecture compliance, (2) finding dependency violations, (3) validating layer boundaries, (4) checking interface placement, (5) refactoring for better separation.'
---

# Clean Architecture Validator

Analyze code for Clean Architecture compliance focusing on the dependency rule: dependencies point inward, never outward.

## Layer Hierarchy (Inner to Outer)

```
Domain -> Application -> Infrastructure/Presentation
```

- **Domain**: Entities, Value Objects, Domain Services, Repository Interfaces
- **Application**: Use Cases, DTOs, Application Services
- **Infrastructure**: Repository Implementations, External APIs, Caches
- **Presentation**: Handlers, Controllers, Templates, UI

## The Dependency Rule

Inner layers must never depend on outer layers:

- Domain: Zero external dependencies (no framework imports, no infrastructure)
- Application: Depends only on Domain
- Infrastructure/Presentation: May depend on Application and Domain

## Analysis Workflow

1. **Map the codebase** - Identify layer boundaries from directory structure
2. **Scan imports** - Check each file's imports against allowed dependencies
3. **Classify violations** - Categorize by severity and type
4. **Report findings** - Present violations with refactoring suggestions

## Quick Violation Checks

**Domain layer violations** (most severe):

- Imports from `infrastructure/`, `presentation/`, framework packages
- Direct database/HTTP/file system calls
- Concrete repository implementations instead of interfaces

**Application layer violations**:

- Imports from `infrastructure/`, `presentation/`
- Direct infrastructure instantiation
- Framework-specific types in use case signatures

**Interface misplacement**:

- Repository interfaces in `infrastructure/` (should be in `domain/`)
- Port interfaces outside domain layer

## Output Format

```
## Architecture Violations Found

### Critical (Domain Layer)
- `internal/domain/entity/user.go:5` - Imports `database/sql` from infrastructure
  -> Move database logic to repository implementation

### Warning (Application Layer)
- `internal/application/usecase/create_user.go:12` - Instantiates `SQLUserRepository`
  -> Accept repository via constructor injection

### Info (Interface Placement)
- `internal/infra/user_repository.go` - Interface defined in infrastructure
  -> Move interface to `internal/domain/user.go`
```

## References

- **Detailed violation patterns**: See [references/violations.md](references/violations.md)
- **Layer rules and examples**: See [references/layer-rules.md](references/layer-rules.md)
