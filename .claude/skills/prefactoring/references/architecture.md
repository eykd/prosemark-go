# Architecture & System Design

**Purpose**: Apply prefactoring principles when designing system structure, packages, and boundaries.

## When to Use

- Creating new packages
- Defining system boundaries or layer organization
- Making architectural decisions about component relationships
- Choosing between embedding and composition

## Core Principles

### Well-Defined Interfaces

Define interfaces before implementation. Document preconditions, postconditions, and side effects.

```go
// UserRepository defines the contract for user persistence.
type UserRepository interface {
    // FindByID returns the user with the given ID.
    // Returns NotFoundError if the user does not exist.
    FindByID(id UserID) (User, error)

    // Save persists the user and returns it with a generated ID.
    Save(user User) (User, error)
}
```

### Decomposition & Modularity

Split systems into cohesive, loosely-coupled packages. Each package has one clear responsibility.

```go
// Good: Separate packages by domain concern
// users/domain/        - User entities, value objects
// users/application/   - Use cases
// users/infrastructure/- Repository implementations
// orders/domain/       - Order entities (separate bounded context)
```

### Separation of Concerns

Each package addresses one concern. Orthogonal concerns live in separate packages.

```go
// Separate concerns: validation, persistence, notification
type CreateUserUseCase struct {
    validator  UserValidator     // Validation concern
    repository UserRepository   // Persistence concern
    notifier   UserNotifier     // Notification concern
}

func NewCreateUserUseCase(
    v UserValidator,
    r UserRepository,
    n UserNotifier,
) *CreateUserUseCase {
    return &CreateUserUseCase{
        validator:  v,
        repository: r,
        notifier:   n,
    }
}
```

### Hierarchy & Layers

Dependencies flow in one direction: higher layers depend on lower layers.

```go
// Layer hierarchy (dependencies flow down)
// Handlers    -> Use Cases    -> Domain      -> (nothing)
// (adapters)  -> (application) -> (entities)

// Use case depends on domain, not vice versa
type ProcessOrderUseCase struct {
    repository OrderRepository
}

func (uc *ProcessOrderUseCase) Execute(req ProcessOrderRequest) (Order, error) {
    order, err := NewOrder(req) // Domain has no use case dependency
    if err != nil {
        return Order{}, err
    }
    return uc.repository.Save(order)
}
```

### Packaging

Components that change together should be packaged together.

```go
// Group by feature/domain, not by type
// Good:
// orders/order.go
// orders/repository.go
// orders/create_order.go

// Bad:
// entities/order.go
// repositories/order_repository.go
// usecases/create_order.go
```

### Think in Interfaces, Not Inheritance

Prefer composition and interfaces over embedding hierarchies.

```go
// Bad: Rigid embedding hierarchy
type Animal struct{}

func (a Animal) Move() {}

type Bird struct {
    Animal
}

func (b Bird) Fly() {}

type Penguin struct {
    Bird // Inherits Fly, but penguins can't fly!
}

// Good: Composition via interfaces
type Mover interface {
    Move()
}

type Flyer interface {
    Fly()
}

type Penguin struct{}

func (p Penguin) Move() {
    // waddle
}
// Penguin satisfies Mover but not Flyer - correct by design
```

## Decision Matrix

| Situation             | Apply                    | Example                      |
| --------------------- | ------------------------ | ---------------------------- |
| New package           | Decomposition, Packaging | Group related files together |
| API boundary          | Well-Defined Interfaces  | Document contracts           |
| Cross-cutting concern | Separation of Concerns   | Extract to separate package  |
| Type hierarchy        | Think in Interfaces      | Prefer composition           |
| Dependency direction  | Hierarchy                | Higher depends on lower      |

## Related References

- [value-objects.md](./value-objects.md): Type-level design decisions
- [contracts.md](./contracts.md): Contract design and validation
