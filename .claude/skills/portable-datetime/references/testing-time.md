# Deterministic Time Testing

Write tests that produce identical results regardless of when or where they run.

## Core Principle: Inject Time as Dependency

Never call `time.Now()` inside domain logic. Always accept time as a parameter:

```go
// BAD - non-deterministic
func createSession() Session {
	return Session{
		ID:        uuid.NewString(),
		StartedAt: time.Now(), // Different every time!
		ExpiresAt: time.Now().Add(time.Hour),
	}
}

// GOOD - deterministic
func createSession(id string, now time.Time) Session {
	utcNow := now.UTC()
	return Session{
		ID:        id,
		StartedAt: utcNow,
		ExpiresAt: utcNow.Add(time.Hour),
	}
}

// In tests
func TestCreateSession(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC)
	session := createSession("session-1", fixedTime)

	if !session.StartedAt.Equal(fixedTime) {
		t.Errorf("want %v, got %v", fixedTime, session.StartedAt)
	}
	wantExpires := time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC)
	if !session.ExpiresAt.Equal(wantExpires) {
		t.Errorf("want %v, got %v", wantExpires, session.ExpiresAt)
	}
}
```

## Fixed Test Timestamps

Create a set of well-documented test timestamps:

```go
// internal/testutil/times.go

// Fixed UTC timestamps for deterministic testing.
// All times are carefully chosen to represent specific scenarios.
var (
	// Wednesday Jan 15, 2025 at 5pm UTC = 9am PT (PST, UTC-8)
	Wednesday9amPT = time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC)

	// Wednesday Jan 15, 2025 at 3pm UTC = 7am PT
	Wednesday7amPT = time.Date(2025, 1, 15, 15, 0, 0, 0, time.UTC)

	// Thursday Jan 16, 2025 at 4pm UTC = 8am PT
	Thursday8amPT = time.Date(2025, 1, 16, 16, 0, 0, 0, time.UTC)

	// Saturday Jan 18, 2025 at 4pm UTC = 8am PT
	Saturday8amPT = time.Date(2025, 1, 18, 16, 0, 0, 0, time.UTC)

	// Monday Jan 20, 2025 at 4pm UTC = 8am PT
	Monday8amPT = time.Date(2025, 1, 20, 16, 0, 0, 0, time.UTC)

	// Summer time (PDT, UTC-7)
	// Wednesday Jul 16, 2025 at 3pm UTC = 8am PT
	SummerWednesday8amPT = time.Date(2025, 7, 16, 15, 0, 0, 0, time.UTC)
)
```

## Testing Time-Based Logic

### Testing "Tomorrow at 8am PT"

```go
func TestComputeResurfaceAt_Tomorrow(t *testing.T) {
	// Given: Wednesday at 9am PT
	now := Wednesday9amPT

	// When: deferring to tomorrow
	result := computeResurfaceAt(DeferTomorrow, now)

	// Then: Thursday at 8am PT (4pm UTC)
	if !result.Equal(Thursday8amPT) {
		t.Errorf("want %v, got %v", Thursday8amPT, result)
	}
}
```

### Testing Day-of-Week Logic

```go
func TestComputeResurfaceAt_ThisWeekend(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "Wednesday to Saturday",
			now:  Wednesday9amPT,
			want: Saturday8amPT,
		},
		{
			name: "Saturday to next Saturday",
			now:  Saturday8amPT,
			want: time.Date(2025, 1, 25, 16, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeResurfaceAt(DeferThisWeekend, tt.now)
			if !result.Equal(tt.want) {
				t.Errorf("want %v, got %v", tt.want, result)
			}
		})
	}
}
```

### Testing DST Transitions

```go
func TestComputeResurfaceAt_DST(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "winter PST (UTC-8)",
			now:  time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC), // 9am PT
			// 9am + 2h = 11am PT = 7pm UTC (UTC-8)
			want: time.Date(2025, 1, 15, 19, 0, 0, 0, time.UTC),
		},
		{
			name: "summer PDT (UTC-7)",
			now:  time.Date(2025, 7, 15, 16, 0, 0, 0, time.UTC), // 9am PT
			// 9am + 2h = 11am PT = 6pm UTC (UTC-7)
			want: time.Date(2025, 7, 15, 18, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeResurfaceAt(DeferLaterToday, tt.now)
			if !result.Equal(tt.want) {
				t.Errorf("want %v, got %v", tt.want, result)
			}
		})
	}
}
```

