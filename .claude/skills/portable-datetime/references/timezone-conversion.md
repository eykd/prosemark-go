# Timezone Conversion Patterns

Convert between UTC and display timezones at system boundaries only.

## The time.LoadLocation Approach

Use `time.LoadLocation` for all timezone conversions - it handles DST automatically:

```go
// Format for display
func formatForDisplay(utc time.Time, tz string) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", fmt.Errorf("loading location %s: %w", tz, err)
	}
	return utc.In(loc).Format("Jan 2, 3:04 PM"), nil
}

// "Jan 15, 8:00 AM"
```

## Getting Components in a Timezone

Extract year, month, day, hour from a UTC time in a specific timezone:

```go
// TimeComponents holds date/time parts in a specific timezone.
type TimeComponents struct {
	Year      int
	Month     time.Month
	Day       int
	Hour      int
	Minute    int
	DayOfWeek time.Weekday
}

func getComponentsInTimezone(t time.Time, tz string) (TimeComponents, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return TimeComponents{}, fmt.Errorf("loading location %s: %w", tz, err)
	}
	lt := t.In(loc)
	return TimeComponents{
		Year:      lt.Year(),
		Month:     lt.Month(),
		Day:       lt.Day(),
		Hour:      lt.Hour(),
		Minute:    lt.Minute(),
		DayOfWeek: lt.Weekday(),
	}, nil
}

// Usage
utc := time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC)
pt, _ := getComponentsInTimezone(utc, "America/Los_Angeles")
// Year: 2025, Month: January, Day: 15, Hour: 8, Minute: 0, DayOfWeek: Wednesday
// (Wednesday at 8am PT)
```

## Creating UTC from Timezone Components

Convert a time in a specific timezone to UTC:

```go
func createUTCFromTimezone(year int, month time.Month, day, hour, minute int, tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, fmt.Errorf("loading location %s: %w", tz, err)
	}
	// time.Date in a specific location automatically handles DST
	local := time.Date(year, month, day, hour, minute, 0, 0, loc)
	return local.UTC(), nil
}

// "8am PT on Jan 15, 2025" as UTC
utc, _ := createUTCFromTimezone(2025, time.January, 15, 8, 0, "America/Los_Angeles")
// 2025-01-15T16:00:00Z (PST = UTC-8)
```

## Display Formatting Recipes

### Relative Time Display

```go
func formatRelative(target, now time.Time, tz string) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", err
	}

	targetLocal := target.In(loc)
	nowLocal := now.In(loc)

	// Same day?
	if sameDay(targetLocal, nowLocal) {
		return "Today", nil
	}

	// Tomorrow?
	tomorrowLocal := nowLocal.AddDate(0, 0, 1)
	if sameDay(targetLocal, tomorrowLocal) {
		return "Tomorrow", nil
	}

	// Otherwise show date
	return targetLocal.Format("Jan 2"), nil
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}
```

### Time with AM/PM

```go
func formatTimeAmPm(utc time.Time, tz string) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", err
	}
	lt := utc.In(loc)

	hour := lt.Hour() % 12
	if hour == 0 {
		hour = 12
	}
	ampm := "am"
	if lt.Hour() >= 12 {
		ampm = "pm"
	}

	if lt.Minute() > 0 {
		return fmt.Sprintf("%d:%02d%s", hour, lt.Minute(), ampm), nil
	}
	return fmt.Sprintf("%d%s", hour, ampm), nil
}

// "8am", "1:30pm", "12pm"
```

### Full Date and Time

```go
func formatFullDateTime(utc time.Time, tz string) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", err
	}
	return utc.In(loc).Format("Mon, Jan 2, 3:04 PM"), nil
}

// "Wed, Jan 15, 8:00 AM"
```

### Day of Week

```go
func getDayOfWeekName(utc time.Time, tz string) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", err
	}
	return utc.In(loc).Weekday().String(), nil
}

// "Wednesday"
```

## DST Handling

`time.LoadLocation` handles DST automatically. Be aware of edge cases:

### DST Transitions

```go
// Winter (PST = UTC-8)
winter := time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC) // 8am PT
loc, _ := time.LoadLocation("America/Los_Angeles")
winterPT := winter.In(loc)
// Hour: 8

// Summer (PDT = UTC-7)
summer := time.Date(2025, 7, 15, 15, 0, 0, 0, time.UTC) // 8am PT
summerPT := summer.In(loc)
// Hour: 8

// Note: Same local hour, different UTC hours!
```

