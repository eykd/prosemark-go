# CLI Layer

Thin adapter translating user input to application service calls. Uses Cobra.

## Location
`cmd/`

## Command Structure

```
cmd/
├── root.go              # Root command, global flags
├── project.go           # project subcommand group
├── project_init.go      # tb project init
├── project_init_test.go
├── project_list.go      # tb project list
└── deploy.go            # tb deploy
```

## Root Command

```go
// cmd/root.go
package cmd

import (
    "os"
    
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "tb",
    Short: "Turtlebased CLI",
    Long:  "A CLI for managing turtlebased projects and deployments.",
}

var configDir string

func init() {
    rootCmd.PersistentFlags().StringVar(&configDir, "config", "", "config directory")
}

func Execute() error {
    return rootCmd.Execute()
}

func getConfigDir() string {
    if configDir != "" {
        return configDir
    }
    if dir := os.Getenv("TB_CONFIG_DIR"); dir != "" {
        return dir
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".tb")
}
```

## Subcommand Group

```go
// cmd/project.go
package cmd

import "github.com/spf13/cobra"

var projectCmd = &cobra.Command{
    Use:   "project",
    Short: "Manage projects",
    Long:  "Commands for creating, listing, and managing projects.",
}

func init() {
    rootCmd.AddCommand(projectCmd)
}
```

### SilenceErrors Contract

When `SilenceErrors: true` is set, Cobra will NOT print errors. The caller of
`Execute()`/`ExecuteContext()` MUST print errors to stderr. An architecture test
(`TestMainPrintsErrorsWhenSilenceErrorsSet`) enforces this.

## Command Implementation

Commands are thin: parse input → call service → format output.

```go
// cmd/project_init.go
package cmd

import (
    "context"
    "errors"
    "fmt"
    
    "github.com/spf13/cobra"
    
    "myproject/internal/app"
    "myproject/internal/domain/project"
)

var projectInitCmd = &cobra.Command{
    Use:   "init <n>",
    Short: "Initialize a new project",
    Long:  "Creates a new project with the specified name.",
    Args:  cobra.ExactArgs(1),
    RunE:  runProjectInit,
}

func init() {
    projectCmd.AddCommand(projectInitCmd)
}

func runProjectInit(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()
    if ctx == nil {
        ctx = context.Background()
    }
    
    // Get service (wired by container)
    svc := getProjectService()
    
    p, err := svc.InitializeProject(ctx, args[0])
    if err != nil {
        return formatError(err)
    }
    
    fmt.Fprintf(cmd.OutOrStdout(), "Project '%s' initialized\n", p.Name())
    return nil
}

func formatError(err error) error {
    var notFound *app.NotFoundError
    var validation *app.ValidationError
    var conflict *app.ConflictError
    
    switch {
    case errors.Is(err, project.ErrEmptyName):
        return fmt.Errorf("project name cannot be empty")
    case errors.Is(err, app.ErrProjectAlreadyExists):
        return fmt.Errorf("project already exists")
    case errors.As(err, &notFound):
        return fmt.Errorf("%s not found: %s", notFound.Resource, notFound.ID)
    case errors.As(err, &validation):
        return fmt.Errorf("invalid %s: %s", validation.Field, validation.Message)
    case errors.As(err, &conflict):
        return fmt.Errorf("cannot proceed: %s", conflict.Message)
    default:
        return err
    }
}
```

## Command with Flags

```go
// cmd/deploy.go
package cmd

var deployCmd = &cobra.Command{
    Use:   "deploy <release-id>",
    Short: "Deploy a release",
    Args:  cobra.ExactArgs(1),
    RunE:  runDeploy,
}

var (
    deployEnv    string
    deployDryRun bool
)

func init() {
    rootCmd.AddCommand(deployCmd)
    deployCmd.Flags().StringVarP(&deployEnv, "environment", "e", "staging", "target environment")
    deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "simulate deployment")
}

func runDeploy(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()
    if ctx == nil {
        ctx = context.Background()
    }
    
    releaseID := args[0]
    svc := getDeploymentService()
    
    if deployDryRun {
        fmt.Fprintf(cmd.OutOrStdout(), "Would deploy %s to %s\n", releaseID, deployEnv)
        return nil
    }
    
    fmt.Fprintf(cmd.OutOrStdout(), "Deploying %s to %s...\n", releaseID, deployEnv)
    
    if err := svc.Deploy(ctx, releaseID, deployEnv); err != nil {
        return formatError(err)
    }
    
    fmt.Fprintf(cmd.OutOrStdout(), "✓ Deployment successful\n")
    return nil
}
```

## Testing Commands

### Unit Test: Flag Parsing and Service Calls

