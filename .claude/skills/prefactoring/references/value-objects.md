# Value Objects & Type Design

**Purpose**: Apply prefactoring principles when wrapping primitives, grouping data, and creating meaningful types.

## When to Use

- Creating new types for domain concepts
- Wrapping primitive values (strings, ints)
- Grouping related parameters into structs
- Extracting magic numbers/strings to constants

## Core Principles

### Be Abstract All the Way

Never use primitives for domain concepts. Wrap them in meaningful types.

```go
// Bad: Primitives lose domain meaning
func CreateUser(email string, age int) {}

// Good: Domain types with validation
type Email struct {
    value string
}

func NewEmail(value string) (Email, error) {
    if !strings.Contains(value, "@") {
        return Email{}, &InvalidEmailError{Value: value}
    }
    return Email{value: value}, nil
}

func (e Email) String() string {
    return e.value
}

type Age struct {
    years int
}

func NewAge(years int) (Age, error) {
    if years < 0 || years > 150 {
        return Age{}, &InvalidAgeError{Years: years}
    }
    return Age{years: years}, nil
}

func CreateUser(email Email, age Age) {}
```

### Clump Data

Group related values into cohesive structs to reduce cognitive load.

```go
// Bad: Parameter explosion
func PlaceOrder(
    street string,
    city string,
    zip string,
    country string,
    cardNumber string,
    expiry string,
    cvv string,
) {}

// Good: Cohesive value objects
type Address struct {
    Street  string
    City    string
    Zip     string
    Country string
}

type PaymentDetails struct {
    CardNumber string
    Expiry     string
    CVV        string
}

func PlaceOrder(address Address, payment PaymentDetails) {}
```

### Never Let a Constant Slip

Use named constants for all meaningful values.

```go
// Bad: Magic numbers
if retryCount > 3 {
    // give up
}
if order.Total > 1000 {
    // apply discount
}

// Good: Named constants
const (
    MaxRetryAttempts   = 3
    BulkOrderThreshold = 1000
)

if retryCount > MaxRetryAttempts {
    // give up
}
if order.Total > BulkOrderThreshold {
    // apply discount
}
```

### Splitters Can Be Lumped

Start with fine-grained abstractions. It's easier to combine than to split.

```go
// Start specific, generalize later
type EmailNotification struct {
    // email-specific fields
}

type SMSNotification struct {
    // SMS-specific fields
}

// Later, if needed, create common abstraction
type Notifier interface {
    Send(recipient Recipient, message Message) error
}
```

## Decision Matrix

| Situation             | Apply                   | Example                         |
| --------------------- | ----------------------- | ------------------------------- |
| String with format    | Be Abstract All the Way | `Email`, `URL`, `PhoneNumber`   |
| Number with units     | Be Abstract All the Way | `Money`, `Duration`, `Distance` |
| Related parameters    | Clump Data              | `Address`, `DateRange`          |
| Literal value         | Named Constants         | `MaxRetries`, `DefaultTimeout`  |
| Uncertain granularity | Splitters Can Be Lumped | Start specific                  |

## Related References

- [collections.md](./collections.md): Domain collections with behavior
- [naming.md](./naming.md): Naming conventions for types
