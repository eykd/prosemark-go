# Layer Rules Reference

## Table of Contents

1. [Layer Overview](#layer-overview)
2. [Dependency Matrix](#dependency-matrix)
3. [Layer Contents](#layer-contents)
4. [Directory Mapping](#directory-mapping)
5. [Refactoring Patterns](#refactoring-patterns)

---

## Layer Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Presentation Layer                        │
│   (Handlers, Controllers, Templates, CLI)                   │
├─────────────────────────────────────────────────────────────┤
│                    Infrastructure Layer                      │
│   (Repositories, External APIs, Caches, File System)        │
├─────────────────────────────────────────────────────────────┤
│                    Application Layer                         │
│   (Use Cases, DTOs, Application Services)                   │
├─────────────────────────────────────────────────────────────┤
│                      Domain Layer                            │
│   (Entities, Value Objects, Domain Services, Interfaces)    │
└─────────────────────────────────────────────────────────────┘
                    Dependencies point DOWN ↓
```

---

## Dependency Matrix

| From \ To          | Domain | Application | Infrastructure | Presentation |
| ------------------ | ------ | ----------- | -------------- | ------------ |
| **Domain**         | ✓      | ❌          | ❌             | ❌           |
| **Application**    | ✓      | ✓           | ❌             | ❌           |
| **Infrastructure** | ✓      | ✓           | ✓              | ❌           |
| **Presentation**   | ✓      | ✓           | ✓              | ✓            |

**Key rule:** ✓ = allowed, ❌ = violation

---

## Layer Contents

### Domain Layer

**Purpose:** Core business logic and rules, independent of technical concerns.

**Contains:**

- Entities (aggregate roots with identity)
- Value Objects (immutable, identity-less)
- Domain Services (operations spanning entities)
- Repository Interfaces (ports for data access)
- Domain Events
- Domain Errors (sentinel errors or custom error types)

**Allowed imports:**

- Other domain types only
- Go standard library built-ins (errors, fmt, strings, time, etc.)

**Forbidden:**

- Framework packages
- Database clients (database/sql, pgx, etc.)
- HTTP types (net/http)
- File system APIs (os, io/fs for direct access)
- External service clients

### Application Layer

**Purpose:** Orchestrate domain objects to fulfill use cases.

**Contains:**

- Use Cases / Application Services
- DTOs (Data Transfer Objects)
- Input/Output boundaries
- Application Events
- Validation logic (for DTOs)

**Allowed imports:**

- Domain types (entities, value objects, interfaces)
- Application types (DTOs, other use cases)

**Forbidden:**

- Infrastructure implementations
- Presentation types (http.Request, http.Response)
- Framework-specific types

### Infrastructure Layer

**Purpose:** Implement technical concerns and integrate with external systems.

**Contains:**

- Repository Implementations
- External API Clients
- Cache Implementations
- Message Queue Adapters
- File System Access
- Database Migrations

**Allowed imports:**

- Domain interfaces (to implement)
- Application types (to use DTOs)
- Framework packages
- Database clients (database/sql, pgx, etc.)
- External SDKs

### Presentation Layer

**Purpose:** Handle HTTP/CLI/UI and translate to application calls.

**Contains:**

- HTTP Handlers
- Route Definitions
- Middleware
- HTML Templates
- CLI Commands (Cobra commands)
- Request/Response Mapping

**Allowed imports:**

- Application use cases
- Domain types (for response mapping)
- Infrastructure (for dependency injection setup)
- Framework packages (net/http, Cobra, etc.)

---

## Directory Mapping

### Standard Structure

```
internal/
├── domain/
│   ├── entity/
│   ├── valueobject/
│   ├── service/
│   └── port/              # Repository/port interfaces HERE
│
├── application/
│   ├── usecase/
│   ├── dto/
│   └── service/
│
├── infrastructure/
│   ├── repository/        # Implements domain/port interfaces
│   ├── cache/
│   ├── external/
│   └── persistence/
│
└── presentation/
    ├── handler/
    ├── middleware/
    ├── template/
    └── route/
```

### Alternative Names

Some projects use different names:

| Standard       | Alternatives                    |
| -------------- | ------------------------------- |
| domain         | core, model, business           |
| application    | usecase, service, app           |
| infrastructure | adapter, data, external, infra  |
| presentation   | web, api, http, ui, cli         |

---

## Refactoring Patterns

### Extract Interface to Domain

**Before:**

```
internal/infrastructure/repository/user_repository.go  # interface + impl
```

**After:**

```
internal/domain/port/user_repository.go                # interface only
internal/infrastructure/repository/sql_user_repository.go  # implementation
```

### Remove Infrastructure from Domain

**Before:**

```go
// internal/domain/entity/user.go
package entity

import "database/sql" // ❌ infrastructure dependency in domain

type User struct {
	ID    string
	Name  string
	Email string
	db    *sql.DB
}

func (u *User) Save() error {
	_, err := u.db.Exec("INSERT INTO users (id, name, email) VALUES (?, ?, ?)", u.ID, u.Name, u.Email)
	return err
}
```

**After:**

```go
// internal/domain/entity/user.go
package entity

// Pure entity, no persistence
type User struct {
	ID    string
	Name  string
	Email string
}
```

```go
// internal/domain/port/user_repository.go
package port

import "myapp/internal/domain/entity"

type UserRepository interface {
	Save(user *entity.User) error
}
```

```go
// internal/infrastructure/repository/sql_user_repository.go
package repository

import (
	"database/sql"
	"myapp/internal/domain/entity"
	"myapp/internal/domain/port"
)

// Verify interface compliance at compile time.
var _ port.UserRepository = (*SQLUserRepository)(nil)

type SQLUserRepository struct {
	db *sql.DB
}

func NewSQLUserRepository(db *sql.DB) *SQLUserRepository {
	return &SQLUserRepository{db: db}
}

func (r *SQLUserRepository) Save(user *entity.User) error {
	_, err := r.db.Exec("INSERT INTO users (id, name, email) VALUES (?, ?, ?)", user.ID, user.Name, user.Email)
	return err
}
```

### Remove Framework Types from Use Cases

**Before:**

```go
// internal/application/usecase/create_user.go
package usecase

import "net/http" // ❌ presentation concern in application

func (uc *CreateUser) Execute(r *http.Request) (*http.Response, error) {
	// ...
}
```

**After:**

```go
// internal/application/dto/create_user.go
package dto

type CreateUserRequest struct {
	Email string
	Name  string
}

type UserResponse struct {
	ID    string
	Email string
	Name  string
}
```

```go
// internal/application/usecase/create_user.go
package usecase

import (
	"myapp/internal/application/dto"
)

func (uc *CreateUser) Execute(req dto.CreateUserRequest) (*dto.UserResponse, error) {
	// ...
}
```

```go
// internal/presentation/handler/user_handler.go
package handler

import (
	"encoding/json"
	"net/http"
	"myapp/internal/application/dto"
	"myapp/internal/application/usecase"
)

type UserHandler struct {
	createUser *usecase.CreateUser
}

func (h *UserHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.createUser.Execute(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
```

### Dependency Injection Setup

Wire dependencies in composition root (entry point):

```go
// cmd/server/main.go (or internal/app/wire.go)
package main

import (
	"database/sql"
	"log"
	"net/http"

	"myapp/internal/application/usecase"
	"myapp/internal/infrastructure/cache"
	"myapp/internal/infrastructure/repository"
	"myapp/internal/presentation/handler"
)

func main() {
	db, err := sql.Open("sqlite3", "app.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create infrastructure implementations
	userRepo := repository.NewSQLUserRepository(db)
	cacheService := cache.NewRedisCache("localhost:6379")

	// Create use cases with injected dependencies
	createUser := usecase.NewCreateUser(userRepo)
	getUser := usecase.NewGetUser(userRepo, cacheService)

	// Create handlers with use cases
	userHandler := handler.NewUserHandler(createUser, getUser)

	// Route requests
	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", userHandler.HandleCreate)
	mux.HandleFunc("GET /users/{id}", userHandler.HandleGet)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
```
