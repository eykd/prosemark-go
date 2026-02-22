# Interface Contracts & Validation

**Purpose**: Apply prefactoring principles when defining APIs, validating inputs, and designing for testability.

## When to Use

- Defining public APIs or contracts
- Implementing input validation at boundaries
- Writing tests that verify behavior
- Designing testable components

## Core Principles

### Create Interface Contracts

Document and enforce preconditions, postconditions, and invariants.

```go
// TransferService transfers money between accounts.
//
// Preconditions:
//   - Both accounts must exist and be active.
//   - Source account must have sufficient balance.
//
// Postconditions:
//   - source.Balance = source.Balance - amount
//   - target.Balance = target.Balance + amount
//
// Errors:
//   - InsufficientFundsError if balance is too low.
//   - AccountNotFoundError if an account does not exist.
type TransferService interface {
    Transfer(source AccountID, target AccountID, amount Money) error
}
```

### Validate, Validate, Validate

Validate at every system boundary. Fail fast with clear error messages.

```go
type CreateUserHandler struct {
    useCase CreateUserUseCase
}

func (h *CreateUserHandler) Handle(r *http.Request) (*http.Response, error) {
    // Validate at entry point
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        return nil, &ValidationError{Message: "invalid request body"}
    }
    if errs := req.Validate(); len(errs) > 0 {
        return jsonResponse(http.StatusBadRequest, map[string]any{"errors": errs})
    }

    // Domain types are already valid
    email, err := NewEmail(req.Email)
    if err != nil {
        return jsonResponse(http.StatusBadRequest, map[string]any{"error": err.Error()})
    }

    user, err := h.useCase.Execute(email)
    if err != nil {
        return nil, err
    }
    return jsonResponse(http.StatusOK, user)
}
```

### Test the Interface, Not the Implementation

Test contracts, not internal details. Enable refactoring without breaking tests.

```go
// Bad: Tests implementation details
func TestSave_CallsCorrectSQL(t *testing.T) {
    // Checking exact SQL strings couples tests to implementation
}

// Good: Tests contract
func TestSave_PersistsUserAndReturnsWithID(t *testing.T) {
    email, _ := NewEmail("test@example.com")
    user := NewUser(email)

    saved, err := repository.Save(user)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if saved.ID == "" {
        t.Fatal("expected saved user to have an ID")
    }

    retrieved, err := repository.FindByID(saved.ID)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if retrieved.Email != user.Email {
        t.Errorf("got email %v, want %v", retrieved.Email, user.Email)
    }
}
```

### Build Flexibility for Testing

Design with dependency injection and clear interfaces.

```go
// Testable design: dependencies injected via constructor
type OrderService struct {
    repository     OrderRepository
    paymentGateway PaymentGateway
    notifier       OrderNotifier
}

func NewOrderService(
    repo OrderRepository,
    gateway PaymentGateway,
    notifier OrderNotifier,
) *OrderService {
    return &OrderService{
        repository:     repo,
        paymentGateway: gateway,
        notifier:       notifier,
    }
}

// In tests: inject test doubles
func TestOrderService(t *testing.T) {
    service := NewOrderService(
        &InMemoryOrderRepository{},
        &MockPaymentGateway{},
        &SpyOrderNotifier{},
    )
    // ...
}
```

## Decision Matrix

| Situation            | Apply               | Example                        |
| -------------------- | ------------------- | ------------------------------ |
| Public API method    | Interface Contracts | Document pre/postconditions    |
| External input       | Validate            | Check at system boundary       |
| Writing tests        | Test Interface      | Verify contract, not internals |
| Complex dependencies | Build Flexibility   | Use dependency injection       |

## Related References

- [error-handling.md](./error-handling.md): Error strategies and messaging
- [architecture.md](./architecture.md): Interface design at package level
