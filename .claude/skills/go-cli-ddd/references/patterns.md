# Advanced Patterns

## Dependency Injection Container

Wire dependencies at application startup.

```go
// internal/bootstrap/container.go
package bootstrap

import (
    "database/sql"
    "os"
    
    "myproject/internal/app"
    "myproject/internal/infra/api"
    "myproject/internal/infra/repository"
)

type Container struct {
    ProjectService    *app.ProjectService
    DeploymentService *app.DeploymentService
}

type Config struct {
    ConfigDir   string
    DatabaseURL string
    APIBaseURL  string
    APIKey      string
}

func LoadConfig() Config {
    return Config{
        ConfigDir:   getEnvOrDefault("TB_CONFIG_DIR", defaultConfigDir()),
        DatabaseURL: os.Getenv("TB_DATABASE_URL"),
        APIBaseURL:  getEnvOrDefault("TB_API_URL", "https://api.example.com"),
        APIKey:      os.Getenv("TB_API_KEY"),
    }
}

func NewContainer(cfg Config) (*Container, error) {
    // Choose repository implementation
    var projectRepo project.Repository
    if cfg.DatabaseURL != "" {
        db, err := sql.Open("postgres", cfg.DatabaseURL)
        if err != nil {
            return nil, err
        }
        projectRepo = repository.NewPostgresProjectRepository(db)
    } else {
        projectRepo = repository.NewFileProjectRepository(cfg.ConfigDir)
    }
    
    // External services
    deployer := api.NewTurtlebasedClient(cfg.APIBaseURL, cfg.APIKey)
    
    // Wire services
    projectSvc := app.NewProjectService(
        app.WithProjectRepository(projectRepo),
    )
    
    deploymentSvc := app.NewDeploymentService(
        app.WithDeployer(deployer),
    )
    
    return &Container{
        ProjectService:    projectSvc,
        DeploymentService: deploymentSvc,
    }, nil
}

func getEnvOrDefault(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}
```

## Main Entry Point

```go
// main.go
package main

import (
    "os"
    
    "myproject/cmd"
    "myproject/internal/bootstrap"
)

func main() {
    if err := run(); err != nil {
        os.Exit(1)
    }
}

func run() error {
    cfg := bootstrap.LoadConfig()
    container, err := bootstrap.NewContainer(cfg)
    if err != nil {
        return err
    }
    
    cmd.SetContainer(container)
    return cmd.Execute()
}
```

## Context Propagation

Thread context for cancellation and timeouts.

```go
// cmd/deploy.go
func runDeploy(cmd *cobra.Command, args []string) error {
    ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
    defer cancel()
    
    return svc.Deploy(ctx, releaseID, environment)
}

// internal/app/deployment_service.go
func (s *DeploymentService) Deploy(ctx context.Context, releaseID, env string) error {
    // Check cancellation at each step
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    
    release, err := s.releaseRepo.FindByID(ctx, deployment.ReleaseID(releaseID))
    // ... pass ctx to all operations
}
```

## Domain Events

Decouple side effects from core logic.

```go
// internal/domain/deployment/events.go
package deployment

type Event interface {
    ReleaseID() ReleaseID
}

type ReleaseDeployed struct {
    releaseID   ReleaseID
    environment Environment
    deployedAt  time.Time
}

func (e ReleaseDeployed) ReleaseID() ReleaseID { return e.releaseID }

type ReleaseDeploymentFailed struct {
    releaseID ReleaseID
    reason    string
}

func (e ReleaseDeploymentFailed) ReleaseID() ReleaseID { return e.releaseID }
```

```go
// internal/domain/deployment/release.go
type Release struct {
    // ... fields
    events []Event
}

func (r *Release) CompleteDeployment(env Environment) error {
    if r.status != StatusDeploying {
        return ErrInvalidStatusTransition
    }
    r.status = StatusDeployed
    now := time.Now()
    r.deployedAt = &now
    
    // Record event
    r.events = append(r.events, ReleaseDeployed{
        releaseID:   r.id,
        environment: env,
        deployedAt:  now,
    })
    return nil
}

func (r *Release) PopEvents() []Event {
    events := r.events
    r.events = nil
    return events
}
```

```go
// internal/app/deployment_service.go
func (s *DeploymentService) Deploy(ctx context.Context, releaseID, env string) error {
    // ... deployment logic ...
    
    release.CompleteDeployment(environment)
    
    if err := s.releaseRepo.Save(ctx, release); err != nil {
        return err
    }
    
    // Publish events after successful save
    for _, event := range release.PopEvents() {
        s.eventPublisher.Publish(ctx, event)
    }
    
    return nil
}
```

## Typed Error Handling

```go
// internal/app/errors.go
package app

type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation: %s - %s", e.Field, e.Message)
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
    return fmt.Sprintf("conflict: %s", e.Message)
}
```

