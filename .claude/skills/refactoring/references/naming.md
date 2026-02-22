# Naming Refactorings

## Change Function Declaration (Rename Function)

**When**: Function name doesn't clearly communicate purpose.

**Steps** (Simple):

1. Change function name in declaration
2. Update all call sites
3. Test

**Steps** (Migration for published APIs):

1. Create new function with better name
2. Have old function delegate to new
3. Migrate callers gradually
4. Remove old function

```go
// Before
func circum(radius float64) float64 {
	return 2 * math.Pi * radius
}

// After
func circumference(radius float64) float64 {
	return 2 * math.Pi * radius
}
```

## Rename Variable

**When**: Variable name is unclear or misleading.

**Steps**:

1. If widely used, consider Encapsulate Variable first
2. Change name in declaration
3. Update all references
4. Use IDE automated renaming when available

```go
// Before
a := height * width

// After
area := height * width
```

## Rename Field

**When**: Field name doesn't match current understanding of data.

**Steps**:

1. If struct has limited scope, rename directly
2. Otherwise, use Encapsulate Record first
3. Rename unexported field
4. Adjust accessor methods

```go
// Before
type Organization struct {
	Name string
	Ctry string
}

// After
type Organization struct {
	Name    string
	Country string
}
```

## Comments to Better Names

**When**: Comments describe what code does (not why).

Comments are often a sign of unclear code. Instead of documenting unclear code, make it clear through better naming.

```go
// Before
// Check if customer is eligible for discount
if customer.Age > 65 || customer.MembershipYears > 10 {
	// Apply senior or loyalty discount
	total = total * 0.9
}

// After
isEligibleForDiscount := customer.Age > 65 || customer.MembershipYears > 10
if isEligibleForDiscount {
	total = applyDiscount(total, loyaltyDiscountRate)
}

// OR even better: Extract Function
if isEligibleForDiscount(customer) {
	total = applyDiscount(total, loyaltyDiscountRate)
}
```

## Good Naming Principles

1. **Use domain language** — Match terms stakeholders use
2. **Reveal intent** — Name after what, not how
3. **Be specific** — `customerCount` not `data`
4. **Avoid abbreviations** — `circumference` not `circum`
5. **Keep comments for "why"** — Code should explain "what"
6. **Follow Go conventions** — Exported `PascalCase`, unexported `camelCase`, acronyms all-caps (`HTTPClient`, `userID`)

| Bad Name    | Good Name        | Reason                   |
| ----------- | ---------------- | ------------------------ |
| `d`         | `elapsedDays`    | Reveals what it measures |
| `list`      | `customers`      | Reveals domain meaning   |
| `flag`      | `isActive`       | Reveals purpose          |
| `temp`      | `subtotal`       | Reveals business concept |
| `doStuff()` | `CalculateTax()` | Reveals operation        |
| `data`      | `userProfile`    | Reveals content type     |
