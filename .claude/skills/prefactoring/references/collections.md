# Domain Collections & Method Placement

**Purpose**: Apply prefactoring principles for collections with behavior and proper method placement.

## When to Use

- Working with slices/maps that have domain meaning
- Deciding where methods should live
- Encapsulating aggregate operations
- Preventing feature envy anti-pattern

## Core Principles

### Collections with Domain Behavior

Wrap collections when they have domain-specific operations.

```go
// Bad: Raw slice with scattered logic
items := []OrderItem{}
total := 0.0
for _, item := range items {
    total += item.Price
}
hasDiscount := len(items) > 10

// Good: Named slice type with behavior
type OrderItems []OrderItem

func (oi OrderItems) Total() Money {
    sum := MoneyZero()
    for _, item := range oi {
        sum = sum.Add(item.Price)
    }
    return sum
}

func (oi OrderItems) HasDiscount() bool {
    return len(oi) > BulkDiscountThreshold
}

func (oi OrderItems) IsEmpty() bool {
    return len(oi) == 0
}
```

### Place Methods by Need

Methods belong where their data lives. Avoid feature envy.

```go
// Bad: Feature envy - function uses another object's data
func PrintOrder(order Order) string {
    return fmt.Sprintf("%s: %d items, $%s", order.ID, len(order.Items), order.Total)
}

func IsValidOrder(order Order) bool {
    return len(order.Items) > 0 && order.Total > 0
}

// Good: Methods on the type with the data
type Order struct {
    ID    string
    Items OrderItems
    Total Money
}

func (o Order) String() string {
    return fmt.Sprintf("%s: %d items, $%s", o.ID, len(o.Items), o.Total)
}

func (o Order) IsValid() bool {
    return len(o.Items) > 0 && o.Total.IsPositive()
}
```

### Package-Level Functions for Non-Instance Operations

If a function doesn't need receiver data, it should be a package-level function.

```go
// Bad: Method that doesn't use receiver data
type DateUtils struct{}

func (d DateUtils) FormatDate(t time.Time) string {
    return t.Format("2006-01-02")
}

// Good: Package-level function
func FormatDate(t time.Time) string {
    return t.Format("2006-01-02")
}
```

## Decision Matrix

| Situation                       | Apply                     | Action                          |
| ------------------------------- | ------------------------- | ------------------------------- |
| Slice with operations           | Named Slice Type          | Create domain collection type   |
| Method uses other type's data   | Place Methods by Need     | Move to data owner              |
| Method doesn't use receiver     | Package-Level Function    | Extract to package function     |
| Multiple methods on same data   | Domain Type               | Group into cohesive struct      |

## Collection Wrapper Checklist

When creating a domain collection, consider:

- [ ] Does it encapsulate the underlying slice/map?
- [ ] Does it provide domain-specific query methods?
- [ ] Does it enforce invariants (e.g., non-empty)?
- [ ] Does it hide implementation details?

```go
// UserGroup is a non-empty collection of users.
type UserGroup struct {
    users []User
}

func NewUserGroup(users []User) (UserGroup, error) {
    if len(users) == 0 {
        return UserGroup{}, &EmptyGroupError{}
    }
    return UserGroup{users: users}, nil
}

func (g UserGroup) FindByEmail(email Email) (User, bool) {
    for _, u := range g.users {
        if u.Email.Equals(email) {
            return u, true
        }
    }
    return User{}, false
}

func (g UserGroup) ActiveUsers() (UserGroup, error) {
    var active []User
    for _, u := range g.users {
        if u.IsActive {
            active = append(active, u)
        }
    }
    return NewUserGroup(active)
}
```

## Related References

- [value-objects.md](./value-objects.md): Creating domain types
- [separation.md](./separation.md): Separating concerns in code
