# Composition Refactorings

Go does not have inheritance. These refactorings use Go's embedding and composition to achieve similar goals.

## Pull Up Method (via Embedding)

**When**: Methods on multiple structs do the same thing; share via embedding.

**Steps**:

1. Check methods are identical (or make them so)
2. Check method signatures match
3. Create shared struct with the common method
4. Embed shared struct in each type
5. Remove duplicate methods
6. Test

```go
// Before: Duplicate in multiple structs
type Engineer struct {
	monthlyCost float64
}

func (e *Engineer) AnnualCost() float64 {
	return e.monthlyCost * 12
}

type Salesman struct {
	monthlyCost float64
}

func (s *Salesman) AnnualCost() float64 {
	return s.monthlyCost * 12
}

// After: Extract shared interface + embedding for shared logic
type CostCalculator interface {
	MonthlyCost() float64
	AnnualCost() float64
}

type baseCost struct {
	monthlyCost float64
}

func (b *baseCost) MonthlyCost() float64 { return b.monthlyCost }
func (b *baseCost) AnnualCost() float64  { return b.monthlyCost * 12 }

type Engineer struct {
	baseCost
}

type Salesman struct {
	baseCost
}
```

## Pull Up Field (via Embedding)

**When**: Multiple structs have the same field.

**Steps**:

1. Check fields are used similarly
2. Create shared struct with the field
3. Embed in each struct
4. Remove duplicate fields

```go
// Before
type Engineer struct {
	name string
}

type Salesman struct {
	name string
}

// After
type employee struct {
	name string
}

type Engineer struct {
	employee
}

type Salesman struct {
	employee
}
```

## Pull Up Constructor Body (via Constructor Helper)

**When**: Multiple constructors have common initialization code.

**Steps**:

1. Create helper function for common initialization
2. Call helper from each constructor

```go
// Before: Repeated initialization
type Engineer struct {
	name           string
	specialization string
}

func NewEngineer(name, specialization string) *Engineer {
	return &Engineer{name: name, specialization: specialization}
}

type Manager struct {
	name       string
	department string
}

func NewManager(name, department string) *Manager {
	return &Manager{name: name, department: department}
}

// After: Shared base via embedding
type employee struct {
	name string
}

type Engineer struct {
	employee
	specialization string
}

func NewEngineer(name, specialization string) *Engineer {
	return &Engineer{
		employee:       employee{name: name},
		specialization: specialization,
	}
}

type Manager struct {
	employee
	department string
}

func NewManager(name, department string) *Manager {
	return &Manager{
		employee:   employee{name: name},
		department: department,
	}
}
```

## Push Down Method

**When**: Method only relevant to a specific type; remove from shared struct.

**Steps**:

1. Add method to the specific type(s) that need it
2. Remove from shared struct

```go
// Before: Only Salesman uses quota
type employee struct {
	quota float64
}

func (e *employee) Quota() float64 { return e.quota }

type Salesman struct {
	employee
}

// After
type employee struct{}

type Salesman struct {
	employee
	quota float64
}

func (s *Salesman) Quota() float64 { return s.quota }
```

## Push Down Field

**When**: Field only used by specific type.

```go
// Before
type employee struct {
	quota float64 // Only used by Salesman
}

// After
type employee struct{}

type Salesman struct {
	employee
	quota float64
}
```

## Replace Embedding with Delegate

**When**: Embedding doesn't fit; need more flexibility.

**Why**:

- Embedding exposes all methods, which may not be desired
- Embedding creates tight coupling
- Relationship isn't true is-a

**Steps**:

1. Create delegate struct for the behavior
2. Add delegate field to the struct
3. Move embedded methods to delegate
4. Replace embedding with explicit delegation

```go
// Before: Booking embedding limits flexibility
type Booking struct {
	show Show
	date time.Time
}

func (b *Booking) HasTalkback() bool { return false }
func (b *Booking) BasePrice() float64 { return b.show.Price }

type PremiumBooking struct {
	Booking
	extras Extras
}

func (p *PremiumBooking) HasTalkback() bool {
	return p.show.Talkback && !p.isPeakDay()
}

func (p *PremiumBooking) BasePrice() float64 {
	return math.Round(p.Booking.BasePrice() + p.extras.PremiumFee)
}

// After: Delegate provides flexibility
type Booking struct {
	show            Show
	date            time.Time
	premiumDelegate *PremiumBookingDelegate
}

func (b *Booking) BePremium(extras Extras) {
	b.premiumDelegate = &PremiumBookingDelegate{host: b, extras: extras}
}

func (b *Booking) HasTalkback() bool {
	if b.premiumDelegate != nil {
		return b.premiumDelegate.HasTalkback()
	}
	return false
}

func (b *Booking) BasePrice() float64 {
	base := b.show.Price
	if b.premiumDelegate != nil {
		return b.premiumDelegate.AdjustPrice(base)
	}
	return base
}

type PremiumBookingDelegate struct {
	host   *Booking
	extras Extras
}

func (d *PremiumBookingDelegate) HasTalkback() bool {
	return d.host.show.Talkback && !d.host.isPeakDay()
}

func (d *PremiumBookingDelegate) AdjustPrice(base float64) float64 {
	return math.Round(base + d.extras.PremiumFee)
}
```

## Extract Shared Interface

**When**: Structs share features; an interface captures the common contract.

**Steps**:

1. Identify common methods across structs
2. Define interface with those methods
3. Ensure each struct satisfies the interface

```go
// Before: Department and Employee share features but have no common type
type Employee struct {
	name       string
	annualCost float64
}

func (e *Employee) Name() string         { return e.name }
func (e *Employee) AnnualCost() float64  { return e.annualCost }
func (e *Employee) MonthlySpend() float64 { return e.annualCost / 12 }

type Department struct {
	name  string
	staff []*Employee
}

func (d *Department) Name() string { return d.name }
func (d *Department) AnnualCost() float64 {
	total := 0.0
	for _, e := range d.staff {
		total += e.AnnualCost()
	}
	return total
}
func (d *Department) MonthlySpend() float64 { return d.AnnualCost() / 12 }

// After: Extract shared interface
type Party interface {
	Name() string
	AnnualCost() float64
	MonthlySpend() float64
}

// Employee and Department both satisfy Party implicitly
```

## When to Prefer Delegation Over Embedding

| Use Embedding When                   | Use Delegation When                 |
| ------------------------------------ | ----------------------------------- |
| True is-a relationship               | Has-a or uses-a relationship        |
| Outer struct uses most of embedded   | Only needs part of behavior         |
| Outer struct is truly a specialization | Need runtime flexibility          |
| Hierarchy is stable                  | Behavior might change independently |
| Want promoted methods                | Want to hide internal methods       |
