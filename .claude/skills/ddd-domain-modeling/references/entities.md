# Entities

Entities have identity that persists through state changes.

## Table of Contents

- [Structure](#structure)
- [Constructor Functions](#constructor-functions)
- [Invariant Enforcement](#invariant-enforcement)
- [Aggregate Roots](#aggregate-roots)
- [Testing Entities](#testing-entities)

## Structure

```go
// TaskStatus is a value object for task state.
type TaskStatus int

const (
	TaskStatusPending TaskStatus = iota
	TaskStatusInProgress
	TaskStatusCompleted
)

// Task is a domain entity with identity and lifecycle.
type Task struct {
	id        string
	userID    string
	title     string
	status    TaskStatus
	createdAt time.Time
}

// NewTask creates a new Task, validating business rules.
func NewTask(userID, title string) (*Task, error) {
	trimmed := strings.TrimSpace(title)
	if len(trimmed) < 3 {
		return nil, errors.New("title must be at least 3 characters")
	}
	return &Task{
		id:        uuid.NewString(),
		userID:    userID,
		title:     trimmed,
		status:    TaskStatusPending,
		createdAt: time.Now(),
	}, nil
}

// ReconstructTask rebuilds a Task from persistence without validation.
func ReconstructTask(id, userID, title string, status TaskStatus, createdAt time.Time) *Task {
	return &Task{id: id, userID: userID, title: title, status: status, createdAt: createdAt}
}

// Getters expose state (never setters).
func (t *Task) ID() string          { return t.id }
func (t *Task) Title() string       { return t.title }
func (t *Task) IsCompleted() bool   { return t.status == TaskStatusCompleted }

// Complete marks the task as completed, enforcing business rules.
func (t *Task) Complete() error {
	if t.status == TaskStatusCompleted {
		return errors.New("task already completed")
	}
	t.status = TaskStatusCompleted
	return nil
}

// Rename updates the task title, enforcing validation.
func (t *Task) Rename(newTitle string) error {
	trimmed := strings.TrimSpace(newTitle)
	if len(trimmed) < 3 {
		return errors.New("title must be at least 3 characters")
	}
	t.title = trimmed
	return nil
}
```

## Constructor Functions

Two constructor functions serve different purposes:

| Function          | Purpose                      | Validates | Generates ID |
| ----------------- | ---------------------------- | --------- | ------------ |
| `NewTask()`       | New entities from user input | Yes       | Yes          |
| `ReconstructTask()` | Rebuild from persistence   | No        | No           |

```go
// Application layer uses NewTask()
task, err := NewTask("user-1", "Buy milk")
if err != nil {
	return fmt.Errorf("creating task: %w", err)
}

// Repository uses ReconstructTask()
func (r *SQLTaskRepository) toDomain(row taskRow) *Task {
	status := TaskStatusPending
	if row.Completed {
		status = TaskStatusCompleted
	}
	return ReconstructTask(
		row.ID,
		row.UserID,
		row.Title,
		status,
		row.CreatedAt,
	)
}
```

## Invariant Enforcement

Invariants are rules that must **always** be true:

```go
// Order is an aggregate root that enforces item invariants.
type Order struct {
	id    string
	items []LineItem
}

// NewOrder creates an Order, requiring at least one item.
func NewOrder(id string, items []LineItem) (*Order, error) {
	if len(items) == 0 {
		return nil, errors.New("order must have at least one item")
	}
	return &Order{id: id, items: items}, nil
}

// AddItem adds a line item, rejecting duplicates.
func (o *Order) AddItem(item LineItem) error {
	for _, existing := range o.items {
		if existing.ProductID == item.ProductID {
			return errors.New("product already in order")
		}
	}
	o.items = append(o.items, item)
	return nil
}

// RemoveItem removes a line item, preventing removal of the last one.
func (o *Order) RemoveItem(productID string) error {
	if len(o.items) == 1 {
		return errors.New("cannot remove last item from order")
	}
	filtered := make([]LineItem, 0, len(o.items)-1)
	for _, item := range o.items {
		if item.ProductID != productID {
			filtered = append(filtered, item)
		}
	}
	o.items = filtered
	return nil
}
```

## Aggregate Roots

Aggregate roots control access to child entities:

```go
// Order controls all access to its child LineItems.
type Order struct {
	id       string
	items    []LineItem
	total    Money
	currency string
}

// Items returns a copy of the order's line items.
func (o *Order) Items() []LineItem {
	cp := make([]LineItem, len(o.items))
	copy(cp, o.items)
	return cp
}

// AddItem adds a line item through the root and recalculates the total.
func (o *Order) AddItem(productID string, quantity int, price Money) error {
	item, err := NewLineItem(productID, quantity, price)
	if err != nil {
		return err
	}
	o.items = append(o.items, *item)
	o.recalculateTotal()
	return nil
}

func (o *Order) recalculateTotal() {
	total := MoneyZero(o.currency)
	for _, item := range o.items {
		subtotal := item.Subtotal()
		sum, _ := total.Add(subtotal)
		total = sum
	}
	o.total = total
}
```

## Testing Entities

Domain entities are pure -- test without mocks:

```go
func TestNewTask(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		title   string
		wantErr string
	}{
		{
			name:   "creates task with valid title",
			userID: "user-1",
			title:  "Buy milk",
		},
		{
			name:    "rejects short titles",
			userID:  "user-1",
			title:   "ab",
			wantErr: "title must be at least 3 characters",
		},
		{
			name:   "trims whitespace",
			userID: "user-1",
			title:  "  Buy milk  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := NewTask(tt.userID, tt.title)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("want error %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if task.Title() != strings.TrimSpace(tt.title) {
				t.Errorf("want title %q, got %q", strings.TrimSpace(tt.title), task.Title())
			}
			if task.IsCompleted() {
				t.Error("new task should not be completed")
			}
			if task.ID() == "" {
				t.Error("task ID should be set")
			}
		})
	}
}

func TestTask_Complete(t *testing.T) {
	t.Run("marks pending task as completed", func(t *testing.T) {
		task, _ := NewTask("user-1", "Test")
		if err := task.Complete(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !task.IsCompleted() {
			t.Error("task should be completed")
		}
	})

	t.Run("returns error when completing twice", func(t *testing.T) {
		task, _ := NewTask("user-1", "Test")
		_ = task.Complete()
		err := task.Complete()
		if err == nil || err.Error() != "task already completed" {
			t.Fatalf("want error %q, got %v", "task already completed", err)
		}
	})
}
```
