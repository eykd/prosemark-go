# Value Objects

Value objects are immutable, defined by their attributes, and have no identity.

## Table of Contents

- [Characteristics](#characteristics)
- [Patterns](#patterns)
- [Common Value Objects](#common-value-objects)
- [Equality and Comparison](#equality-and-comparison)
- [Testing Value Objects](#testing-value-objects)

## Characteristics

| Characteristic   | Description                                        |
| ---------------- | -------------------------------------------------- |
| Immutable        | State never changes after creation                 |
| No identity      | Two instances with same values are interchangeable |
| Self-validating  | Constructor rejects invalid values                 |
| Side-effect free | Methods return new instances                       |

## Patterns

### Basic Value Object

```go
// Email represents a validated email address.
type Email struct {
	value string
}

// NewEmail creates an Email, normalizing and validating the input.
func NewEmail(value string) (Email, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if !isValidEmail(normalized) {
		return Email{}, errors.New("invalid email format")
	}
	return Email{value: normalized}, nil
}

func isValidEmail(email string) bool {
	parts := strings.SplitN(email, "@", 2)
	return len(parts) == 2 && parts[0] != "" && strings.Contains(parts[1], ".")
}

// String returns the email address string.
func (e Email) String() string { return e.value }

// Domain returns the domain portion of the email.
func (e Email) Domain() string {
	return strings.SplitN(e.value, "@", 2)[1]
}

// Equal reports whether two Email values are identical.
func (e Email) Equal(other Email) bool {
	return e.value == other.value
}
```

### Enum-Style Value Object

```go
// TaskStatus represents the state of a task.
type TaskStatus int

const (
	TaskStatusPending    TaskStatus = iota
	TaskStatusInProgress
	TaskStatusCompleted
)

// String returns the string representation of the status.
func (s TaskStatus) String() string {
	switch s {
	case TaskStatusPending:
		return "pending"
	case TaskStatusInProgress:
		return "in_progress"
	case TaskStatusCompleted:
		return "completed"
	default:
		return "unknown"
	}
}

// ParseTaskStatus converts a string to a TaskStatus.
func ParseTaskStatus(value string) (TaskStatus, error) {
	switch value {
	case "pending":
		return TaskStatusPending, nil
	case "in_progress":
		return TaskStatusInProgress, nil
	case "completed":
		return TaskStatusCompleted, nil
	default:
		return 0, fmt.Errorf("invalid status: %s", value)
	}
}

// IsPending reports whether the status is pending.
func (s TaskStatus) IsPending() bool { return s == TaskStatusPending }

// IsCompleted reports whether the status is completed.
func (s TaskStatus) IsCompleted() bool { return s == TaskStatusCompleted }

// CanTransitionTo reports whether a transition to the target status is allowed.
func (s TaskStatus) CanTransitionTo(target TaskStatus) bool {
	switch s {
	case TaskStatusPending:
		return target == TaskStatusInProgress
	case TaskStatusInProgress:
		return target == TaskStatusCompleted || target == TaskStatusPending
	default:
		return false // Completed is terminal
	}
}
```

### Composite Value Object

```go
// Money represents a monetary amount in cents with a currency code.
type Money struct {
	Amount   int    // cents
	Currency string // 3-letter ISO 4217 code
}

// NewMoney creates a Money value, validating inputs.
func NewMoney(amount int, currency string) (Money, error) {
	if amount < 0 {
		return Money{}, errors.New("amount cannot be negative")
	}
	matched, _ := regexp.MatchString(`^[A-Z]{3}$`, currency)
	if !matched {
		return Money{}, errors.New("currency must be 3-letter ISO code")
	}
	return Money{Amount: amount, Currency: currency}, nil
}

// MoneyZero returns a zero Money for the given currency.
func MoneyZero(currency string) Money {
	return Money{Amount: 0, Currency: currency}
}

// Add returns a new Money with the sum. Returns error on currency mismatch.
func (m Money) Add(other Money) (Money, error) {
	if err := m.assertSameCurrency(other); err != nil {
		return Money{}, err
	}
	return Money{Amount: m.Amount + other.Amount, Currency: m.Currency}, nil
}

// Subtract returns a new Money with the difference.
func (m Money) Subtract(other Money) (Money, error) {
	if err := m.assertSameCurrency(other); err != nil {
		return Money{}, err
	}
	if other.Amount > m.Amount {
		return Money{}, errors.New("cannot subtract: would result in negative amount")
	}
	return Money{Amount: m.Amount - other.Amount, Currency: m.Currency}, nil
}

// Multiply returns a new Money scaled by the given factor.
func (m Money) Multiply(factor int) (Money, error) {
	if factor < 0 {
		return Money{}, errors.New("factor cannot be negative")
	}
	return Money{Amount: m.Amount * factor, Currency: m.Currency}, nil
}

func (m Money) assertSameCurrency(other Money) error {
	if m.Currency != other.Currency {
		return fmt.Errorf("currency mismatch: %s vs %s", m.Currency, other.Currency)
	}
	return nil
}

// Equal reports whether two Money values are identical.
func (m Money) Equal(other Money) bool {
	return m.Amount == other.Amount && m.Currency == other.Currency
}

// String returns a human-readable representation.
func (m Money) String() string {
	return fmt.Sprintf("%s %d.%02d", m.Currency, m.Amount/100, m.Amount%100)
}
```

## Common Value Objects

### ID Value Object

```go
// TaskID is a strongly-typed identifier for tasks.
type TaskID struct {
	value string
}

// NewTaskID generates a new random TaskID.
func NewTaskID() TaskID {
	return TaskID{value: uuid.NewString()}
}

// ParseTaskID creates a TaskID from an existing string.
func ParseTaskID(value string) (TaskID, error) {
	if strings.TrimSpace(value) == "" {
		return TaskID{}, errors.New("task ID cannot be empty")
	}
	return TaskID{value: value}, nil
}

// String returns the ID string.
func (id TaskID) String() string { return id.value }

// Equal reports whether two TaskIDs are identical.
func (id TaskID) Equal(other TaskID) bool {
	return id.value == other.value
}
```

### Date Range Value Object

```go
// DateRange represents a start-end time interval.
type DateRange struct {
	Start time.Time
	End   time.Time
}

// NewDateRange creates a DateRange, validating that end is after start.
func NewDateRange(start, end time.Time) (DateRange, error) {
	if end.Before(start) {
		return DateRange{}, errors.New("end date must be after start date")
	}
	return DateRange{Start: start, End: end}, nil
}

// Contains reports whether the given time falls within the range.
func (r DateRange) Contains(t time.Time) bool {
	return !t.Before(r.Start) && !t.After(r.End)
}

// Overlaps reports whether two ranges share any time.
func (r DateRange) Overlaps(other DateRange) bool {
	return !r.Start.After(other.End) && !r.End.Before(other.Start)
}

// DurationDays returns the number of days in the range.
func (r DateRange) DurationDays() int {
	return int(r.End.Sub(r.Start).Hours() / 24)
}
```

### Address Value Object

```go
// Address represents a postal address.
type Address struct {
	Street     string
	City       string
	PostalCode string
	Country    string
}

// NewAddress creates an Address, validating all fields.
func NewAddress(street, city, postalCode, country string) (Address, error) {
	street = strings.TrimSpace(street)
	city = strings.TrimSpace(city)
	postalCode = strings.ToUpper(strings.TrimSpace(postalCode))
	country = strings.TrimSpace(country)

	if street == "" {
		return Address{}, errors.New("street is required")
	}
	if city == "" {
		return Address{}, errors.New("city is required")
	}
	if postalCode == "" {
		return Address{}, errors.New("postal code is required")
	}
	if country == "" {
		return Address{}, errors.New("country is required")
	}

	return Address{
		Street:     street,
		City:       city,
		PostalCode: postalCode,
		Country:    country,
	}, nil
}

// Equal reports whether two Address values are identical.
func (a Address) Equal(other Address) bool {
	return a.Street == other.Street &&
		a.City == other.City &&
		a.PostalCode == other.PostalCode &&
		a.Country == other.Country
}

// String returns a formatted address string.
func (a Address) String() string {
	return fmt.Sprintf("%s, %s, %s, %s", a.Street, a.City, a.PostalCode, a.Country)
}
```

## Equality and Comparison

Always implement `Equal()`:

```go
// By value comparison
func (m Money) Equal(other Money) bool {
	return m.Amount == other.Amount && m.Currency == other.Currency
}

// In collections
prices := []Money{must(NewMoney(1000, "USD")), must(NewMoney(2000, "USD"))}
target := must(NewMoney(1000, "USD"))
for _, p := range prices {
	if p.Equal(target) {
		// found
	}
}
```

## Testing Value Objects

```go
func TestNewMoney(t *testing.T) {
	tests := []struct {
		name     string
		amount   int
		currency string
		wantErr  string
	}{
		{
			name:     "creates money with valid amount and currency",
			amount:   10000,
			currency: "USD",
		},
		{
			name:     "rejects negative amounts",
			amount:   -1000,
			currency: "USD",
			wantErr:  "cannot be negative",
		},
		{
			name:     "rejects invalid currency codes",
			amount:   1000,
			currency: "US",
			wantErr:  "3-letter ISO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := NewMoney(tt.amount, tt.currency)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if money.Amount != tt.amount {
				t.Errorf("want amount %d, got %d", tt.amount, money.Amount)
			}
			if money.Currency != tt.currency {
				t.Errorf("want currency %q, got %q", tt.currency, money.Currency)
			}
		})
	}
}

func TestMoney_Add(t *testing.T) {
	t.Run("adds same currency", func(t *testing.T) {
		a, _ := NewMoney(1000, "USD")
		b, _ := NewMoney(2000, "USD")
		result, err := a.Add(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Amount != 3000 {
			t.Errorf("want 3000, got %d", result.Amount)
		}
	})

	t.Run("rejects adding different currencies", func(t *testing.T) {
		usd, _ := NewMoney(1000, "USD")
		eur, _ := NewMoney(1000, "EUR")
		_, err := usd.Add(eur)
		if err == nil || !strings.Contains(err.Error(), "currency mismatch") {
			t.Fatalf("want currency mismatch error, got %v", err)
		}
	})
}

func TestMoney_Equal(t *testing.T) {
	t.Run("equals same value", func(t *testing.T) {
		a, _ := NewMoney(1000, "USD")
		b, _ := NewMoney(1000, "USD")
		if !a.Equal(b) {
			t.Error("want equal, got not equal")
		}
	})

	t.Run("not equals different amount", func(t *testing.T) {
		a, _ := NewMoney(1000, "USD")
		b, _ := NewMoney(2000, "USD")
		if a.Equal(b) {
			t.Error("want not equal, got equal")
		}
	})
}
```
