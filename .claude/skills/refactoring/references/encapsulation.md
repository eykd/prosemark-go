# Encapsulation Refactorings

## Encapsulate Variable

**When**: Data is accessed widely; need to control access, add validation, or prepare for restructuring.

**Steps**:

1. Create getter and setter methods
2. Replace all direct references with method calls
3. Make fields unexported
4. Test

```go
// Before
var DefaultOwner = Owner{FirstName: "Martin", LastName: "Fowler"}
spaceship.Owner = DefaultOwner

// After
var defaultOwner = Owner{FirstName: "Martin", LastName: "Fowler"}

func GetDefaultOwner() Owner {
	return defaultOwner
}

func SetDefaultOwner(owner Owner) {
	defaultOwner = owner
}

spaceship.Owner = GetDefaultOwner()
```

## Encapsulate Record

**When**: Data structures (structs, maps) need controlled access.

**Steps**:

1. Create struct with unexported fields
2. Provide methods to get/set values
3. Replace raw struct usage with encapsulated type
4. Consider making immutable

```go
// Before
type Organization struct {
	Name    string
	Country string
}

org := Organization{Name: "Acme", Country: "US"}
org.Name = "New Name"

// After
type Organization struct {
	name    string
	country string
}

func NewOrganization(name, country string) *Organization {
	return &Organization{name: name, country: country}
}

func (o *Organization) Name() string        { return o.name }
func (o *Organization) SetName(name string)  { o.name = name }
func (o *Organization) Country() string      { return o.country }
```

## Encapsulate Collection

**When**: Collection is exposed directly, allowing uncontrolled modification.

**Steps**:

1. Add methods to add/remove items
2. Return a copy from getter
3. Never return mutable reference to internal slice

```go
// Before
type Person struct {
	Courses []Course
}
person.Courses = append(person.Courses, newCourse)

// After
type Person struct {
	courses []Course
}

func (p *Person) Courses() []Course {
	result := make([]Course, len(p.courses))
	copy(result, p.courses)
	return result
}

func (p *Person) AddCourse(course Course) {
	p.courses = append(p.courses, course)
}

func (p *Person) RemoveCourse(course Course) {
	for i, c := range p.courses {
		if c == course {
			p.courses = append(p.courses[:i], p.courses[i+1:]...)
			return
		}
	}
}
```

## Replace Primitive with Object

**When**: Primitive types (string, int) represent domain concepts.

**Steps**:

1. Create type for the value
2. Replace primitive with new type
3. Move related behavior into type's methods
4. Consider making immutable

```go
// Before
func deliveryDate(order Order) time.Time {
	if order.Priority == "high" {
		return order.PlacedOn.AddDate(0, 0, 1)
	}
	return order.PlacedOn.AddDate(0, 0, 3)
}

// After
type Priority struct {
	value string
}

func NewPriority(value string) (Priority, error) {
	valid := map[string]bool{"low": true, "normal": true, "high": true, "rush": true}
	if !valid[value] {
		return Priority{}, fmt.Errorf("invalid priority: %s", value)
	}
	return Priority{value: value}, nil
}

func (p Priority) HigherThan(other Priority) bool {
	levels := map[string]int{"low": 0, "normal": 1, "high": 2, "rush": 3}
	return levels[p.value] > levels[other.value]
}
```

## Hide Delegate

**When**: Client navigates through one object to get to another (message chains).

**Steps**:

1. Create delegating method on server for each delegate method needed
2. Adjust client to call server
3. Remove client's knowledge of delegate

```go
// Before
manager := person.Department().Manager()

// After (add method to Person)
func (p *Person) Manager() *Employee {
	return p.department.Manager()
}

// Client code
manager := person.Manager()
```

## Introduce Special Case (Null Object)

**When**: Same nil/special case checks appear everywhere.

**Steps**:

1. Create special-case type satisfying same interface
2. Return special-case instance instead of nil
3. Move special-case behavior into special type

```go
// Before
func customerName(site Site) string {
	customer := site.Customer()
	if customer == nil {
		return "occupant"
	}
	return customer.Name()
}

// After
type Customer interface {
	Name() string
	BillingPlan() BillingPlan
}

type UnknownCustomer struct{}

func (u UnknownCustomer) Name() string            { return "occupant" }
func (u UnknownCustomer) BillingPlan() BillingPlan { return BasicBillingPlan() }

func customerName(site Site) string {
	return site.Customer().Name() // Works for real and unknown customers
}
```
