# Domain Layer

Pure business logic. No dependencies on outer layers.

## Location
`internal/domain/<bounded-context>/`

## Entities

Objects with identity that persists across state changes.

```go
// internal/domain/project/project.go
package project

type ID string

type Project struct {
    id        ID
    name      Name
    status    Status
    createdAt time.Time
}

// Constructor with options pattern
func NewProject(name Name, opts ...Option) *Project {
    p := &Project{
        id:        ID(uuid.New().String()),
        name:      name,
        status:    StatusActive,
        createdAt: time.Now(),
    }
    for _, opt := range opts {
        opt(p)
    }
    return p
}

type Option func(*Project)

func WithID(id ID) Option {
    return func(p *Project) { p.id = id }
}

func WithCreatedAt(t time.Time) Option {
    return func(p *Project) { p.createdAt = t }
}

// Behavior methods - enforce invariants
func (p *Project) Rename(newName Name) error {
    if p.status == StatusArchived {
        return ErrCannotModifyArchived
    }
    p.name = newName
    return nil
}

func (p *Project) Archive() error {
    if p.status == StatusArchived {
        return ErrAlreadyArchived
    }
    p.status = StatusArchived
    return nil
}

// Query methods - read-only
func (p *Project) ID() ID            { return p.id }
func (p *Project) Name() Name        { return p.name }
func (p *Project) IsActive() bool    { return p.status == StatusActive }
```

### Testing Entities
```go
func TestProject_Rename_ActiveProject(t *testing.T) {
    name, _ := project.NewName("old")
    p := project.NewProject(name)
    newName, _ := project.NewName("new")
    
    err := p.Rename(newName)
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if p.Name() != newName {
        t.Errorf("Name() = %v, want %v", p.Name(), newName)
    }
}

func TestProject_Rename_ArchivedProject(t *testing.T) {
    name, _ := project.NewName("test")
    p := project.NewProject(name)
    p.Archive()
    newName, _ := project.NewName("new")
    
    err := p.Rename(newName)
    
    if !errors.Is(err, project.ErrCannotModifyArchived) {
        t.Errorf("error = %v, want ErrCannotModifyArchived", err)
    }
}
```

## Value Objects

Immutable. Defined by attributes, not identity. Carry validation.

```go
// internal/domain/project/name.go
package project

type Name string

func NewName(s string) (Name, error) {
    s = strings.TrimSpace(s)
    if s == "" {
        return "", ErrEmptyName
    }
    if len(s) > 100 {
        return "", ErrNameTooLong
    }
    if !validNamePattern.MatchString(s) {
        return "", ErrInvalidNameFormat
    }
    return Name(s), nil
}

func (n Name) String() string { return string(n) }
func (n Name) IsEmpty() bool  { return n == "" }
```

```go
// internal/domain/deployment/version.go
package deployment

var versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9]+)?$`)

type Version string

func NewVersion(s string) (Version, error) {
    if !versionPattern.MatchString(s) {
        return "", fmt.Errorf("invalid version format: %s", s)
    }
    return Version(s), nil
}

func (v Version) String() string    { return string(v) }
func (v Version) IsPrerelease() bool { return strings.Contains(string(v), "-") }
```

### Testing Value Objects
```go
func TestNewName(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Name
        wantErr error
    }{
        {"valid", "my-project", Name("my-project"), nil},
        {"trims whitespace", "  spaced  ", Name("spaced"), nil},
        {"empty", "", "", ErrEmptyName},
        {"only whitespace", "   ", "", ErrEmptyName},
        {"too long", string(make([]byte, 101)), "", ErrNameTooLong},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := NewName(tt.input)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("error = %v, want %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Aggregates

Cluster of entities/value objects with a root that enforces invariants.

```go
// internal/domain/deployment/release.go
package deployment

type Release struct {
    id        ReleaseID
    projectID ProjectID
    version   Version
    artifacts []Artifact
    status    Status
}

// Root controls all modifications
func (r *Release) AddArtifact(a Artifact) error {
    if r.status != StatusDraft {
        return ErrCannotModifyAfterSubmit
    }
    // Invariant: no duplicate artifacts
    for _, existing := range r.artifacts {
        if existing.Name == a.Name {
            return ErrDuplicateArtifact
        }
    }
    r.artifacts = append(r.artifacts, a)
    return nil
}

func (r *Release) Submit() error {
    if r.status != StatusDraft {
        return ErrInvalidStatusTransition
    }
    if len(r.artifacts) == 0 {
        return ErrNoArtifacts
    }
    r.status = StatusPending
    return nil
}
```

## Repository Interfaces

Defined in domain, implemented in infrastructure.

```go
// internal/domain/project/repository.go
package project

import "context"

type Repository interface {
    FindByID(ctx context.Context, id ID) (*Project, error)
    FindByName(ctx context.Context, name Name) (*Project, error)
    Save(ctx context.Context, p *Project) error
    Delete(ctx context.Context, id ID) error
    List(ctx context.Context) ([]*Project, error)
}
```

## Domain Errors

```go
// internal/domain/project/errors.go
package project

import "errors"

var (
    ErrEmptyName            = errors.New("project name cannot be empty")
    ErrNameTooLong          = errors.New("project name exceeds maximum length")
    ErrInvalidNameFormat    = errors.New("project name contains invalid characters")
    ErrNotFound             = errors.New("project not found")
    ErrCannotModifyArchived = errors.New("cannot modify archived project")
    ErrAlreadyArchived      = errors.New("project is already archived")
)
```

## Domain Services

Operations spanning multiple entities.

```go
// internal/domain/deployment/deployer.go
package deployment

import "context"

type Deployer interface {
    Deploy(ctx context.Context, release *Release, env Environment) error
}
```