## Testing Display Formatting

```go
func TestFormatForDisplay(t *testing.T) {
	tz := "America/Los_Angeles"

	tests := []struct {
		name string
		utc  time.Time
		want string
	}{
		{
			name: "formats UTC as PT display time",
			utc:  time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC), // 8am PT
			want: "Jan 15, 8:00 AM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatForDisplay(tt.utc, tz)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestFormatTimeAmPm(t *testing.T) {
	tz := "America/Los_Angeles"

	tests := []struct {
		name string
		utc  time.Time
		want string
	}{
		{
			name: "noon",
			utc:  time.Date(2025, 1, 15, 20, 0, 0, 0, time.UTC), // 12pm PT
			want: "12pm",
		},
		{
			name: "afternoon",
			utc:  time.Date(2025, 1, 15, 21, 0, 0, 0, time.UTC), // 1pm PT
			want: "1pm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatTimeAmPm(tt.utc, tz)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}
}
```

## Testing Relative Time

```go
func TestFormatRelative(t *testing.T) {
	tz := "America/Los_Angeles"

	tests := []struct {
		name   string
		target time.Time
		now    time.Time
		want   string
	}{
		{
			name:   "same day in PT",
			target: time.Date(2025, 1, 15, 20, 0, 0, 0, time.UTC), // 12pm PT
			now:    time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC), // 9am PT
			want:   "Today",
		},
		{
			name:   "next day in PT",
			target: time.Date(2025, 1, 16, 16, 0, 0, 0, time.UTC), // Jan 16 8am PT
			now:    time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC), // Jan 15 9am PT
			want:   "Tomorrow",
		},
		{
			name:   "day boundary in PT",
			target: time.Date(2025, 1, 16, 16, 0, 0, 0, time.UTC), // Jan 16, 8am PT
			now:    time.Date(2025, 1, 16, 6, 0, 0, 0, time.UTC),  // Jan 15, 10pm PT
			want:   "Tomorrow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatRelative(tt.target, tt.now, tz)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}
}
```

## Using a Clock Interface (When Necessary)

For code that internally needs the current time:

```go
// Clock provides the current time.
type Clock interface {
	Now() time.Time
}

// RealClock returns the actual current time.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

// FixedClock returns a fixed time (for testing).
type FixedClock struct {
	Time time.Time
}

func (c FixedClock) Now() time.Time { return c.Time }

// Usage in service
type SessionService struct {
	clock Clock
}

func (s *SessionService) IsExpired(session *Session) bool {
	return !session.ExpiresAt.After(s.clock.Now())
}

// In tests
func TestSessionService_IsExpired(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC)
	service := &SessionService{clock: FixedClock{Time: fixedTime}}

	session := &Session{
		ExpiresAt: time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC), // 1 hour ago
	}

	if !service.IsExpired(session) {
		t.Error("expected session to be expired")
	}
}
```

**Note:** Prefer injecting `time.Time` directly over a Clock interface when possible. Clock interfaces are for cases where the function is called repeatedly and needs a fresh time each call.

## Test Data Builders with Time

```go
// internal/testutil/builders.go

type ActionBuilder struct {
	action NextAction
}

func NewActionBuilder() *ActionBuilder {
	return &ActionBuilder{
		action: NextAction{
			ID:        "action-1",
			Text:      "Test action",
			CreatedAt: Wednesday9amPT,
			UpdatedAt: Wednesday9amPT,
		},
	}
}

func (b *ActionBuilder) WithID(id string) *ActionBuilder {
	b.action.ID = id
	return b
}

func (b *ActionBuilder) ResurfacingAt(t time.Time) *ActionBuilder {
	b.action.ResurfaceAt = &t
	return b
}

func (b *ActionBuilder) ResurfacingTomorrow() *ActionBuilder {
	t := Thursday8amPT
	b.action.ResurfaceAt = &t
	return b
}

func (b *ActionBuilder) Completed() *ActionBuilder {
	t := b.action.UpdatedAt
	b.action.DoneAt = &t
	return b
}

func (b *ActionBuilder) Build() NextAction {
	return b.action
}

// Usage
action := NewActionBuilder().WithID("action-123").ResurfacingTomorrow().Build()
```