### Creating Times Near DST Boundary

When scheduling events, Go's `time.Date` with a `*Location` handles DST automatically:

```go
// Spring forward: 2am PT doesn't exist on March 9, 2025
// Go's time.Date adjusts automatically
loc, _ := time.LoadLocation("America/Los_Angeles")
t := time.Date(2025, 3, 9, 2, 0, 0, 0, loc)
// Adjusted to 3:00 AM PDT

// Fall back: 1am PT happens twice on November 2, 2025
// Go picks the first occurrence
```

## Presentation Layer Integration

### CLI Output Example

```go
// internal/infra/cli_formatter.go
const userTimezone = "America/Los_Angeles"

func formatActionMeta(action *NextAction) string {
	if action.ResurfaceAt == nil {
		return ""
	}

	relative, _ := formatRelative(*action.ResurfaceAt, time.Now().UTC(), userTimezone)
	timeStr, _ := formatTimeAmPm(*action.ResurfaceAt, userTimezone)

	return fmt.Sprintf("Resurfaces: %s at %s", relative, timeStr)
}
```

### API Response Example

```go
// internal/infra/api_formatter.go
type SessionResponse struct {
	ID               string `json:"id"`
	StartedAt        string `json:"startedAt"`        // UTC for API consumers
	StartedAtDisplay string `json:"startedAtDisplay"`  // Human-readable
	ExpiresAt        string `json:"expiresAt"`
	ExpiresAtDisplay string `json:"expiresAtDisplay"`
}

func formatSessionResponse(s *Session, userTZ string) SessionResponse {
	startDisplay, _ := formatFullDateTime(s.StartedAt, userTZ)
	expiresDisplay, _ := formatFullDateTime(s.ExpiresAt, userTZ)
	return SessionResponse{
		ID:               s.ID,
		StartedAt:        s.StartedAt.Format(time.RFC3339),
		StartedAtDisplay: startDisplay,
		ExpiresAt:        s.ExpiresAt.Format(time.RFC3339),
		ExpiresAtDisplay: expiresDisplay,
	}
}
```

## Common Timezone Identifiers

```go
// US Timezones
"America/Los_Angeles"  // Pacific (PT)
"America/Denver"       // Mountain (MT)
"America/Chicago"      // Central (CT)
"America/New_York"     // Eastern (ET)

// Other common
"Europe/London"        // UK (GMT/BST)
"Europe/Paris"         // Central Europe (CET/CEST)
"Asia/Tokyo"           // Japan (JST)
"Australia/Sydney"     // Australia Eastern (AEST/AEDT)

// UTC
"UTC"                  // Coordinated Universal Time
```

## Anti-Patterns

### Don't hardcode offsets

```go
// BAD - hardcoded offset ignores DST
func ptToUTC(ptHour int) int {
	return ptHour + 8 // Only correct in winter!
}

// GOOD - use time.LoadLocation for correct offset
loc, _ := time.LoadLocation("America/Los_Angeles")
utc := time.Date(2025, 1, 15, 8, 0, 0, 0, loc).UTC()
```

### Don't use local time methods in domain

```go
// BAD - uses system timezone
hour := t.Hour()    // Depends on server location!
day := t.Weekday()

// GOOD - explicit timezone
loc, _ := time.LoadLocation("America/Los_Angeles")
hour := t.In(loc).Hour()
day := t.In(loc).Weekday()
```

### Don't convert in domain logic

```go
// BAD - timezone logic in domain
func (a *NextAction) DisplayTime() string {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	return a.resurfaceAt.In(loc).Format("3:04 PM") // Wrong place!
}

// GOOD - convert in adapter only
// Domain returns UTC
utcTime := action.ResurfaceAt()
// Adapter converts for display
display := formatForDisplay(utcTime, userTimezone)
```

## Summary

- Use `time.LoadLocation` + `time.In(loc)` for all timezone conversions
- Extract components with `In(loc)` and standard `time.Time` methods
- Create UTC from timezone components using `time.Date(y, m, d, h, min, 0, 0, loc).UTC()`
- Convert only at system boundaries (adapters/presenters)
- DST is handled automatically by Go's `time` package
- Never hardcode timezone offsets