```go
// cmd/errors.go
func formatError(err error) error {
    var notFound *app.NotFoundError
    var validation *app.ValidationError
    var conflict *app.ConflictError
    
    switch {
    case errors.As(err, &notFound):
        return fmt.Errorf("%s '%s' not found", notFound.Resource, notFound.ID)
    case errors.As(err, &validation):
        return fmt.Errorf("invalid %s: %s", validation.Field, validation.Message)
    case errors.As(err, &conflict):
        return fmt.Errorf("cannot proceed: %s", conflict.Message)
    default:
        return err
    }
}
```

## Feature Flags

```go
// internal/domain/feature/flags.go
package feature

import "context"

type Flags interface {
    IsEnabled(ctx context.Context, flag string) bool
}

// internal/infra/feature/env_flags.go
type EnvFlags struct{}

func (f *EnvFlags) IsEnabled(ctx context.Context, flag string) bool {
    return os.Getenv("FEATURE_"+strings.ToUpper(flag)) == "true"
}

// Usage in service
func (s *DeploymentService) Deploy(ctx context.Context, releaseID, env string) error {
    if s.features.IsEnabled(ctx, "parallel_deploy") {
        return s.deployParallel(ctx, release, environment)
    }
    return s.deploySequential(ctx, release, environment)
}
```

## Test Builders

Create complex test objects fluently.

```go
// internal/domain/deployment/testing.go (or test file)
type ReleaseBuilder struct {
    release *Release
}

func NewReleaseBuilder() *ReleaseBuilder {
    version, _ := NewVersion("1.0.0")
    return &ReleaseBuilder{
        release: NewRelease(ProjectID("proj-1"), version),
    }
}

func (b *ReleaseBuilder) WithVersion(v string) *ReleaseBuilder {
    version, _ := NewVersion(v)
    b.release.version = version
    return b
}

func (b *ReleaseBuilder) WithStatus(s Status) *ReleaseBuilder {
    b.release.status = s
    return b
}

func (b *ReleaseBuilder) WithArtifact(name string) *ReleaseBuilder {
    b.release.artifacts = append(b.release.artifacts, Artifact{Name: name})
    return b
}

func (b *ReleaseBuilder) Submitted() *ReleaseBuilder {
    b.release.AddArtifact(Artifact{Name: "app.jar"})
    b.release.Submit()
    return b
}

func (b *ReleaseBuilder) Build() *Release {
    return b.release
}

// Usage
func TestDeployment(t *testing.T) {
    release := NewReleaseBuilder().
        WithVersion("2.0.0").
        Submitted().
        Build()
    // ...
}
```

## In-Memory Fakes for Testing

Full working implementations for tests.

```go
// internal/infra/repository/inmemory_project_repo.go
type InMemoryProjectRepository struct {
    projects map[project.ID]*project.Project
    mu       sync.RWMutex
}

func NewInMemoryProjectRepository() *InMemoryProjectRepository {
    return &InMemoryProjectRepository{
        projects: make(map[project.ID]*project.Project),
    }
}

func (r *InMemoryProjectRepository) Save(ctx context.Context, p *project.Project) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.projects[p.ID()] = p
    return nil
}

func (r *InMemoryProjectRepository) FindByID(ctx context.Context, id project.ID) (*project.Project, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    p, ok := r.projects[id]
    if !ok {
        return nil, project.ErrNotFound
    }
    return p, nil
}

func (r *InMemoryProjectRepository) FindByName(ctx context.Context, name project.Name) (*project.Project, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    for _, p := range r.projects {
        if p.Name() == name {
            return p, nil
        }
    }
    return nil, project.ErrNotFound
}

func (r *InMemoryProjectRepository) List(ctx context.Context) ([]*project.Project, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    result := make([]*project.Project, 0, len(r.projects))
    for _, p := range r.projects {
        result = append(result, p)
    }
    return result, nil
}

func (r *InMemoryProjectRepository) Delete(ctx context.Context, id project.ID) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.projects, id)
    return nil
}
```

## Spy for Verification

```go
type SpyNotifier struct {
    SuccessCalls []NotifySuccessCall
    FailureCalls []NotifyFailureCall
}

type NotifySuccessCall struct {
    Release *deployment.Release
}

type NotifyFailureCall struct {
    Release *deployment.Release
    Err     error
}

func (s *SpyNotifier) NotifySuccess(ctx context.Context, r *deployment.Release) {
    s.SuccessCalls = append(s.SuccessCalls, NotifySuccessCall{Release: r})
}

func (s *SpyNotifier) NotifyFailure(ctx context.Context, r *deployment.Release, err error) {
    s.FailureCalls = append(s.FailureCalls, NotifyFailureCall{Release: r, Err: err})
}

// Usage
func TestDeployment_NotifiesOnSuccess(t *testing.T) {
    notifier := &SpyNotifier{}
    svc := NewDeploymentService(WithNotifier(notifier))
    
    svc.Deploy(ctx, releaseID, "production")
    
    if len(notifier.SuccessCalls) != 1 {
        t.Errorf("expected 1 success notification, got %d", len(notifier.SuccessCalls))
    }
}
```
