# Infrastructure Layer

Implements domain interfaces. Handles technical concerns: storage, APIs, file I/O.

## Location
```
internal/infra/
├── repository/    # Storage implementations
└── api/           # External API clients
```

## File-Based Repository

```go
// internal/infra/repository/file_project_repo.go
package repository

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "time"
    
    "myproject/internal/domain/project"
)

type FileProjectRepository struct {
    baseDir string
}

func NewFileProjectRepository(baseDir string) *FileProjectRepository {
    return &FileProjectRepository{baseDir: baseDir}
}

// DTO for JSON serialization (internal to infra)
type projectDTO struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Status    string `json:"status"`
    CreatedAt string `json:"created_at"`
}

func (r *FileProjectRepository) projectsDir() string {
    return filepath.Join(r.baseDir, "projects")
}

func (r *FileProjectRepository) projectPath(name project.Name) string {
    return filepath.Join(r.projectsDir(), name.String()+".json")
}

func (r *FileProjectRepository) Save(ctx context.Context, p *project.Project) error {
    if err := os.MkdirAll(r.projectsDir(), 0755); err != nil {
        return err
    }
    
    dto := projectDTO{
        ID:        string(p.ID()),
        Name:      p.Name().String(),
        Status:    string(p.Status()),
        CreatedAt: p.CreatedAt().Format(time.RFC3339),
    }
    
    data, err := json.MarshalIndent(dto, "", "  ")
    if err != nil {
        return err
    }
    
    return os.WriteFile(r.projectPath(p.Name()), data, 0644)
}

func (r *FileProjectRepository) FindByName(ctx context.Context, name project.Name) (*project.Project, error) {
    data, err := os.ReadFile(r.projectPath(name))
    if os.IsNotExist(err) {
        return nil, project.ErrNotFound
    }
    if err != nil {
        return nil, err
    }
    
    var dto projectDTO
    if err := json.Unmarshal(data, &dto); err != nil {
        return nil, err
    }
    
    return r.dtoToProject(dto)
}

func (r *FileProjectRepository) FindByID(ctx context.Context, id project.ID) (*project.Project, error) {
    entries, err := os.ReadDir(r.projectsDir())
    if err != nil {
        if os.IsNotExist(err) {
            return nil, project.ErrNotFound
        }
        return nil, err
    }
    
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        data, err := os.ReadFile(filepath.Join(r.projectsDir(), entry.Name()))
        if err != nil {
            continue
        }
        var dto projectDTO
        if json.Unmarshal(data, &dto) == nil && dto.ID == string(id) {
            return r.dtoToProject(dto)
        }
    }
    return nil, project.ErrNotFound
}

func (r *FileProjectRepository) List(ctx context.Context) ([]*project.Project, error) {
    entries, err := os.ReadDir(r.projectsDir())
    if err != nil {
        if os.IsNotExist(err) {
            return []*project.Project{}, nil
        }
        return nil, err
    }
    
    var projects []*project.Project
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
            continue
        }
        data, err := os.ReadFile(filepath.Join(r.projectsDir(), entry.Name()))
        if err != nil {
            continue
        }
        var dto projectDTO
        if json.Unmarshal(data, &dto) == nil {
            p, _ := r.dtoToProject(dto)
            projects = append(projects, p)
        }
    }
    return projects, nil
}

func (r *FileProjectRepository) Delete(ctx context.Context, id project.ID) error {
    p, err := r.FindByID(ctx, id)
    if err != nil {
        return err
    }
    return os.Remove(r.projectPath(p.Name()))
}

func (r *FileProjectRepository) dtoToProject(dto projectDTO) (*project.Project, error) {
    name, _ := project.NewName(dto.Name)
    createdAt, _ := time.Parse(time.RFC3339, dto.CreatedAt)
    
    return project.NewProject(
        name,
        project.WithID(project.ID(dto.ID)),
        project.WithStatus(project.Status(dto.Status)),
        project.WithCreatedAt(createdAt),
    ), nil
}
```

## Testing File Repository

```go
// internal/infra/repository/file_project_repo_test.go
package repository

import (
    "context"
    "errors"
    "os"
    "path/filepath"
    "testing"
    
    "myproject/internal/domain/project"
)

func TestFileProjectRepository_SaveAndFindByName(t *testing.T) {
    tmpDir := t.TempDir()
    repo := NewFileProjectRepository(tmpDir)
    ctx := context.Background()
    
    name, _ := project.NewName("test-project")
    p := project.NewProject(name)
    
    // Save
    err := repo.Save(ctx, p)
    if err != nil {
        t.Fatalf("Save() error: %v", err)
    }
    
    // Verify file exists
    path := filepath.Join(tmpDir, "projects", "test-project.json")
    if _, err := os.Stat(path); os.IsNotExist(err) {
        t.Errorf("expected file %s to exist", path)
    }
    
    // Find
    found, err := repo.FindByName(ctx, name)
    if err != nil {
        t.Fatalf("FindByName() error: %v", err)
    }
    if found.ID() != p.ID() {
        t.Errorf("ID = %v, want %v", found.ID(), p.ID())
    }
}

func TestFileProjectRepository_FindByName_NotFound(t *testing.T) {
    tmpDir := t.TempDir()
    repo := NewFileProjectRepository(tmpDir)
    
    name, _ := project.NewName("nonexistent")
    _, err := repo.FindByName(context.Background(), name)
    
    if !errors.Is(err, project.ErrNotFound) {
        t.Errorf("error = %v, want ErrNotFound", err)
    }
}

func TestFileProjectRepository_List_Empty(t *testing.T) {
    tmpDir := t.TempDir()
    repo := NewFileProjectRepository(tmpDir)
    
    projects, err := repo.List(context.Background())
    
    if err != nil {
        t.Fatalf("List() error: %v", err)
    }
    if len(projects) != 0 {
        t.Errorf("expected empty list, got %d items", len(projects))
    }
}
```

