# Application Layer

Orchestrates domain objects. Defines use cases. Depends on domain interfaces, not implementations.

## Location
`internal/app/` or `internal/service/`

## Application Service Pattern

```go
// internal/app/project_service.go
package app

import (
    "context"
    "errors"
    
    "myproject/internal/domain/project"
)

type ProjectService struct {
    repo project.Repository
}

type ProjectServiceOption func(*ProjectService)

func NewProjectService(opts ...ProjectServiceOption) *ProjectService {
    svc := &ProjectService{}
    for _, opt := range opts {
        opt(svc)
    }
    return svc
}

func WithProjectRepository(repo project.Repository) ProjectServiceOption {
    return func(s *ProjectService) { s.repo = repo }
}

// Use case: Initialize a new project
func (s *ProjectService) InitializeProject(ctx context.Context, nameStr string) (*project.Project, error) {
    // 1. Validate input (create value object)
    name, err := project.NewName(nameStr)
    if err != nil {
        return nil, err
    }
    
    // 2. Check business rules
    existing, err := s.repo.FindByName(ctx, name)
    if err != nil && !errors.Is(err, project.ErrNotFound) {
        return nil, err
    }
    if existing != nil {
        return nil, ErrProjectAlreadyExists
    }
    
    // 3. Create domain object
    p := project.NewProject(name)
    
    // 4. Persist
    if err := s.repo.Save(ctx, p); err != nil {
        return nil, err
    }
    
    return p, nil
}

// Use case: Rename a project
func (s *ProjectService) RenameProject(ctx context.Context, id, newNameStr string) (*project.Project, error) {
    newName, err := project.NewName(newNameStr)
    if err != nil {
        return nil, err
    }
    
    p, err := s.repo.FindByID(ctx, project.ID(id))
    if err != nil {
        return nil, err
    }
    
    if err := p.Rename(newName); err != nil {
        return nil, err
    }
    
    if err := s.repo.Save(ctx, p); err != nil {
        return nil, err
    }
    
    return p, nil
}

// Use case: List all projects
func (s *ProjectService) ListProjects(ctx context.Context) ([]*project.Project, error) {
    return s.repo.List(ctx)
}
```

## Application-Level Errors

```go
// internal/app/errors.go
package app

import "fmt"

var (
    ErrProjectAlreadyExists = errors.New("project already exists")
)

// Typed errors for CLI layer to handle
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

type NotFoundError struct {
    Resource string
    ID       string
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

type ConflictError struct {
    Resource string
    Message  string
}

func (e *ConflictError) Error() string {
    return fmt.Sprintf("conflict: %s - %s", e.Resource, e.Message)
}
```

## Testing Application Services

Mock repository interfaces to test orchestration logic.

