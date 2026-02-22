# Extraction Refactorings

## Extract Function

**When**: Code block has clear purpose, appears multiple times, or function is too long.

**Steps**:

1. Create new function named after intent (what, not how)
2. Copy code to new function
3. Identify variables needed—pass as parameters
4. Replace original with function call
5. Test

```go
// Before
func printOwing(invoice Invoice) {
	outstanding := 0.0
	for _, o := range invoice.Orders {
		outstanding += o.Amount
	}

	fmt.Printf("Customer: %s\n", invoice.Customer)
	fmt.Printf("Amount: %.2f\n", outstanding)
}

// After
func printOwing(invoice Invoice) {
	outstanding := calculateOutstanding(invoice)
	printDetails(invoice, outstanding)
}

func calculateOutstanding(invoice Invoice) float64 {
	total := 0.0
	for _, o := range invoice.Orders {
		total += o.Amount
	}
	return total
}

func printDetails(invoice Invoice, outstanding float64) {
	fmt.Printf("Customer: %s\n", invoice.Customer)
	fmt.Printf("Amount: %.2f\n", outstanding)
}
```

## Inline Function

**When**: Function body is as clear as its name, or function is simple delegation.

**Steps**:

1. Check function is not part of an interface implementation
2. Find all callers
3. Replace each call with function body
4. Remove function definition
5. Test after each replacement

```go
// Before
func getRating(driver Driver) int {
	if moreThanFiveLateDeliveries(driver) {
		return 2
	}
	return 1
}

func moreThanFiveLateDeliveries(driver Driver) bool {
	return driver.LateDeliveries > 5
}

// After
func getRating(driver Driver) int {
	if driver.LateDeliveries > 5 {
		return 2
	}
	return 1
}
```

## Extract Variable

**When**: Complex expression is hard to understand.

**Steps**:

1. Ensure expression has no side effects
2. Declare variable with clear name
3. Assign expression to variable
4. Replace expression with variable reference

```go
// Before
func price(order Order) float64 {
	return order.Quantity*order.ItemPrice -
		math.Max(0, float64(order.Quantity-500))*order.ItemPrice*0.05 +
		math.Min(order.Quantity*order.ItemPrice*0.1, 100)
}

// After
func price(order Order) float64 {
	basePrice := float64(order.Quantity) * order.ItemPrice
	quantityDiscount := math.Max(0, float64(order.Quantity-500)) * order.ItemPrice * 0.05
	shipping := math.Min(basePrice*0.1, 100)
	return basePrice - quantityDiscount + shipping
}
```

## Inline Variable

**When**: Variable name doesn't add meaning beyond the expression.

**Steps**:

1. Check variable is assigned only once
2. Replace all references with expression
3. Remove declaration

```go
// Before
basePrice := order.BasePrice
return basePrice > 1000

// After
return order.BasePrice > 1000
```

## Replace Temp with Query

**When**: Temporary variable holds calculation that could be a function.

**Steps**:

1. Extract calculation into function
2. Replace temp references with function call
3. Remove temp declaration

```go
// Before
func (o *Order) Price() float64 {
	basePrice := float64(o.Quantity) * o.ItemPrice
	if basePrice > 1000 {
		return basePrice * 0.95
	}
	return basePrice * 0.98
}

// After
func (o *Order) Price() float64 {
	if o.basePrice() > 1000 {
		return o.basePrice() * 0.95
	}
	return o.basePrice() * 0.98
}

func (o *Order) basePrice() float64 {
	return float64(o.Quantity) * o.ItemPrice
}
```
