# Data Refactorings

## Split Variable

**When**: Variable is assigned multiple times for different purposes (except loop variables and accumulators).

**Steps**:

1. Change variable name at first declaration
2. Change references up to next assignment
3. Declare new variable at next assignment
4. Repeat for each assignment

```go
// Before: temp used for two different things
temp := 2 * (height + width)
fmt.Println(temp) // perimeter
temp = height * width
fmt.Println(temp) // area

// After: Separate variables for separate purposes
perimeter := 2 * (height + width)
fmt.Println(perimeter)
area := height * width
fmt.Println(area)
```

## Replace Derived Variable with Query

**When**: Variable can be calculated from other data; eliminates mutable state.

**Steps**:

1. Identify all update points
2. Create method to calculate value
3. Use assertion to verify calculation matches
4. Replace variable reads with method call
5. Remove variable and updates

```go
// Before: discountedTotal updated manually
type ProductionPlan struct {
	production      float64
	discountedTotal float64
}

func (p *ProductionPlan) AddAdjustment(amount float64) {
	p.production += amount
	p.discountedTotal += amount * 0.9
}

func (p *ProductionPlan) DiscountedTotal() float64 {
	return p.discountedTotal
}

// After: Calculate when needed
type ProductionPlan struct {
	production float64
}

func (p *ProductionPlan) AddAdjustment(amount float64) {
	p.production += amount
}

func (p *ProductionPlan) DiscountedTotal() float64 {
	return p.production * 0.9
}
```

## Change Reference to Value

**When**: Object should have value semantics (compared by content, not identity).

**Steps**:

1. Check object is/can be immutable
2. Create equals method based on fields
3. Remove setter methods
4. Consider making fields unexported with no setters

```go
// Before: TelephoneNumber compared by pointer
type Person struct {
	telephoneNumber *TelephoneNumber
}

func (p *Person) SetOfficeAreaCode(value string) {
	p.telephoneNumber.AreaCode = value
}

// After: Immutable value object
type TelephoneNumber struct {
	areaCode string
	number   string
}

func NewTelephoneNumber(areaCode, number string) TelephoneNumber {
	return TelephoneNumber{areaCode: areaCode, number: number}
}

func (t TelephoneNumber) AreaCode() string { return t.areaCode }
func (t TelephoneNumber) Number() string   { return t.number }

func (t TelephoneNumber) Equal(other TelephoneNumber) bool {
	return t.areaCode == other.areaCode && t.number == other.number
}

type Person struct {
	telephoneNumber TelephoneNumber
}

func (p *Person) SetOfficeAreaCode(value string) {
	p.telephoneNumber = NewTelephoneNumber(value, p.telephoneNumber.Number())
}
```

## Change Value to Reference

**When**: Need to share single instance so updates are seen everywhere.

**Steps**:

1. Create repository for instances
2. Ensure constructor can look up correct instance
3. Change factory to return reference from repository

```go
// Before: Each Order has its own Customer copy
type Order struct {
	customer *Customer
}

func NewOrder(data OrderData) *Order {
	return &Order{customer: NewCustomer(data.CustomerID)}
}

// After: Orders share Customer references
var customerRepository = map[string]*Customer{}

func FindCustomer(id string) *Customer {
	if c, ok := customerRepository[id]; ok {
		return c
	}
	c := NewCustomer(id)
	customerRepository[id] = c
	return c
}

func NewOrder(data OrderData) *Order {
	return &Order{customer: FindCustomer(data.CustomerID)}
}
```

## Replace Loop with Named Function

**When**: Loop processes collection; a named function is clearer.

**Steps**:

1. Create variable for loop result
2. Extract loop body into a named function
3. Replace loop with function call

```go
// Before
var names []string
for _, person := range people {
	if person.Job == "programmer" {
		names = append(names, person.Name)
	}
}

// After
names := programmerNames(people)

func programmerNames(people []Person) []string {
	var names []string
	for _, p := range people {
		if p.Job == "programmer" {
			names = append(names, p.Name)
		}
	}
	return names
}
```

## Common Loop Patterns in Go

| Loop Pattern               | Go Idiom                                  |
| -------------------------- | ----------------------------------------- |
| Filter items               | `for range` + `if` + `append`             |
| Transform items            | `for range` + `append` transformed value  |
| Find single item           | `for range` + `if` + `return`             |
| Check if any match         | `for range` + `if` + `return true`        |
| Check if all match         | `for range` + `if !` + `return false`     |
| Accumulate to single value | `for range` + accumulator variable        |
| Flatten nested slices      | nested `for range` + `append`             |