## Common Test Scenarios

### Timezone Edge Cases

```go
func TestTimezoneEdgeCases(t *testing.T) {
	loc, _ := time.LoadLocation("America/Los_Angeles")

	t.Run("late PT is next day in UTC", func(t *testing.T) {
		// 11pm PT on Jan 15 = 7am UTC on Jan 16
		utc := time.Date(2025, 1, 16, 7, 0, 0, 0, time.UTC)
		lt := utc.In(loc)
		if lt.Day() != 15 {
			t.Errorf("want day 15, got %d", lt.Day())
		}
		if lt.Hour() != 23 {
			t.Errorf("want hour 23, got %d", lt.Hour())
		}
	})

	t.Run("early PT spans midnight UTC", func(t *testing.T) {
		// 1am PT on Jan 16 = 9am UTC on Jan 16
		utc := time.Date(2025, 1, 16, 9, 0, 0, 0, time.UTC)
		lt := utc.In(loc)
		if lt.Day() != 16 {
			t.Errorf("want day 16, got %d", lt.Day())
		}
		if lt.Hour() != 1 {
			t.Errorf("want hour 1, got %d", lt.Hour())
		}
	})
}
```

### Leap Year and Month Boundaries

```go
func TestDateBoundaries(t *testing.T) {
	t.Run("leap year February", func(t *testing.T) {
		feb28 := time.Date(2024, 2, 28, 16, 0, 0, 0, time.UTC)
		tomorrow := feb28.AddDate(0, 0, 1)
		if tomorrow.Day() != 29 {
			t.Errorf("want day 29, got %d", tomorrow.Day())
		}
		if tomorrow.Month() != time.February {
			t.Errorf("want February, got %v", tomorrow.Month())
		}
	})

	t.Run("month boundary", func(t *testing.T) {
		jan31 := time.Date(2025, 1, 31, 16, 0, 0, 0, time.UTC)
		nextDay := jan31.AddDate(0, 0, 1)
		if nextDay.Day() != 1 {
			t.Errorf("want day 1, got %d", nextDay.Day())
		}
		if nextDay.Month() != time.February {
			t.Errorf("want February, got %v", nextDay.Month())
		}
	})
}
```

## Anti-Patterns

### Don't rely on system timezone in tests

```go
// BAD - depends on test runner's timezone
func TestShowsCorrectHour(t *testing.T) {
	d := time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC)
	if d.Hour() != 8 { // Only passes in PT!
		t.Error("wrong hour")
	}
}

// GOOD - explicit timezone
func TestShowsCorrectHourInPT(t *testing.T) {
	d := time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC)
	loc, _ := time.LoadLocation("America/Los_Angeles")
	if d.In(loc).Hour() != 8 {
		t.Error("wrong hour")
	}
}
```

### Don't use time.Now() in tests

```go
// BAD - different results on different days
func TestFormatsAsToday(t *testing.T) {
	now := time.Now()
	target := now.Add(2 * time.Hour)
	// ...
}

// GOOD - fixed dates
func TestFormatsAsToday(t *testing.T) {
	now := time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC)
	target := time.Date(2025, 1, 15, 19, 0, 0, 0, time.UTC)
	// ...
}
```

## Summary

- Inject time as a parameter, never call `time.Now()` in domain logic
- Create well-documented fixed test timestamps with `time.Date`
- Test both winter (PST) and summer (PDT) scenarios
- Use explicit timezone in assertions via `time.LoadLocation`
- Test edge cases: midnight boundaries, DST transitions, leap years
- Prefer time injection over Clock interfaces when possible
- Never rely on the test runner's system timezone