```go
// cmd/project_init_test.go
package cmd

import (
    "bytes"
    "context"
    "strings"
    "testing"
)

func TestProjectInitCmd_Success(t *testing.T) {
    var capturedName string
    mockSvc := &mockProjectService{
        initFunc: func(ctx context.Context, name string) (*project.Project, error) {
            capturedName = name
            n, _ := project.NewName(name)
            return project.NewProject(n), nil
        },
    }
    
    // Inject mock
    origGetter := getProjectService
    getProjectService = func() *app.ProjectService { return mockSvc }
    defer func() { getProjectService = origGetter }()
    
    // Execute command
    cmd := rootCmd
    cmd.SetArgs([]string{"project", "init", "my-project"})
    out := new(bytes.Buffer)
    cmd.SetOut(out)
    
    err := cmd.Execute()
    
    if err != nil {
        t.Fatalf("command failed: %v", err)
    }
    if capturedName != "my-project" {
        t.Errorf("name = %q, want %q", capturedName, "my-project")
    }
    if !strings.Contains(out.String(), "initialized") {
        t.Errorf("output = %q, want success message", out.String())
    }
}

func TestProjectInitCmd_EmptyName(t *testing.T) {
    mockSvc := &mockProjectService{
        initFunc: func(ctx context.Context, name string) (*project.Project, error) {
            return nil, project.ErrEmptyName
        },
    }
    
    origGetter := getProjectService
    getProjectService = func() *app.ProjectService { return mockSvc }
    defer func() { getProjectService = origGetter }()
    
    cmd := rootCmd
    cmd.SetArgs([]string{"project", "init", ""})
    errOut := new(bytes.Buffer)
    cmd.SetErr(errOut)
    
    err := cmd.Execute()
    
    if err == nil {
        t.Fatal("expected error")
    }
    if !strings.Contains(err.Error(), "cannot be empty") {
        t.Errorf("error = %q, want empty name error", err.Error())
    }
}

type mockProjectService struct {
    initFunc func(ctx context.Context, name string) (*project.Project, error)
}

func (m *mockProjectService) InitializeProject(ctx context.Context, name string) (*project.Project, error) {
    return m.initFunc(ctx, name)
}
```

### Acceptance Test: End-to-End

```go
// test/acceptance/project_init_test.go
package acceptance

import (
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

func TestProjectInit_CreatesProject(t *testing.T) {
    tmpDir := t.TempDir()
    
    cmd := exec.Command(binaryPath(t), "project", "init", "my-project")
    cmd.Env = append(os.Environ(), "TB_CONFIG_DIR="+tmpDir)
    output, err := cmd.CombinedOutput()
    
    if err != nil {
        t.Fatalf("command failed: %v\noutput: %s", err, output)
    }
    
    if !strings.Contains(string(output), "initialized") {
        t.Errorf("unexpected output: %s", output)
    }
    
    // Verify side effect
    projectFile := filepath.Join(tmpDir, "projects", "my-project.json")
    if _, err := os.Stat(projectFile); os.IsNotExist(err) {
        t.Errorf("project file not created: %s", projectFile)
    }
}

func TestProjectInit_RejectsDuplicate(t *testing.T) {
    tmpDir := t.TempDir()
    env := append(os.Environ(), "TB_CONFIG_DIR="+tmpDir)
    
    // Create first
    cmd := exec.Command(binaryPath(t), "project", "init", "existing")
    cmd.Env = env
    cmd.Run()
    
    // Try duplicate
    cmd = exec.Command(binaryPath(t), "project", "init", "existing")
    cmd.Env = env
    output, err := cmd.CombinedOutput()
    
    if err == nil {
        t.Fatal("expected error for duplicate")
    }
    if !strings.Contains(string(output), "already exists") {
        t.Errorf("unexpected error: %s", output)
    }
}

func binaryPath(t *testing.T) string {
    t.Helper()
    // Assumes binary built to bin/tb
    return filepath.Join(os.Getenv("PROJECT_ROOT"), "bin", "tb")
}
```

## Structured Output

Support multiple formats for scriptability.

```go
// cmd/output.go
package cmd

import (
    "encoding/json"
    "fmt"
    "io"
    "text/tabwriter"
)

var outputFormat string

func init() {
    rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "output format (text|json|table)")
}

type ProjectView struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Status string `json:"status"`
}

func printProject(w io.Writer, p *project.Project) error {
    view := ProjectView{
        ID:     string(p.ID()),
        Name:   p.Name().String(),
        Status: string(p.Status()),
    }
    
    switch outputFormat {
    case "json":
        return json.NewEncoder(w).Encode(view)
    case "table":
        tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
        fmt.Fprintf(tw, "ID:\t%s\n", view.ID)
        fmt.Fprintf(tw, "Name:\t%s\n", view.Name)
        fmt.Fprintf(tw, "Status:\t%s\n", view.Status)
        return tw.Flush()
    default:
        fmt.Fprintf(w, "Project: %s (%s)\n", view.Name, view.ID)
        return nil
    }
}

func printProjects(w io.Writer, projects []*project.Project) error {
    if outputFormat == "json" {
        views := make([]ProjectView, len(projects))
        for i, p := range projects {
            views[i] = ProjectView{
                ID:     string(p.ID()),
                Name:   p.Name().String(),
                Status: string(p.Status()),
            }
        }
        return json.NewEncoder(w).Encode(views)
    }
    
    tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
    fmt.Fprintf(tw, "NAME\tID\tSTATUS\n")
    for _, p := range projects {
        fmt.Fprintf(tw, "%s\t%s\t%s\n", p.Name(), p.ID(), p.Status())
    }
    return tw.Flush()
}
```

## Service Injection

```go
// cmd/container.go
package cmd

import (
    "myproject/internal/app"
    "myproject/internal/infra/repository"
)

var container *Container

type Container struct {
    ProjectService    *app.ProjectService
    DeploymentService *app.DeploymentService
}

func SetContainer(c *Container) {
    container = c
}

func getProjectService() *app.ProjectService {
    if container != nil {
        return container.ProjectService
    }
    // Fallback: create with defaults
    repo := repository.NewFileProjectRepository(getConfigDir())
    return app.NewProjectService(app.WithProjectRepository(repo))
}

func getDeploymentService() *app.DeploymentService {
    if container != nil {
        return container.DeploymentService
    }
    // Fallback or panic
    panic("deployment service not configured")
}
```
