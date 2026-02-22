# Violation Patterns Reference

## Table of Contents

1. [Domain Layer Violations](#domain-layer-violations)
2. [Application Layer Violations](#application-layer-violations)
3. [Interface Misplacement](#interface-misplacement)
4. [Common Framework Violations](#common-framework-violations)

---

## Domain Layer Violations

Domain code must be pure—no external dependencies.

### Infrastructure Import

**Bad:**

```go
// internal/domain/entity/user.go
import "database/sql"           // ❌
import "github.com/jmoiron/sqlx" // ❌
```

**Fix:** Remove all infrastructure imports. Domain entities should only import other domain types.

### Direct Database Access

**Bad:**

```go
// internal/domain/service/user_service.go
package service

import "database/sql" // ❌

type UserService struct {
	db *sql.DB // ❌
}

func (s *UserService) FindUser(id string) (*User, error) {
	row := s.db.QueryRow("SELECT * FROM users WHERE id = ?", id) // ❌
	// ...
}
```

**Fix:** Define repository interface in domain, inject implementation:

```go
// internal/domain/port/user_repository.go
package port

import "myapp/internal/domain/entity"

type UserRepository interface {
	FindByID(id string) (*entity.User, error)
}
```

```go
// internal/domain/service/user_service.go
package service

import (
	"myapp/internal/domain/entity"
	"myapp/internal/domain/port"
)

type UserService struct {
	userRepo port.UserRepository
}

func NewUserService(repo port.UserRepository) *UserService {
	return &UserService{userRepo: repo}
}

func (s *UserService) FindUser(id string) (*entity.User, error) {
	return s.userRepo.FindByID(id)
}
```

### HTTP/Request Objects in Domain

**Bad:**

```go
// internal/domain/entity/task.go
package entity

import "net/http" // ❌

func TaskFromRequest(r *http.Request) (*Task, error) { // ❌
	// ...
}
```

**Fix:** Use plain DTOs. Parse requests in presentation layer.

### External API Calls in Domain Logic

**Bad:**

```go
// internal/domain/entity/order.go
package entity

import (
	"encoding/json"
	"net/http" // ❌
)

func (o *Order) CalculateTotal() (float64, error) {
	resp, err := http.Get("https://api.exchange.com/rate") // ❌
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var rate float64
	json.NewDecoder(resp.Body).Decode(&rate)
	return o.Amount * rate, nil
}
```

**Fix:** Pass rates as parameters or use domain services with injected dependencies.

---

## Application Layer Violations

Application layer orchestrates domain objects but must not know infrastructure details.

### Concrete Infrastructure Instantiation

**Bad:**

```go
// internal/application/usecase/create_user.go
package usecase

import "myapp/internal/infrastructure/repository" // ❌

type CreateUser struct{}

func (uc *CreateUser) Execute(data CreateUserRequest) error {
	repo := repository.NewSQLUserRepository(db) // ❌
	// ...
}
```

**Fix:** Accept interface via constructor:

```go
// internal/application/usecase/create_user.go
package usecase

import "myapp/internal/domain/port"

type CreateUser struct {
	userRepo port.UserRepository // ✓
}

func NewCreateUser(repo port.UserRepository) *CreateUser {
	return &CreateUser{userRepo: repo}
}

func (uc *CreateUser) Execute(data CreateUserRequest) error {
	// use uc.userRepo
}
```

### Framework Types in Use Case Signatures

**Bad:**

```go
// internal/application/usecase/get_tasks.go
package usecase

import "net/http" // ❌

type GetTasks struct{}

func (uc *GetTasks) Execute(r *http.Request) (*http.Response, error) { // ❌
	// ...
}
```

**Fix:** Use plain DTOs:

```go
// internal/application/usecase/get_tasks.go
package usecase

type GetTasks struct{}

func (uc *GetTasks) Execute(query GetTasksQuery) ([]TaskResponse, error) { // ✓
	// ...
}
```

### Direct External API Calls

**Bad:**

```go
// internal/application/usecase/send_notification.go
package usecase

import "net/http" // ❌

type SendNotification struct{}

func (uc *SendNotification) Execute(userID string) error {
	_, err := http.Post("https://api.sendgrid.com/send", "application/json", body) // ❌
	return err
}
```

**Fix:** Define port interface, inject adapter:

```go
// internal/domain/port/notification_service.go
package port

type NotificationService interface {
	Send(to string, message string) error
}
```

```go
// internal/application/usecase/send_notification.go
package usecase

import "myapp/internal/domain/port"

type SendNotification struct {
	notifier port.NotificationService
}

func NewSendNotification(notifier port.NotificationService) *SendNotification {
	return &SendNotification{notifier: notifier}
}

func (uc *SendNotification) Execute(userID string) error {
	return uc.notifier.Send(userID, "Hello")
}
```

---

## Interface Misplacement

Repository interfaces are ports—they belong in the domain layer.

### Interface in Infrastructure

**Bad:**

```
internal/infrastructure/repository/
├── user_repository.go         # Interface defined here ❌
└── sql_user_repository.go     # Implementation
```

**Fix:**

```
internal/domain/port/
└── user_repository.go         # Interface here ✓

internal/infrastructure/repository/
└── sql_user_repository.go     # Implementation imports interface
```

### Interface Importing Implementation Types

**Bad:**

```go
// internal/domain/port/cache_service.go
package port

import "github.com/go-redis/redis/v8" // ❌

type CacheService interface {
	Get(key string, client *redis.Client) (string, error) // ❌
}
```

**Fix:** Keep interface pure:

```go
// internal/domain/port/cache_service.go
package port

type CacheService interface {
	Get(key string) (string, error) // ✓
}
```

---

## Common Framework Violations

### net/http in Domain/Application

Violation indicators:

- `"net/http"` imported in domain or application packages
- `http.Request`, `http.Response`, `http.Handler` in domain
- `http.StatusOK` or other HTTP constants in domain

### database/sql in Domain

Violation indicators:

- `"database/sql"` imported in domain packages
- `sql.DB`, `sql.Tx`, `sql.Row` in domain entities or services
- Database driver imports in domain (e.g., `github.com/lib/pq`)

### ORM Libraries in Domain

Violation indicators:

- GORM struct tags (`gorm:"column:name"`) on domain entities
- `"gorm.io/gorm"` imported in domain
- Ent/SQLBoiler generated types used as domain entities
- Database-specific struct tags in domain (e.g., `db:"column_name"`)

### Validation Libraries

**Acceptable:** Using validator tags or validation logic in application layer DTOs
**Violation:** Validation library imports directly on domain entities

```go
// Bad - domain entity
// internal/domain/entity/user.go
package entity

import "github.com/go-playground/validator/v10" // ❌ in domain

type User struct {
	Email string `validate:"required,email"` // ❌
}
```

```go
// Good - application DTO
// internal/application/dto/create_user.go
package dto

type CreateUserRequest struct {
	Email string `validate:"required,email"` // ✓ in application/dto
	Name  string `validate:"required"`
}
```
