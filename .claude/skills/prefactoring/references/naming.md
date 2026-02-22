# Naming & Code Communication

**Purpose**: Apply prefactoring principles for clear naming, self-documenting code, and explicit intent.

## When to Use

- Naming functions, types, or variables
- Making code self-documenting
- Choosing between implicit and explicit behavior
- Deciding whether to build or reuse

## Core Principles

### A Rose by Any Other Name

Each concept gets one clear, consistent name from the domain language.

```go
// Bad: Inconsistent naming
func GetUser() {}
func FetchCustomer() {} // Same concept, different name
func RetrievePerson() {} // Same concept, different name

// Good: Consistent ubiquitous language
func FindUser() {}
func FindUserByID() {}
func FindUserByEmail() {}
```

### Communicate with Your Code

Code should communicate intent without requiring comments.

```go
// Bad: Comment explains unclear code
// Check if user can access the resource
if user.Role == "admin" || user.ID == resource.OwnerID {
}

// Good: Self-documenting code
isAdmin := user.HasRole(RoleAdmin)
isOwner := resource.IsOwnedBy(user)
if isAdmin || isOwner {
}
```

### Explicitness Beats Implicitness

State intent clearly. Avoid magic behavior.

```go
// Bad: Implicit behavior
type OrderOptions struct {
    Notify *bool // nil means... true? false?
}

func ProcessOrder(order Order, opts OrderOptions) {
    shouldNotify := opts.Notify == nil || *opts.Notify // Implicit default
}

// Good: Explicit parameters
type OrderOptions struct {
    Notify bool
}

func ProcessOrder(order Order, opts OrderOptions) {
    if opts.Notify {
        // ...
    }
}
```

### Don't Reinvent the Wheel

Use existing solutions before creating new ones.

```go
// Bad: Custom date formatting
func formatDate(t time.Time) string {
    return fmt.Sprintf("%02d/%02d/%04d", t.Month(), t.Day(), t.Year())
}

// Good: Use standard library
formatted := t.Format("01/02/2006")
```

## Decision Matrix

| Situation                     | Apply                 | Action                   |
| ----------------------------- | --------------------- | ------------------------ |
| Same concept, different names | Ubiquitous Language   | Standardize naming       |
| Code needs explanation        | Communicate with Code | Rename for clarity       |
| Magic defaults                | Explicitness          | Make parameters explicit |
| Standard problem              | Don't Reinvent        | Use existing library     |

## Naming Checklist

When naming, ask:

- [ ] Does it use domain language? (not technical jargon)
- [ ] Is it consistent with similar concepts?
- [ ] Does it reveal intent? (not implementation)
- [ ] Is it searchable? (avoid abbreviations)

```go
// Domain language examples
type Order struct{}     // Not: DataRecord, Entity
type Money struct{}     // Not: NumberWrapper, AmountValue
func PlaceOrder() {}   // Not: ProcessData, HandleRequest
```

## Related References

- [separation.md](./separation.md): Code structure and DRY
- [value-objects.md](./value-objects.md): Naming domain types
