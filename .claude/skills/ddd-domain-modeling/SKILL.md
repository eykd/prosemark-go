---
name: ddd-domain-modeling
description: 'Use when: (1) building domain entities with validation, (2) creating value objects, (3) defining repository interfaces, (4) implementing domain services, (5) questions about DDD patterns or business logic encapsulation.'
---

# DDD Domain Modeling

Create domain models in pure Go with zero external dependencies.

## Core Principle

The domain layer contains **pure business logic only**:

- No framework imports
- No I/O operations in entities/value objects
- No database or HTTP concerns
- Validate invariants in constructors
- Dependencies point inward (infrastructure -> application -> domain)

## Directory Structure

```
internal/domain/
├── order.go              # Aggregate roots and entities
├── order_test.go
├── money.go              # Immutable value types
├── money_test.go
├── email.go
├── email_test.go
├── pricing.go            # Stateless domain logic (services)
├── pricing_test.go
└── repository.go         # Repository interfaces (ports)
```

## Quick Reference

### Entity (3 parts)

```go
// Order is an aggregate root with identity and lifecycle.
type Order struct {
	id        string
	userID    string
	title     string
	status    OrderStatus
	createdAt time.Time
}

// NewOrder creates a new Order, validating business rules.
func NewOrder(userID, title string) (*Order, error) {
	if strings.TrimSpace(title) == "" {
		return nil, errors.New("title is required")
	}
	return &Order{
		id:        uuid.NewString(),
		userID:    userID,
		title:     strings.TrimSpace(title),
		status:    OrderStatusPending,
		createdAt: time.Now(),
	}, nil
}

// ReconstructOrder rebuilds an Order from persistence without validation.
func ReconstructOrder(id, userID, title string, status OrderStatus, createdAt time.Time) *Order {
	return &Order{id: id, userID: userID, title: title, status: status, createdAt: createdAt}
}

// Getters + behavior methods that enforce rules
func (o *Order) ID() string         { return o.id }
func (o *Order) Title() string      { return o.title }
```

### Value Object

```go
// Money represents a monetary amount with currency.
type Money struct {
	Amount   int    // cents
	Currency string // ISO 4217
}

// NewMoney creates a Money value, validating inputs.
func NewMoney(amount int, currency string) (Money, error) {
	if amount < 0 {
		return Money{}, errors.New("amount cannot be negative")
	}
	if len(currency) != 3 {
		return Money{}, errors.New("currency must be 3-letter ISO code")
	}
	return Money{Amount: amount, Currency: currency}, nil
}

// Add returns a new Money with the sum. Returns error on currency mismatch.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("currency mismatch: %s vs %s", m.Currency, other.Currency)
	}
	return Money{Amount: m.Amount + other.Amount, Currency: m.Currency}, nil
}

// Equal reports whether two Money values are identical.
func (m Money) Equal(other Money) bool {
	return m.Amount == other.Amount && m.Currency == other.Currency
}
```

### Repository Interface

```go
// Domain layer - interface only
type OrderRepository interface {
	FindByID(ctx context.Context, id string) (*Order, error)
	Save(ctx context.Context, order *Order) error
}
```

### Domain Service

```go
// PricingService contains stateless calculation logic across entities.
type PricingService struct{}

// CalculateTotal computes the total from line items and discounts.
func (s *PricingService) CalculateTotal(items []LineItem, discounts []Discount) (Money, error) {
	// Complex calculation logic here
}
```

## Workflow

1. **Identify the concept**: Entity (has identity) or Value Object (defined by attributes)?
2. **Define invariants**: What rules must always be true?
3. **Choose pattern**: See detailed references below
4. **Write tests first**: Domain code is pure -- test without mocks

## Detailed References

- **Entities with identity and lifecycle**: See [references/entities.md](references/entities.md)
- **Value Objects with validation**: See [references/value-objects.md](references/value-objects.md)
- **Repository interfaces (ports)**: See [references/repositories.md](references/repositories.md)
- **Domain services for complex logic**: See [references/domain-services.md](references/domain-services.md)

## Anti-Patterns to Avoid

| Anti-Pattern                              | Instead                                                            |
| ----------------------------------------- | ------------------------------------------------------------------ |
| `import "database/sql"` in domain         | Define interface in domain, implement in infrastructure            |
| I/O in entity methods                     | Keep entities pure; I/O belongs in repositories/infrastructure     |
| Exposing struct fields directly           | Use unexported fields + accessor methods: `order.AddItem()` not `order.Items = nil` |
| Validation in application layer           | Validate in entity/value object constructors                       |
| Anemic domain model                       | Put behavior with the data it operates on                          |
