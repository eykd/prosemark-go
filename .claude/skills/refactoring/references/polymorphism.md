# Polymorphism Refactorings

## Replace Conditional with Polymorphism

**When**: Same switch/if-else checks type in multiple places.

**Steps**:

1. Define interface for the polymorphic behavior
2. Create struct types implementing the interface
3. Use factory function for object creation
4. Replace conditional with interface method call

```go
// Before: Switch on type in multiple places
func plumage(bird BirdData) string {
	switch bird.Type {
	case "EuropeanSwallow":
		return "average"
	case "AfricanSwallow":
		if bird.NumberOfCoconuts > 2 {
			return "tired"
		}
		return "average"
	case "NorwegianBlueParrot":
		if bird.Voltage > 100 {
			return "scorched"
		}
		return "beautiful"
	default:
		return "unknown"
	}
}

func airSpeed(bird BirdData) float64 {
	switch bird.Type {
	case "EuropeanSwallow":
		return 35
	case "AfricanSwallow":
		return 40 - 2*float64(bird.NumberOfCoconuts)
	case "NorwegianBlueParrot":
		if bird.IsNailed {
			return 0
		}
		return 10 + bird.Voltage/10
	default:
		return 0
	}
}

// After: Interface + concrete types
type Bird interface {
	Plumage() string
	AirSpeed() float64
}

type EuropeanSwallow struct{}

func (e EuropeanSwallow) Plumage() string  { return "average" }
func (e EuropeanSwallow) AirSpeed() float64 { return 35 }

type AfricanSwallow struct {
	numberOfCoconuts int
}

func (a AfricanSwallow) Plumage() string {
	if a.numberOfCoconuts > 2 {
		return "tired"
	}
	return "average"
}

func (a AfricanSwallow) AirSpeed() float64 {
	return 40 - 2*float64(a.numberOfCoconuts)
}

type NorwegianBlueParrot struct {
	voltage  float64
	isNailed bool
}

func (n NorwegianBlueParrot) Plumage() string {
	if n.voltage > 100 {
		return "scorched"
	}
	return "beautiful"
}

func (n NorwegianBlueParrot) AirSpeed() float64 {
	if n.isNailed {
		return 0
	}
	return 10 + n.voltage/10
}

// Factory
func NewBird(data BirdData) (Bird, error) {
	switch data.Type {
	case "EuropeanSwallow":
		return EuropeanSwallow{}, nil
	case "AfricanSwallow":
		return AfricanSwallow{numberOfCoconuts: data.NumberOfCoconuts}, nil
	case "NorwegianBlueParrot":
		return NorwegianBlueParrot{voltage: data.Voltage, isNailed: data.IsNailed}, nil
	default:
		return nil, fmt.Errorf("unknown bird type: %s", data.Type)
	}
}
```

## Decompose Conditional

**When**: Complex condition obscures intent; first step toward polymorphism.

**Steps**:

1. Extract condition into function with clear name
2. Extract then-branch into function
3. Extract else-branch into function

```go
// Before
if date.Before(summerStart) || date.After(summerEnd) {
	charge = float64(quantity)*winterRate + winterServiceCharge
} else {
	charge = float64(quantity) * summerRate
}

// After
if isSummer(date) {
	charge = summerCharge(quantity)
} else {
	charge = winterCharge(quantity)
}

func isSummer(date time.Time) bool {
	return !date.Before(summerStart) && !date.After(summerEnd)
}

func summerCharge(quantity int) float64 {
	return float64(quantity) * summerRate
}

func winterCharge(quantity int) float64 {
	return float64(quantity)*winterRate + winterServiceCharge
}
```

## Replace Type Code with Interface Implementations

**When**: Type code affects behavior; enables polymorphism.

**Steps**:

1. Self-encapsulate type code if not already
2. Define interface for the varying behavior
3. Create struct for each type code value implementing the interface
4. Create factory function to return appropriate implementation

```go
// Before: Type code as field
type Employee struct {
	employeeType string // "engineer", "salesman", "manager"
	salary       float64
	sales        float64
	teamBonus    float64
}

func (e *Employee) Bonus() float64 {
	switch e.employeeType {
	case "engineer":
		return e.salary * 0.1
	case "salesman":
		return e.sales * 0.15
	case "manager":
		return e.salary*0.2 + e.teamBonus
	default:
		return 0
	}
}

// After: Interface + implementations
type Employee interface {
	Bonus() float64
}

type Engineer struct {
	salary float64
}

func (e *Engineer) Bonus() float64 { return e.salary * 0.1 }

type Salesman struct {
	sales float64
}

func (s *Salesman) Bonus() float64 { return s.sales * 0.15 }

type Manager struct {
	salary    float64
	teamBonus float64
}

func (m *Manager) Bonus() float64 { return m.salary*0.2 + m.teamBonus }

func NewEmployee(employeeType string, data EmployeeData) (Employee, error) {
	switch employeeType {
	case "engineer":
		return &Engineer{salary: data.Salary}, nil
	case "salesman":
		return &Salesman{sales: data.Sales}, nil
	case "manager":
		return &Manager{salary: data.Salary, teamBonus: data.TeamBonus}, nil
	default:
		return nil, fmt.Errorf("unknown employee type: %s", employeeType)
	}
}
```

## Introduce Assertion

**When**: Assumptions about state should be explicit.

**Steps**:

1. Identify assumption that must be true
2. Add check and return error or panic for invariants
3. Use for invariants, not user input validation

```go
// Before: Implicit assumption about discount
func applyDiscount(price, discountRate float64) float64 {
	return price - price*discountRate
}

// After: Explicit check
func applyDiscount(price, discountRate float64) (float64, error) {
	if discountRate < 0 || discountRate > 1 {
		return 0, fmt.Errorf("discount rate must be 0-1, got %f", discountRate)
	}
	return price - price*discountRate, nil
}
```

## When NOT to Use Polymorphism

- **One-off conditional**: If switch appears only once, leave it
- **Simple cases**: Don't create interface for 2-3 simple cases
- **External data**: Type info comes from JSON/DB, keep factory switch
- **Performance critical**: Interface dispatch has overhead
