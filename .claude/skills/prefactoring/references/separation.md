# Separation & DRY Principles

**Purpose**: Apply prefactoring principles for eliminating duplication and separating concerns.

## When to Use

- Deciding between duplication and abstraction
- Separating business rules from technical logic
- Implementing varying algorithms
- Structuring complex logic

## Core Principles

### Don't Repeat Yourself

Every piece of knowledge has a single, authoritative representation.

```go
// Bad: Duplicated validation logic
func CreateUser(email string) error {
    if !strings.Contains(email, "@") {
        return errors.New("invalid email")
    }
    // ...
    return nil
}

func UpdateEmail(email string) error {
    if !strings.Contains(email, "@") {
        return errors.New("invalid email")
    }
    // ...
    return nil
}

// Good: Single source of truth
type Email struct {
    value string
}

func NewEmail(value string) (Email, error) {
    if !strings.Contains(value, "@") {
        return Email{}, &InvalidEmailError{Value: value}
    }
    return Email{value: value}, nil
}

func CreateUser(email Email) error { return nil }
func UpdateEmail(email Email) error { return nil }
```

### Separate Policy from Implementation

Keep the "what" separate from the "how" for flexibility and clarity.

```go
// Bad: Policy mixed with implementation
func CalculateDiscount(order Order) float64 {
    if order.Total > 1000 {
        return order.Total * 0.1
    }
    if len(order.Items) > 10 {
        return order.Total * 0.05
    }
    return 0
}

// Good: Policy separate from implementation
type DiscountPolicy interface {
    Applies(order Order) bool
    Calculate(order Order) Money
}

type BulkOrderDiscount struct{}

func (d BulkOrderDiscount) Applies(order Order) bool {
    return order.Total.Exceeds(BulkThreshold)
}

func (d BulkOrderDiscount) Calculate(order Order) Money {
    return order.Total.Multiply(0.1)
}

type DiscountCalculator struct {
    policies []DiscountPolicy
}

func NewDiscountCalculator(policies []DiscountPolicy) *DiscountCalculator {
    return &DiscountCalculator{policies: policies}
}

func (c *DiscountCalculator) Calculate(order Order) Money {
    for _, p := range c.policies {
        if p.Applies(order) {
            return p.Calculate(order)
        }
    }
    return MoneyZero()
}
```

### Avoid Premature Generalization

Solve the specific problem first. Generalize when patterns emerge.

```go
// Bad: Premature abstraction before second use case
type DataProcessor[T any, R any] interface {
    Process(input T) R
    Validate(input T) bool
    Transform(input T) T
}

// Good: Solve specific problem first
func ProcessUserRegistration(data RegistrationData) (User, error) {
    // Implement specifically for this use case
    return User{}, nil
}

// Later, when you have 2-3 similar cases, THEN abstract
func ProcessOrderSubmission(data OrderData) (Order, error) {
    // If pattern emerges, consider shared abstraction
    return Order{}, nil
}
```

### Adopt a Prefactoring Attitude

Eliminate duplication before it occurs. Look for patterns during design.

```go
// Before implementing feature #2, check if it shares logic with feature #1
// If so, extract shared logic BEFORE implementing #2

// Example: Before adding SMS notifications alongside email
type NotificationChannel interface {
    Send(recipient Recipient, message Message) error
}

// Now both implementations follow the same pattern
type EmailChannel struct {
    // ...
}

func (c *EmailChannel) Send(recipient Recipient, message Message) error {
    // ...
    return nil
}

type SMSChannel struct {
    // ...
}

func (c *SMSChannel) Send(recipient Recipient, message Message) error {
    // ...
    return nil
}
```

## Decision Matrix

| Situation                          | Apply                          | Action                     |
| ---------------------------------- | ------------------------------ | -------------------------- |
| Logic in multiple places           | DRY                            | Extract to single location |
| Algorithm can vary                 | Policy/Implementation          | Use strategy pattern       |
| Building first implementation      | Avoid Premature Generalization | Keep specific              |
| About to implement similar feature | Prefactoring Attitude          | Extract pattern first      |

## When to Extract vs. Duplicate

```
Rule of Three:
1st occurrence: Just write it
2nd occurrence: Note the duplication
3rd occurrence: Extract the abstraction
```

## Related References

- [naming.md](./naming.md): Naming extracted abstractions
- [architecture.md](./architecture.md): Package-level separation
