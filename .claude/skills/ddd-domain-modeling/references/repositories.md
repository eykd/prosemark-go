# Repository Interfaces

Repository interfaces define the contract for persistence in the domain layer. Implementation lives in infrastructure.

## Table of Contents

- [The Ports and Adapters Pattern](#the-ports-and-adapters-pattern)
- [Interface Design](#interface-design)
- [Common Repository Methods](#common-repository-methods)
- [Specification Pattern](#specification-pattern)
- [Implementation Guidelines](#implementation-guidelines)

## The Ports and Adapters Pattern

```
+---------------------------------------------------------+
|                      Domain Layer                        |
|  +---------------------------------------------------+  |
|  |  type TaskRepository interface (PORT)             |  |
|  |    FindByID(ctx, id string) (*Task, error)        |  |
|  |    Save(ctx, task *Task) error                    |  |
|  +---------------------------------------------------+  |
+---------------------------------------------------------+
                          ^
                          | implements
+---------------------------------------------------------+
|                  Infrastructure Layer                     |
|  +---------------------------------------------------+  |
|  |  type SQLTaskRepository struct (ADAPTER)          |  |
|  |    db *sql.DB                                     |  |
|  |    FindByID(ctx, id) (*Task, error)               |  |
|  |    Save(ctx, task) error                          |  |
|  +---------------------------------------------------+  |
+---------------------------------------------------------+
```

**Key Rule**: Domain layer defines the interface. Infrastructure layer provides the implementation.

## Interface Design

### Basic Repository Interface

```go
// internal/domain/repository.go

// TaskRepository defines persistence operations for tasks.
type TaskRepository interface {
	FindByID(ctx context.Context, id string) (*Task, error)
	FindAll(ctx context.Context) ([]*Task, error)
	Save(ctx context.Context, task *Task) error
	Delete(ctx context.Context, id string) error
}
```

### Repository with Query Methods

```go
// OrderRepository defines persistence and query operations for orders.
type OrderRepository interface {
	// Core CRUD
	FindByID(ctx context.Context, id string) (*Order, error)
	Save(ctx context.Context, order *Order) error
	Delete(ctx context.Context, id string) error

	// Domain-specific queries
	FindByCustomerID(ctx context.Context, customerID string) ([]*Order, error)
	FindByStatus(ctx context.Context, status OrderStatus) ([]*Order, error)
	FindByDateRange(ctx context.Context, r DateRange) ([]*Order, error)

	// Aggregate queries
	CountByStatus(ctx context.Context, status OrderStatus) (int, error)
	ExistsByID(ctx context.Context, id string) (bool, error)
}
```

### Repository with Pagination

```go
// Page holds a page of results with metadata.
type Page[T any] struct {
	Items       []T
	Total       int
	PageNum     int
	PageSize    int
	HasNext     bool
	HasPrevious bool
}

// PageRequest specifies pagination parameters.
type PageRequest struct {
	Page     int
	PageSize int
	SortBy   string
	SortDesc bool
}

// ProductRepository defines persistence for products with pagination.
type ProductRepository interface {
	FindByID(ctx context.Context, id string) (*Product, error)
	Save(ctx context.Context, product *Product) error

	// Paginated queries
	FindAll(ctx context.Context, req PageRequest) (Page[*Product], error)
	FindByCategory(ctx context.Context, categoryID string, req PageRequest) (Page[*Product], error)
	Search(ctx context.Context, query string, req PageRequest) (Page[*Product], error)
}
```

## Common Repository Methods

| Method              | Purpose           | Returns           |
| ------------------- | ----------------- | ----------------- |
| `FindByID(ctx, id)` | Get single entity | `(*Entity, error)` |
| `FindAll(ctx)`       | Get all entities  | `([]*Entity, error)` |
| `Save(ctx, entity)`  | Insert or update  | `error`           |
| `Delete(ctx, id)`    | Remove entity     | `error`           |
| `ExistsByID(ctx, id)` | Check existence  | `(bool, error)`   |
| `Count(ctx)`          | Total count      | `(int, error)`    |

### Naming Conventions

```go
// Query methods start with "Find"
FindByID(ctx context.Context, id string) (*Task, error)
FindByUserID(ctx context.Context, userID string) ([]*Task, error)
FindByStatus(ctx context.Context, status TaskStatus) ([]*Task, error)
FindCompletedBefore(ctx context.Context, t time.Time) ([]*Task, error)

// Boolean checks use "Exists"
ExistsByID(ctx context.Context, id string) (bool, error)
ExistsByEmail(ctx context.Context, email Email) (bool, error)

// Counts use "Count"
Count(ctx context.Context) (int, error)
CountByStatus(ctx context.Context, status TaskStatus) (int, error)
```

## Specification Pattern

For complex queries, define specifications in the domain:

```go
// TaskSpecification filters tasks by a domain rule.
type TaskSpecification interface {
	IsSatisfiedBy(task *Task) bool
}

// TaskQueryableRepository supports spec-based queries.
type TaskQueryableRepository interface {
	TaskRepository
	FindMatching(ctx context.Context, spec TaskSpecification) ([]*Task, error)
}

// OverdueTaskSpec matches tasks that are past due and not completed.
type OverdueTaskSpec struct {
	Now time.Time
}

func (s OverdueTaskSpec) IsSatisfiedBy(task *Task) bool {
	return task.DueDate().Before(s.Now) && !task.IsCompleted()
}
```

## Implementation Guidelines

### Infrastructure Implementation

```go
// internal/infra/repository/sql_task.go

// taskRow maps to the database schema.
type taskRow struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	Title     string    `db:"title"`
	Completed bool      `db:"completed"`
	CreatedAt time.Time `db:"created_at"`
}

// SQLTaskRepository implements TaskRepository using database/sql.
type SQLTaskRepository struct {
	db *sql.DB
}

// NewSQLTaskRepository creates a new SQL-backed task repository.
func NewSQLTaskRepository(db *sql.DB) *SQLTaskRepository {
	return &SQLTaskRepository{db: db}
}

func (r *SQLTaskRepository) FindByID(ctx context.Context, id string) (*Task, error) {
	var row taskRow
	err := r.db.QueryRowContext(ctx,
		"SELECT id, user_id, title, completed, created_at FROM tasks WHERE id = $1", id,
	).Scan(&row.ID, &row.UserID, &row.Title, &row.Completed, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying task %s: %w", id, err)
	}
	return r.toDomain(row), nil
}

func (r *SQLTaskRepository) FindAll(ctx context.Context) ([]*Task, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, user_id, title, completed, created_at FROM tasks ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("querying tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		var row taskRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.Title, &row.Completed, &row.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning task row: %w", err)
		}
		tasks = append(tasks, r.toDomain(row))
	}
	return tasks, rows.Err()
}

func (r *SQLTaskRepository) Save(ctx context.Context, task *Task) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tasks (id, user_id, title, completed, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT(id) DO UPDATE SET
			title = EXCLUDED.title,
			completed = EXCLUDED.completed`,
		task.ID(), task.UserID(), task.Title(), task.IsCompleted(), task.CreatedAt(),
	)
	if err != nil {
		return fmt.Errorf("saving task %s: %w", task.ID(), err)
	}
	return nil
}

func (r *SQLTaskRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting task %s: %w", id, err)
	}
	return nil
}

func (r *SQLTaskRepository) toDomain(row taskRow) *Task {
	status := TaskStatusPending
	if row.Completed {
		status = TaskStatusCompleted
	}
	return ReconstructTask(row.ID, row.UserID, row.Title, status, row.CreatedAt)
}
```

### Unit of Work (Optional)

For transactional consistency across multiple repositories:

```go
// UnitOfWork groups repository operations in a transaction.
type UnitOfWork interface {
	Tasks() TaskRepository
	Users() UserRepository
	Commit() error
	Rollback() error
}

// Usage in application layer
func (s *OrderService) PlaceOrder(ctx context.Context, req PlaceOrderRequest) error {
	uow, err := s.uowFactory.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	user, err := uow.Users().FindByID(ctx, req.UserID)
	if err != nil {
		_ = uow.Rollback()
		return err
	}

	order, err := NewOrder(user.ID(), req.Items)
	if err != nil {
		_ = uow.Rollback()
		return err
	}

	if err := uow.Tasks().Save(ctx, order); err != nil {
		_ = uow.Rollback()
		return err
	}

	return uow.Commit()
}
```

### Testing with Repository Interface

```go
// In-memory implementation for unit tests
type inMemoryTaskRepository struct {
	tasks map[string]*Task
}

func newInMemoryTaskRepository() *inMemoryTaskRepository {
	return &inMemoryTaskRepository{tasks: make(map[string]*Task)}
}

func (r *inMemoryTaskRepository) FindByID(_ context.Context, id string) (*Task, error) {
	task, ok := r.tasks[id]
	if !ok {
		return nil, nil
	}
	return task, nil
}

func (r *inMemoryTaskRepository) FindAll(_ context.Context) ([]*Task, error) {
	tasks := make([]*Task, 0, len(r.tasks))
	for _, t := range r.tasks {
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (r *inMemoryTaskRepository) Save(_ context.Context, task *Task) error {
	r.tasks[task.ID()] = task
	return nil
}

func (r *inMemoryTaskRepository) Delete(_ context.Context, id string) error {
	delete(r.tasks, id)
	return nil
}

// Use case test
func TestCreateTask(t *testing.T) {
	repo := newInMemoryTaskRepository()
	service := NewTaskService(repo)

	task, err := service.Create(context.Background(), "user-1", "Test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	saved, err := repo.FindByID(context.Background(), task.ID())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved == nil {
		t.Fatal("expected task to be persisted")
	}
	if saved.Title() != "Test" {
		t.Errorf("want title %q, got %q", "Test", saved.Title())
	}
}
```