## External API Client

```go
// internal/infra/api/turtlebased_client.go
package api

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    
    "myproject/internal/domain/deployment"
)

type TurtlebasedClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

func NewTurtlebasedClient(baseURL, apiKey string) *TurtlebasedClient {
    return &TurtlebasedClient{
        baseURL:    baseURL,
        apiKey:     apiKey,
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }
}

// Implements deployment.Deployer interface
func (c *TurtlebasedClient) Deploy(ctx context.Context, r *deployment.Release, env deployment.Environment) error {
    payload := map[string]string{
        "release_id":  string(r.ID()),
        "version":     r.Version().String(),
        "environment": env.String(),
    }
    
    body, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    
    req, err := http.NewRequestWithContext(ctx, http.MethodPost,
        c.baseURL+"/api/v1/deploy",
        bytes.NewReader(body),
    )
    if err != nil {
        return err
    }
    
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("deployment API returned status %d", resp.StatusCode)
    }
    
    return nil
}
```

## Testing API Client with httptest

```go
// internal/infra/api/turtlebased_client_test.go
package api

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    
    "myproject/internal/domain/deployment"
)

func TestTurtlebasedClient_Deploy_Success(t *testing.T) {
    var receivedPayload map[string]string
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/api/v1/deploy" {
            t.Errorf("path = %q, want /api/v1/deploy", r.URL.Path)
        }
        if r.Header.Get("Authorization") != "Bearer test-key" {
            t.Error("missing authorization header")
        }
        json.NewDecoder(r.Body).Decode(&receivedPayload)
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()
    
    client := NewTurtlebasedClient(server.URL, "test-key")
    release := createTestRelease(t)
    env, _ := deployment.NewEnvironment("production")
    
    err := client.Deploy(context.Background(), release, env)
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if receivedPayload["version"] != release.Version().String() {
        t.Errorf("version = %q, want %q", receivedPayload["version"], release.Version())
    }
}

func TestTurtlebasedClient_Deploy_APIError(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer server.Close()
    
    client := NewTurtlebasedClient(server.URL, "test-key")
    release := createTestRelease(t)
    env, _ := deployment.NewEnvironment("production")
    
    err := client.Deploy(context.Background(), release, env)
    
    if err == nil {
        t.Fatal("expected error for 500 response")
    }
}

func createTestRelease(t *testing.T) *deployment.Release {
    t.Helper()
    version, _ := deployment.NewVersion("1.0.0")
    return deployment.NewRelease(deployment.ProjectID("proj-1"), version)
}
```

## Database Repository (PostgreSQL)

```go
// internal/infra/repository/postgres_project_repo.go
package repository

import (
    "context"
    "database/sql"
    
    "myproject/internal/domain/project"
)

type PostgresProjectRepository struct {
    db *sql.DB
}

func NewPostgresProjectRepository(db *sql.DB) *PostgresProjectRepository {
    return &PostgresProjectRepository{db: db}
}

func (r *PostgresProjectRepository) Save(ctx context.Context, p *project.Project) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO projects (id, name, status, created_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            status = EXCLUDED.status
    `, p.ID(), p.Name(), p.Status(), p.CreatedAt())
    return err
}

func (r *PostgresProjectRepository) FindByID(ctx context.Context, id project.ID) (*project.Project, error) {
    row := r.db.QueryRowContext(ctx, `
        SELECT id, name, status, created_at FROM projects WHERE id = $1
    `, id)
    
    var dto struct {
        ID, Name, Status string
        CreatedAt        time.Time
    }
    err := row.Scan(&dto.ID, &dto.Name, &dto.Status, &dto.CreatedAt)
    if err == sql.ErrNoRows {
        return nil, project.ErrNotFound
    }
    if err != nil {
        return nil, err
    }
    
    name, _ := project.NewName(dto.Name)
    return project.NewProject(name,
        project.WithID(project.ID(dto.ID)),
        project.WithStatus(project.Status(dto.Status)),
        project.WithCreatedAt(dto.CreatedAt),
    ), nil
}
```
