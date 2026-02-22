---
name: portable-datetime
description: 'Write portable datetime handling code that stores in UTC, calculates in UTC, and displays in a configurable timezone. Use when: (1) Working with dates/times that must work across timezones, (2) Storing timestamps in databases, (3) Displaying times to users in their locale, (4) Scheduling future events, (5) Writing timezone-independent tests, or (6) Converting between UTC and local time.'
---

# Portable Datetime Handling

Handle datetimes correctly across timezones using the UTC Sandwich pattern: convert to UTC at input boundaries, work exclusively in UTC within your domain, and convert to display timezones only at output boundaries.

## The UTC Sandwich Pattern

```
+---------------------------------------------------------+
|                    System Boundary                       |
|  +-------------+                      +-------------+   |
|  |   INPUT     |    +-----------+     |   OUTPUT    |   |
|  |  (any tz)   |--->|  DOMAIN   |---->|  (display   |   |
|  |             |    |  (UTC)    |     |   tz)       |   |
|  |  Convert    |    |           |     |  Convert    |   |
|  |  to UTC     |    |  Store    |     |  from UTC   |   |
|  +-------------+    |  Calc     |     +-------------+   |
|                     |  Compare  |                       |
|                     +-----------+                       |
+---------------------------------------------------------+
```

**Why this matters:** Tests run identically anywhere, database queries work without timezone math, comparisons are simple, DST transitions don't corrupt logic, and scheduling is unambiguous.

## Quick Reference

| What you need                      | Where to look                                                                           |
| ---------------------------------- | --------------------------------------------------------------------------------------- |
| Store timestamps in database       | [utc-storage.md](references/utc-storage.md) - RFC 3339 patterns                        |
| Display times to users             | [timezone-conversion.md](references/timezone-conversion.md) - `time.LoadLocation`      |
| Add/subtract time, schedule events | [common-operations.md](references/common-operations.md) - Calculation recipes           |
| Write deterministic tests          | [testing-time.md](references/testing-time.md) - Time injection patterns                 |

## Decision Framework

```go
// STORING? -> RFC 3339 UTC strings or time.Time in UTC
createdAt := now.UTC().Format(time.RFC3339) // "2025-01-15T16:00:00Z"

// CALCULATING? -> time.Time methods (always in UTC)
tomorrow := now.UTC().AddDate(0, 0, 1)

// DISPLAYING? -> Convert at boundary only
loc, _ := time.LoadLocation("America/Los_Angeles")
display := utcTime.In(loc).Format("Jan 2, 3:04 PM")
```

## Essential Patterns

**Store as UTC** - See [utc-storage.md](references/utc-storage.md)

```go
startedAt := now.UTC().Format(time.RFC3339) // "2025-01-15T16:00:00Z"
```

**Calculate in UTC** - See [common-operations.md](references/common-operations.md)

```go
tomorrow := now.UTC().AddDate(0, 0, 1)
morning := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 8, 0, 0, 0, time.UTC)
```

**Display at boundaries** - See [timezone-conversion.md](references/timezone-conversion.md)

```go
loc, _ := time.LoadLocation("America/Los_Angeles")
display := utcTime.In(loc).Format("Jan 2, 3:04 PM")
```

**Inject time for testing** - See [testing-time.md](references/testing-time.md)

```go
func computeResurfaceAt(option DeferOption, now time.Time) time.Time {
	tomorrow := now.UTC().AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(),
		8, 0, 0, 0, time.UTC)
}

func TestComputeResurfaceAt(t *testing.T) {
	now := time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC)
	got := computeResurfaceAt(DeferTomorrow, now)
	want := time.Date(2025, 1, 16, 16, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("want %v, got %v", want, got)
	}
}
```

## Critical Anti-Patterns

```go
// BAD - time.Now() inside domain logic
func createSession() Session {
	return Session{StartedAt: time.Now()} // Non-deterministic!
}
// GOOD - Inject time
func createSession(now time.Time) Session {
	return Session{StartedAt: now.UTC()}
}

// BAD - Domain entity knows about display timezone
func (s *Session) DisplayTime() string {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	return s.startedAt.In(loc).Format("3:04 PM")
}
// GOOD - Return UTC, convert at boundary
func (s *Session) StartedAt() time.Time {
	return s.startedAt
}

// BAD - Parsing without explicit timezone
t, _ := time.Parse("2006-01-02 15:04:05", "2025-01-15 08:00:00") // Local TZ!
// GOOD - Explicit UTC
t, _ := time.Parse(time.RFC3339, "2025-01-15T08:00:00Z")
```

## Summary Rules

1. Store UTC: All persisted timestamps are RFC 3339 UTC strings or `time.Time` in UTC
2. Calculate UTC: Use `time.Time` methods in UTC for date math
3. Convert at boundaries: Only use `time.In(loc)` in adapters/presenters
4. Inject time: Pass `now time.Time` as parameter, never `time.Now()` in domain
5. Test with fixed UTC: Use `time.Date(...)` with `time.UTC`, assert UTC outputs

## Reference Documentation

- [utc-storage.md](references/utc-storage.md) - Database schemas, type definitions, parsing, validation
- [timezone-conversion.md](references/timezone-conversion.md) - Display formatting, component extraction, DST handling
- [testing-time.md](references/testing-time.md) - Test fixtures, time injection, edge cases
- [common-operations.md](references/common-operations.md) - Duration math, scheduling, business days, comparisons

## Related Skills

- `/prefactoring` - Design time-handling abstractions and boundaries
- `/go-tdd` - Write deterministic datetime tests
