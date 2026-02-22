# Domain Services

Domain services contain business logic that doesn't naturally fit within a single entity or value object.

## Table of Contents

- [When to Use Domain Services](#when-to-use-domain-services)
- [Characteristics](#characteristics)
- [Patterns](#patterns)
- [Testing Domain Services](#testing-domain-services)

## When to Use Domain Services

Use a domain service when the operation:

- Involves multiple entities or aggregates
- Requires business logic that doesn't belong to any single entity
- Implements a domain concept that is a "verb" rather than a "noun"
- Needs to enforce cross-entity invariants

| Scenario                              | Solution                                    |
| ------------------------------------- | ------------------------------------------- |
| Calculate order total from line items | Entity method: `order.CalculateTotal()`     |
| Check if user can afford a purchase   | Domain service: `PaymentEligibilityService` |
| Transfer funds between accounts       | Domain service: `FundsTransferService`      |
| Validate an email format              | Value object: `NewEmail()`                  |

## Characteristics

Domain services are:

- **Stateless**: No instance state, operate purely on inputs
- **Pure**: No side effects, same inputs -> same outputs
- **Framework-free**: No database, HTTP, or infrastructure dependencies
- **Synchronous**: No I/O operations (those belong in application layer)

```go
// Good: Pure domain service
type PricingService struct{}

func (s *PricingService) CalculateDiscount(items []LineItem, tier CustomerTier) (Money, error) {
	// Pure calculation logic
}

// Bad: Has infrastructure concerns
type PricingService struct {
	db *sql.DB // Infrastructure dependency!
}

func (s *PricingService) CalculateDiscount(ctx context.Context, customerID string) (Money, error) {
	customer, err := s.db.QueryRow(...) // I/O operation!
	// ...
}
```

## Patterns

### Calculation Service

```go
// internal/domain/pricing.go

// PricingService performs price calculations across line items and discounts.
type PricingService struct{}

// CalculateSubtotal sums all line item totals.
func (s *PricingService) CalculateSubtotal(items []LineItem) (Money, error) {
	if len(items) == 0 {
		return MoneyZero("USD"), nil
	}

	total := MoneyZero(items[0].Price.Currency)
	for _, item := range items {
		itemTotal, err := item.Price.Multiply(item.Quantity)
		if err != nil {
			return Money{}, err
		}
		total, err = total.Add(itemTotal)
		if err != nil {
			return Money{}, err
		}
	}
	return total, nil
}

// DiscountType distinguishes percentage from fixed-amount discounts.
type DiscountType int

const (
	DiscountPercentage DiscountType = iota
	DiscountFixed
)

// Discount represents a price reduction.
type Discount struct {
	Type  DiscountType
	Value int // percentage (0-100) or fixed amount in cents
}

// ApplyDiscounts reduces the subtotal by the given discounts.
// Percentage discounts are applied before fixed discounts.
// The result is never negative.
func (s *PricingService) ApplyDiscounts(subtotal Money, discounts []Discount) Money {
	total := subtotal

	// Apply percentage discounts first, then fixed
	for _, d := range discounts {
		if d.Type == DiscountPercentage {
			reduction, _ := total.Multiply(d.Value)
			reduction = Money{Amount: reduction.Amount / 100, Currency: total.Currency}
			total, _ = total.Subtract(reduction)
		}
	}
	for _, d := range discounts {
		if d.Type == DiscountFixed {
			fixed := Money{Amount: d.Value, Currency: total.Currency}
			result, err := total.Subtract(fixed)
			if err != nil {
				return MoneyZero(total.Currency) // Never go below zero
			}
			total = result
		}
	}

	if total.Amount < 0 {
		return MoneyZero(total.Currency)
	}
	return total
}

// CalculateTax computes tax on an amount at the given rate (basis points).
func (s *PricingService) CalculateTax(amount Money, rateBasisPoints int) Money {
	tax := amount.Amount * rateBasisPoints / 10000
	return Money{Amount: tax, Currency: amount.Currency}
}
```

### Validation Service

```go
// internal/domain/order_validation.go

// ValidationResult holds the outcome of a domain validation.
type ValidationResult struct {
	IsValid bool
	Errors  []string
}

// OrderValidationService validates orders against business rules.
type OrderValidationService struct{}

// ValidateOrder checks an order against customer and business constraints.
func (s *OrderValidationService) ValidateOrder(order *Order, customer *Customer) ValidationResult {
	var errs []string

	// Business rule: Customer must be active
	if !customer.IsActive() {
		errs = append(errs, "customer account is not active")
	}

	// Business rule: Order value within customer's credit limit
	if order.Total().Amount > customer.CreditLimit().Amount {
		errs = append(errs, "order exceeds customer credit limit")
	}

	// Business rule: Minimum order value
	minimumCents := 1000 // $10.00
	if order.Total().Amount < minimumCents {
		errs = append(errs, "order must be at least $10")
	}

	// Business rule: No more than 100 items per order
	if order.ItemCount() > 100 {
		errs = append(errs, "order cannot exceed 100 items")
	}

	return ValidationResult{
		IsValid: len(errs) == 0,
		Errors:  errs,
	}
}
```

### Policy Service

```go
// internal/domain/shipping.go

// ShippingOption describes an available shipping method.
type ShippingOption struct {
	Name          string
	Cost          Money
	EstimatedDays int
}

// ShippingPolicyService determines available shipping options.
type ShippingPolicyService struct{}

const freeShippingThresholdCents = 5000 // $50.00

// GetAvailableOptions returns shipping options based on order and destination.
func (s *ShippingPolicyService) GetAvailableOptions(order *Order, destination Address) []ShippingOption {
	currency := order.Total().Currency
	var options []ShippingOption

	// Standard shipping always available
	options = append(options, ShippingOption{
		Name:          "Standard",
		Cost:          s.calculateStandardCost(order, destination),
		EstimatedDays: s.standardDeliveryDays(destination),
	})

	// Express available for orders under 50lbs
	if order.TotalWeight() < 50 {
		cost, _ := NewMoney(1599, currency)
		options = append(options, ShippingOption{
			Name:          "Express",
			Cost:          cost,
			EstimatedDays: 2,
		})
	}

	// Overnight for domestic only
	if destination.Country == "US" {
		cost, _ := NewMoney(2999, currency)
		options = append(options, ShippingOption{
			Name:          "Overnight",
			Cost:          cost,
			EstimatedDays: 1,
		})
	}

	return options
}

func (s *ShippingPolicyService) calculateStandardCost(order *Order, dest Address) Money {
	currency := order.Total().Currency
	if order.Total().Amount >= freeShippingThresholdCents {
		return MoneyZero(currency)
	}

	baseCents := 599
	if dest.Country != "US" {
		baseCents = 1499
	}

	weightSurcharge := 0
	if order.TotalWeight() > 5 {
		weightSurcharge = (order.TotalWeight() - 5) * 50 // 50 cents per lb over 5
	}

	cost, _ := NewMoney(baseCents+weightSurcharge, currency)
	return cost
}

func (s *ShippingPolicyService) standardDeliveryDays(dest Address) int {
	if dest.Country == "US" {
		return 5
	}
	return 14
}
```

### Allocation/Distribution Service

```go
// internal/domain/allocation.go

// Allocation records how much stock is allocated from a warehouse.
type Allocation struct {
	WarehouseID string
	Quantity    int
}

// Unallocated records demand that could not be satisfied.
type Unallocated struct {
	ProductID string
	Quantity  int
}

// AllocationResult holds the outcome of an inventory allocation.
type AllocationResult struct {
	Allocations map[string][]Allocation // keyed by product ID
	Unallocated []Unallocated
}

// InventoryAllocationService assigns warehouse stock to order items.
type InventoryAllocationService struct{}

// AllocateStock distributes order items across warehouses by available stock.
func (s *InventoryAllocationService) AllocateStock(items []OrderItem, stocks []WarehouseStock) AllocationResult {
	result := AllocationResult{
		Allocations: make(map[string][]Allocation),
	}

	for _, item := range items {
		remaining := item.Quantity

		// Gather stocks for this product, sorted by available (highest first)
		matching := filterAndSortStocks(stocks, item.ProductID)

		for _, stock := range matching {
			if remaining <= 0 {
				break
			}
			allocate := min(remaining, stock.Available)
			if allocate > 0 {
				result.Allocations[item.ProductID] = append(
					result.Allocations[item.ProductID],
					Allocation{WarehouseID: stock.WarehouseID, Quantity: allocate},
				)
				remaining -= allocate
			}
		}

		if remaining > 0 {
			result.Unallocated = append(result.Unallocated,
				Unallocated{ProductID: item.ProductID, Quantity: remaining})
		}
	}

	return result
}

func filterAndSortStocks(stocks []WarehouseStock, productID string) []WarehouseStock {
	var matching []WarehouseStock
	for _, s := range stocks {
		if s.ProductID == productID {
			matching = append(matching, s)
		}
	}
	sort.Slice(matching, func(i, j int) bool {
		return matching[i].Available > matching[j].Available
	})
	return matching
}
```

## Testing Domain Services

Domain services are pure functions -- test without mocks:

```go
func TestPricingService_CalculateSubtotal(t *testing.T) {
	svc := &PricingService{}

	tests := []struct {
		name  string
		items []LineItem
		want  int // expected amount in cents
	}{
		{
			name: "sums all line items",
			items: []LineItem{
				newLineItem(1000, "USD", 2), // $10 x 2
				newLineItem(500, "USD", 3),  // $5 x 3
			},
			want: 3500, // $35
		},
		{
			name:  "returns zero for empty items",
			items: nil,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.CalculateSubtotal(tt.items)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Amount != tt.want {
				t.Errorf("want %d, got %d", tt.want, result.Amount)
			}
		})
	}
}

func TestPricingService_ApplyDiscounts(t *testing.T) {
	svc := &PricingService{}

	tests := []struct {
		name      string
		subtotal  Money
		discounts []Discount
		want      int
	}{
		{
			name:      "applies percentage discount",
			subtotal:  Money{Amount: 10000, Currency: "USD"},
			discounts: []Discount{{Type: DiscountPercentage, Value: 10}},
			want:      9000,
		},
		{
			name:      "applies fixed discount",
			subtotal:  Money{Amount: 10000, Currency: "USD"},
			discounts: []Discount{{Type: DiscountFixed, Value: 1500}},
			want:      8500,
		},
		{
			name:     "applies percentage before fixed",
			subtotal: Money{Amount: 10000, Currency: "USD"},
			discounts: []Discount{
				{Type: DiscountFixed, Value: 1000},
				{Type: DiscountPercentage, Value: 10},
			},
			want: 8000, // 10000 - 10% = 9000, then 9000 - 1000 = 8000
		},
		{
			name:      "never returns negative",
			subtotal:  Money{Amount: 1000, Currency: "USD"},
			discounts: []Discount{{Type: DiscountFixed, Value: 5000}},
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.ApplyDiscounts(tt.subtotal, tt.discounts)
			if result.Amount != tt.want {
				t.Errorf("want %d, got %d", tt.want, result.Amount)
			}
		})
	}
}

// Test helper
func newLineItem(priceCents int, currency string, quantity int) LineItem {
	return LineItem{
		ProductID: "prod-1",
		Price:     Money{Amount: priceCents, Currency: currency},
		Quantity:  quantity,
	}
}
```

### Integration with Use Cases

```go
// internal/app/place_order.go

// PlaceOrderService orchestrates the order placement use case.
type PlaceOrderService struct {
	orders     OrderRepository
	customers  CustomerRepository
	pricing    *PricingService
	shipping   *ShippingPolicyService
	validation *OrderValidationService
}

// NewPlaceOrderService creates a PlaceOrderService with its dependencies.
func NewPlaceOrderService(
	orders OrderRepository,
	customers CustomerRepository,
	pricing *PricingService,
	shipping *ShippingPolicyService,
	validation *OrderValidationService,
) *PlaceOrderService {
	return &PlaceOrderService{
		orders:     orders,
		customers:  customers,
		pricing:    pricing,
		shipping:   shipping,
		validation: validation,
	}
}

// PlaceOrderResult holds the outcome of placing an order.
type PlaceOrderResult struct {
	Success bool
	OrderID string
	Errors  []string
}

// Execute runs the place-order use case.
func (s *PlaceOrderService) Execute(ctx context.Context, req PlaceOrderRequest) (PlaceOrderResult, error) {
	customer, err := s.customers.FindByID(ctx, req.CustomerID)
	if err != nil {
		return PlaceOrderResult{}, fmt.Errorf("finding customer: %w", err)
	}
	if customer == nil {
		return PlaceOrderResult{}, errors.New("customer not found")
	}

	// Use domain services for business logic
	subtotal, err := s.pricing.CalculateSubtotal(req.Items)
	if err != nil {
		return PlaceOrderResult{}, fmt.Errorf("calculating subtotal: %w", err)
	}
	afterDiscounts := s.pricing.ApplyDiscounts(subtotal, req.Discounts)
	shippingOptions := s.shipping.GetAvailableOptions(
		&Order{total: afterDiscounts, items: req.Items},
		req.ShippingAddress,
	)
	shippingCost := shippingOptions[0].Cost

	order, err := NewOrder(customer.ID(), req.Items)
	if err != nil {
		return PlaceOrderResult{}, fmt.Errorf("creating order: %w", err)
	}

	// Validate using domain service
	validation := s.validation.ValidateOrder(order, customer)
	if !validation.IsValid {
		return PlaceOrderResult{Success: false, Errors: validation.Errors}, nil
	}

	if err := s.orders.Save(ctx, order); err != nil {
		return PlaceOrderResult{}, fmt.Errorf("saving order: %w", err)
	}

	_ = shippingCost // used for order total calculation
	return PlaceOrderResult{Success: true, OrderID: order.ID()}, nil
}
```
