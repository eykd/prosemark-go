# API Refactorings

## Introduce Parameter Object

**When**: Same parameters travel together; group reveals deeper abstraction.

**Steps**:

1. Create struct for grouped parameters
2. Add parameter of new type
3. Replace individual parameters
4. Consider moving behavior into new struct's methods

```go
// Before: Range parameters appear together everywhere
func amountInvoicedIn(start, end time.Time) float64 { /* ... */ }
func amountReceivedIn(start, end time.Time) float64 { /* ... */ }
func amountOverdueIn(start, end time.Time) float64  { /* ... */ }

// After: DateRange reveals domain concept
type DateRange struct {
	Start time.Time
	End   time.Time
}

func (r DateRange) Contains(date time.Time) bool {
	return !date.Before(r.Start) && !date.After(r.End)
}

func amountInvoicedIn(r DateRange) float64 { /* ... */ }
func amountReceivedIn(r DateRange) float64 { /* ... */ }
func amountOverdueIn(r DateRange) float64  { /* ... */ }
```

## Remove Flag Argument

**When**: Boolean/enum parameter changes function behavior; separate functions are clearer.

**Steps**:

1. Create explicit function for each flag value
2. Replace callers with explicit version
3. Remove original or leave as unexported wrapper

```go
// Before: What does true mean?
setDimension(name, 10, true)

// After: Intent is clear
SetWidth(10)
SetHeight(10)

// Implementation
func SetWidth(value float64) {
	setDimension("width", value)
}

func SetHeight(value float64) {
	setDimension("height", value)
}
```

### Alternative: Functional Options Pattern

```go
// Before: Multiple optional booleans
func NewServer(addr string, tls bool, timeout int, maxConns int) *Server { /* ... */ }

// After: Functional options
type ServerOption func(*Server)

func WithTLS() ServerOption {
	return func(s *Server) { s.tls = true }
}

func WithTimeout(d time.Duration) ServerOption {
	return func(s *Server) { s.timeout = d }
}

func WithMaxConns(n int) ServerOption {
	return func(s *Server) { s.maxConns = n }
}

func NewServer(addr string, opts ...ServerOption) *Server {
	s := &Server{addr: addr}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Caller: intent is clear
srv := NewServer(":8080", WithTLS(), WithTimeout(30*time.Second))
```

## Preserve Whole Object

**When**: Passing multiple values extracted from single object.

**Steps**:

1. Add parameter for whole object
2. Adjust function to extract values
3. Update callers to pass whole object
4. Consider if dependency is acceptable

```go
// Before: Extracting values just to pass them
low := room.DaysTempRange.Low
high := room.DaysTempRange.High
if plan.WithinRange(low, high) {
}

// After: Pass the whole object
if plan.WithinRange(room.DaysTempRange) {
}

type HeatingPlan struct {
	temperatureRange TempRange
}

func (h *HeatingPlan) WithinRange(r TempRange) bool {
	return r.Low >= h.temperatureRange.Low && r.High <= h.temperatureRange.High
}
```

## Replace Parameter with Query

**When**: Parameter can be determined from other parameters or context.

**Steps**:

1. Extract calculation if needed
2. Replace parameter references with calculation
3. Remove parameter from declaration and calls

```go
// Before: quantity can be derived
func (p *Pricing) FinalPrice(basePrice float64, discountLevel int, quantity int) float64 {
	return basePrice * (1 - p.discountFor(discountLevel, quantity))
}

// After: Calculate quantity internally
func (p *Pricing) FinalPrice(basePrice float64, discountLevel int) float64 {
	quantity := p.order.Quantity
	return basePrice * (1 - p.discountFor(discountLevel, quantity))
}
```

## Replace Query with Parameter

**When**: Need to reduce dependencies or make function more flexible.

**Steps**:

1. Extract calculation if needed
2. Add parameter to function
3. Replace internal query with parameter
4. Update callers to pass value

```go
// Before: Method depends on global thermostat
type HeatingPlan struct {
	max float64
	min float64
}

func (h *HeatingPlan) TargetTemperature() float64 {
	selected := thermostat.SelectedTemperature()
	if selected > h.max {
		return h.max
	}
	if selected < h.min {
		return h.min
	}
	return selected
}

// After: Caller provides temperature
func (h *HeatingPlan) TargetTemperature(selectedTemp float64) float64 {
	if selectedTemp > h.max {
		return h.max
	}
	if selectedTemp < h.min {
		return h.min
	}
	return selectedTemp
}

// Caller
plan.TargetTemperature(thermostat.SelectedTemperature())
```

## Separate Query from Modifier

**When**: Function returns value AND has side effects.

**Steps**:

1. Copy function for query version
2. Remove side effects from query
3. Remove return from modifier
4. Replace callers: query for value, modifier for effect

```go
// Before: getTotalAndSendBill does two things
func getTotalAndSendBill(orders []Order) float64 {
	total := 0.0
	for _, o := range orders {
		total += o.Amount
	}
	sendBill(total)
	return total
}

// After: Separate concerns
func getTotal(orders []Order) float64 {
	total := 0.0
	for _, o := range orders {
		total += o.Amount
	}
	return total
}

func sendBill(orders []Order) {
	sendBillEmail(getTotal(orders))
}

// Caller
total := getTotal(orders)
sendBill(orders)
```

## Remove Setting Method

**When**: Field should be set only at construction time.

**Steps**:

1. Check field is only set in constructor
2. Add to constructor parameters if needed
3. Remove setter method
4. Make field unexported

```go
// Before: id can be changed after creation
type Person struct {
	id string
}

func (p *Person) ID() string       { return p.id }
func (p *Person) SetID(id string)  { p.id = id }

// After: Immutable after construction
type Person struct {
	id string
}

func NewPerson(id string) *Person {
	return &Person{id: id}
}

func (p *Person) ID() string { return p.id }
```

## Split Dual-Purpose Parameter

**When**: A single parameter is used for two logically distinct purposes (e.g., finding existing data AND computing new values). These purposes may align initially but diverge in recursive or iterative contexts.

**Symptom**: A recursive function passes `oldX` to itself, but the caller's context has changed `X` to `newX`. The recursion uses `oldX` for one purpose but needs `newX` for the other.

**Steps**:

1. Identify the two roles the parameter serves
2. Add a second parameter for the second role
3. At the top-level call site, both parameters are the same value
4. In recursive/iterative calls, they may diverge

```go
// Before: parentMP serves two purposes
func compact(parsed []File, parentMP string) map[string]string {
    for i, oldChild := range children {
        newChild := buildPath(parentMP, nums[i])  // Uses parentMP as new prefix
        // ...
        compact(parsed, oldChild)  // Passes old path — but newChild is the correct new prefix
    }
}

// After: Separate old (for matching) from new (for generating)
func compact(parsed []File, oldParentMP, newParentMP string) map[string]string {
    for i, oldChild := range children {
        newChild := buildPath(newParentMP, nums[i])  // Correct new prefix
        // ...
        compact(parsed, oldChild, newChild)  // Both old and new propagated
    }
}

// Top-level call: both are the same
compact(parsed, selector, selector)
```