```go
// internal/app/project_service_test.go
package app

import (
    "context"
    "errors"
    "testing"
    
    "myproject/internal/domain/project"
)

func TestProjectService_InitializeProject_Success(t *testing.T) {
    repo := newMockProjectRepository()
    svc := NewProjectService(WithProjectRepository(repo))
    ctx := context.Background()
    
    p, err := svc.InitializeProject(ctx, "my-project")
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if p.Name().String() != "my-project" {
        t.Errorf("Name() = %q, want %q", p.Name(), "my-project")
    }
    if len(repo.saved) != 1 {
        t.Errorf("expected 1 save call, got %d", len(repo.saved))
    }
}

func TestProjectService_InitializeProject_EmptyName(t *testing.T) {
    repo := newMockProjectRepository()
    svc := NewProjectService(WithProjectRepository(repo))
    
    _, err := svc.InitializeProject(context.Background(), "")
    
    if !errors.Is(err, project.ErrEmptyName) {
        t.Errorf("error = %v, want ErrEmptyName", err)
    }
}

func TestProjectService_InitializeProject_Duplicate(t *testing.T) {
    repo := newMockProjectRepository()
    name, _ := project.NewName("existing")
    repo.existing[name.String()] = project.NewProject(name)
    svc := NewProjectService(WithProjectRepository(repo))
    
    _, err := svc.InitializeProject(context.Background(), "existing")
    
    if !errors.Is(err, ErrProjectAlreadyExists) {
        t.Errorf("error = %v, want ErrProjectAlreadyExists", err)
    }
}

// Mock repository
type mockProjectRepository struct {
    existing map[string]*project.Project
    saved    []*project.Project
}

func newMockProjectRepository() *mockProjectRepository {
    return &mockProjectRepository{
        existing: make(map[string]*project.Project),
    }
}

func (m *mockProjectRepository) FindByID(ctx context.Context, id project.ID) (*project.Project, error) {
    for _, p := range m.existing {
        if p.ID() == id {
            return p, nil
        }
    }
    return nil, project.ErrNotFound
}

func (m *mockProjectRepository) FindByName(ctx context.Context, name project.Name) (*project.Project, error) {
    p, ok := m.existing[name.String()]
    if !ok {
        return nil, project.ErrNotFound
    }
    return p, nil
}

func (m *mockProjectRepository) Save(ctx context.Context, p *project.Project) error {
    m.saved = append(m.saved, p)
    m.existing[p.Name().String()] = p
    return nil
}

func (m *mockProjectRepository) Delete(ctx context.Context, id project.ID) error {
    return nil
}

func (m *mockProjectRepository) List(ctx context.Context) ([]*project.Project, error) {
    var result []*project.Project
    for _, p := range m.existing {
        result = append(result, p)
    }
    return result, nil
}
```

## Service with Multiple Dependencies

```go
// internal/app/deployment_service.go
type DeploymentService struct {
    projectRepo project.Repository
    releaseRepo deployment.ReleaseRepository
    deployer    deployment.Deployer
    notifier    Notifier
}

func NewDeploymentService(opts ...DeploymentServiceOption) *DeploymentService {
    svc := &DeploymentService{}
    for _, opt := range opts {
        opt(svc)
    }
    return svc
}

func WithReleaseRepository(r deployment.ReleaseRepository) DeploymentServiceOption {
    return func(s *DeploymentService) { s.releaseRepo = r }
}

func WithDeployer(d deployment.Deployer) DeploymentServiceOption {
    return func(s *DeploymentService) { s.deployer = d }
}

func WithNotifier(n Notifier) DeploymentServiceOption {
    return func(s *DeploymentService) { s.notifier = n }
}

func (s *DeploymentService) Deploy(ctx context.Context, releaseID, envStr string) error {
    release, err := s.releaseRepo.FindByID(ctx, deployment.ReleaseID(releaseID))
    if err != nil {
        return &NotFoundError{Resource: "release", ID: releaseID}
    }
    
    env, err := deployment.NewEnvironment(envStr)
    if err != nil {
        return &ValidationError{Field: "environment", Message: err.Error()}
    }
    
    if err := release.StartDeployment(); err != nil {
        return &ConflictError{Resource: "release", Message: err.Error()}
    }
    
    if err := s.releaseRepo.Save(ctx, release); err != nil {
        return err
    }
    
    if err := s.deployer.Deploy(ctx, release, env); err != nil {
        release.FailDeployment()
        s.releaseRepo.Save(ctx, release)
        s.notifier.NotifyFailure(ctx, release, err)
        return err
    }
    
    release.CompleteDeployment()
    if err := s.releaseRepo.Save(ctx, release); err != nil {
        return err
    }
    
    s.notifier.NotifySuccess(ctx, release)
    return nil
}
```

## Interface Definitions for Side Effects

```go
// internal/app/notifier.go
package app

import "context"

type Notifier interface {
    NotifySuccess(ctx context.Context, release *deployment.Release)
    NotifyFailure(ctx context.Context, release *deployment.Release, err error)
}
```
