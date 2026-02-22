# Simplification Refactorings

## Inline Function

**When**: Function body is as clear as name, or just delegates.

**Steps**:

1. Check not part of an interface implementation
2. Find all callers
3. Replace each call with body
4. Remove function
5. Test after each replacement

```go
// Before: Unnecessary indirection
func rating(driver Driver) int {
	if moreThanFiveLateDeliveries(driver) {
		return 2
	}
	return 1
}

func moreThanFiveLateDeliveries(driver Driver) bool {
	return driver.NumberOfLateDeliveries > 5
}

// After
func rating(driver Driver) int {
	if driver.NumberOfLateDeliveries > 5 {
		return 2
	}
	return 1
}
```

## Inline Struct

**When**: Struct no longer justifies its existence after refactoring.

**Steps**:

1. Move all methods and fields to target struct
2. Update references to use target
3. Remove empty struct

```go
// Before: TrackingInformation has become trivial
type Shipment struct {
	tracking *TrackingInformation
}

func (s *Shipment) TrackingInfo() string {
	return s.tracking.Display()
}

type TrackingInformation struct {
	ShippingCompany string
	TrackingNumber  string
}

func (t *TrackingInformation) Display() string {
	return t.ShippingCompany + ": " + t.TrackingNumber
}

// After: Merged into Shipment
type Shipment struct {
	shippingCompany string
	trackingNumber  string
}

func (s *Shipment) TrackingInfo() string {
	return s.shippingCompany + ": " + s.trackingNumber
}
```

## Remove Dead Code

**When**: Code is never executed. Trust version control for history.

**Steps**:

1. Use tools/IDE to find unused code
2. Delete it
3. Test

```go
// Before: HasDiscount is never called
type Order struct {
	items        []Item
	discountCode string
}

func (o *Order) Total() float64 {
	total := 0.0
	for _, item := range o.items {
		total += item.Price
	}
	return total
}

// Dead code - delete it
func (o *Order) HasDiscount() bool {
	return o.discountCode != ""
}

// After
type Order struct {
	items []Item
}

func (o *Order) Total() float64 {
	total := 0.0
	for _, item := range o.items {
		total += item.Price
	}
	return total
}
```

## Collapse Hierarchy

**When**: Embedded struct and outer struct are too similar.

**Steps**:

1. Choose which struct to remove
2. Merge fields and methods
3. Remove empty struct
4. Update references

```go
// Before: Employee and Salesman have almost identical behavior
type Employee struct {
	name   string
	salary float64
}

type Salesman struct {
	Employee
	sales float64
}

func (s *Salesman) Bonus() float64 {
	return s.sales * 0.1
}

// After: If all employees can have sales, collapse
type Employee struct {
	name   string
	salary float64
	sales  float64
}

func (e *Employee) Bonus() float64 {
	return e.sales * 0.1
}
```

## Remove Middle Man

**When**: Struct mostly just delegates to another struct.

**Steps**:

1. Expose delegate object
2. For each delegating method, adjust client to call delegate directly
3. Remove delegating methods

```go
// Before: Person just delegates to Department
type Person struct {
	department *Department
}

func (p *Person) Department() *Department { return p.department }
func (p *Person) Manager() *Employee      { return p.department.Manager() }
func (p *Person) Budget() float64         { return p.department.Budget() }
func (p *Person) HeadCount() int          { return p.department.HeadCount() }

// After: Let clients talk to Department directly
type Person struct {
	department *Department
}

func (p *Person) Department() *Department { return p.department }

// Client
manager := person.Department().Manager()
```

## Replace Embedding with Delegate

**When**: Embedding isn't a true is-a relationship; composition is better.

```go
// Before: Stack embeds List but isn't really a List
type Stack struct {
	List // exposes all List methods, which is wrong
}

func (s *Stack) Push(item int) {
	s.Append(item)
}

func (s *Stack) Pop() int {
	return s.RemoveLast()
}

// After: Stack uses List via delegation
type Stack struct {
	storage *List
}

func (s *Stack) Push(item int) {
	s.storage.Append(item)
}

func (s *Stack) Pop() int {
	return s.storage.RemoveLast()
}
// Note: List methods like Get(), Set() are NOT exposed
```

## Substitute Algorithm

**When**: Simpler algorithm becomes apparent.

**Steps**:

1. Ensure algorithm is in separate function
2. Prepare new algorithm
3. Test new algorithm independently
4. Replace old with new
5. Test

```go
// Before: Complex search
func findPerson(people []Person) *Person {
	for i := range people {
		if people[i].Name == "Don" {
			return &people[i]
		}
		if people[i].Name == "John" {
			return &people[i]
		}
		if people[i].Name == "Kent" {
			return &people[i]
		}
	}
	return nil
}

// After: Simpler approach
func findPerson(people []Person) *Person {
	candidates := map[string]bool{"Don": true, "John": true, "Kent": true}
	for i := range people {
		if candidates[people[i].Name] {
			return &people[i]
		}
	}
	return nil
}
```

## Signs of Speculative Generality

Remove these if not actually used:

- Interfaces with only one implementation (and no planned extensions)
- Unused parameters
- Functions only called by tests
- Type parameters with only one instantiation
- Exported types/functions that are only used within the same package
