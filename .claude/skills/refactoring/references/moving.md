# Moving Refactorings

## Move Function

**When**: Function uses elements from another context more than its own (Feature Envy).

**Steps**:

1. Examine function's context usage
2. Copy function to target context
3. Adjust for new location (rename, change parameters)
4. Set up delegation from old to new
5. Test, then remove old or leave as delegation

```go
// Before: trackSummary uses totalDistance more than its own struct
type GPS struct{}

func (g *GPS) TrackSummary(points []Point) Summary {
	totalDist := g.calculateDistance(points)
	pace := totalDist / float64(len(points))
	return Summary{Distance: totalDist, Pace: pace}
}

func (g *GPS) calculateDistance(points []Point) float64 {
	total := 0.0
	for i := 1; i < len(points); i++ {
		total += points[i].DistanceTo(points[i-1])
	}
	return total
}

// After: Move to where data lives
type Track struct {
	points []Point
}

func (t *Track) TotalDistance() float64 {
	total := 0.0
	for i := 1; i < len(t.points); i++ {
		total += t.points[i].DistanceTo(t.points[i-1])
	}
	return total
}

func (t *Track) Pace() float64 {
	return t.TotalDistance() / float64(len(t.points))
}
```

## Move Field

**When**: Field is used more by another struct, or data structures are too coupled.

**Steps**:

1. If exported, use Encapsulate Variable first
2. Create field in target
3. Adjust references to use target field
4. Remove source field

```go
// Before: discount relates more to CustomerContract
type Customer struct {
	DiscountRate float64
	Contract     *CustomerContract
}

// After
type Customer struct {
	Contract *CustomerContract
}

func (c *Customer) DiscountRate() float64 {
	return c.Contract.DiscountRate
}

type CustomerContract struct {
	DiscountRate float64
}
```

## Move Statements into Function

**When**: Same code appears in multiple callers of a function.

**Steps**:

1. If statements aren't adjacent, use Slide Statements first
2. Copy statements into function body
3. Test
4. Remove statements from callers

```go
// Before
func renderPerson(person Person) string {
	var result []string
	result = append(result, fmt.Sprintf("<p>%s</p>", person.Name))
	result = append(result, renderPhoto(person.Photo))
	result = append(result, fmt.Sprintf("<p>title: %s</p>", person.Photo.Title))
	return strings.Join(result, "\n")
}

// After: title rendering moved into renderPhoto
func renderPhoto(photo Photo) string {
	return fmt.Sprintf("<img src=\"%s\">\n<p>title: %s</p>", photo.URL, photo.Title)
}
```

## Move Statements to Callers

**When**: Function does too much; some behavior should vary by caller.

**Steps**:

1. Use Slide Statements to move varying code to function exit
2. Copy varying code to each caller
3. Remove from function

```go
// Before: emitPhotoData always outputs location, but not all callers want it
func emitPhotoData(photo Photo) string {
	return fmt.Sprintf("<p>title: %s</p>\n<p>location: %s</p>", photo.Title, photo.Location)
}

// After
func emitPhotoData(photo Photo) string {
	return fmt.Sprintf("<p>title: %s</p>", photo.Title)
}

// Callers that need location add it themselves
fmt.Println(emitPhotoData(photo) + fmt.Sprintf("\n<p>location: %s</p>", photo.Location))
```

## Slide Statements

**When**: Related code is scattered; group for Extract Function or clarity.

**Steps**:

1. Identify target position
2. Check for dependencies that would break if moved
3. Move code to target
4. Test

```go
// Before
pricingPlan := retrievePricingPlan()
order := retrieveOrder()
var charge float64
chargePerUnit := pricingPlan.Unit

// After: Group pricing-related code
pricingPlan := retrievePricingPlan()
chargePerUnit := pricingPlan.Unit
order := retrieveOrder()
var charge float64
```

## Replace Inline Code with Function Call

**When**: Code duplicates logic that exists in a library or elsewhere.

```go
// Before
hasDiscount := false
for _, customer := range customers {
	if customer.IsPremium {
		hasDiscount = true
		break
	}
}

// After
hasDiscount := hasPremiumCustomer(customers)

func hasPremiumCustomer(customers []Customer) bool {
	for _, c := range customers {
		if c.IsPremium {
			return true
		}
	}
	return false
}
```

## Split Loop

**When**: Loop does multiple unrelated things.

**Steps**:

1. Copy loop
2. Remove different operations from each copy
3. Test
4. Consider Extract Function on each loop

```go
// Before
youngest := math.MaxInt
totalSalary := 0.0
for _, p := range people {
	if p.Age < youngest {
		youngest = p.Age
	}
	totalSalary += p.Salary
}

// After
youngest := math.MaxInt
for _, p := range people {
	if p.Age < youngest {
		youngest = p.Age
	}
}

totalSalary := 0.0
for _, p := range people {
	totalSalary += p.Salary
}

// Even better: Extract into named functions
youngest := findYoungestAge(people)
totalSalary := sumSalaries(people)
```
