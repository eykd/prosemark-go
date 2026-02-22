# UTC Storage Patterns

Store all timestamps as UTC `time.Time` values or RFC 3339 strings for portability and consistency.

## RFC 3339 UTC Format

```
2025-01-15T16:00:00Z
|         |         |
|         |         +- Z = Zulu time (UTC)
|         +- Time: 16:00:00
+- Date: 2025-01-15
```

**Why RFC 3339 UTC:**

- Unambiguous - no timezone confusion
- Sortable - string comparison works correctly
- Portable - works across all databases, languages, and systems
- Parseable - `time.Parse(time.RFC3339, s)` works everywhere

## Entity Timestamps

```go
// NextAction represents an item with UTC timestamps.
type NextAction struct {
	ID          string
	Text        string
	ResurfaceAt *time.Time // UTC, nil = no scheduled resurface
	CreatedAt   time.Time  // UTC
	UpdatedAt   time.Time  // UTC
	DoneAt      *time.Time // UTC, nil = not completed
}

// Session tracks a time-bounded activity.
type Session struct {
	ID        string
	StartedAt time.Time  // UTC
	ExpiresAt time.Time  // UTC
	ClosedAt  *time.Time // UTC, nil = still open
}
```

## Creating Timestamps

### Current Time

```go
// Simple - get current UTC time
now := time.Now().UTC()

// In a function that accepts injected time
func createAction(text string, now time.Time) NextAction {
	utcNow := now.UTC()
	return NextAction{
		ID:        uuid.NewString(),
		Text:      text,
		CreatedAt: utcNow,
		UpdatedAt: utcNow,
	}
}
```

### Future Timestamps

```go
// Add hours
func addHours(t time.Time, hours int) time.Time {
	return t.Add(time.Duration(hours) * time.Hour)
}

// Add days
func addDays(t time.Time, days int) time.Time {
	return t.AddDate(0, 0, days)
}

// Example: Session expires in 1 hour
session := Session{
	StartedAt: now,
	ExpiresAt: now.Add(time.Hour),
}
```

### Specific Time in Display Timezone

When you need a specific time in a display timezone (e.g., "8am PT tomorrow"):

```go
func createTimestampAt(base time.Time, hour, daysFromNow int, tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, fmt.Errorf("loading location %s: %w", tz, err)
	}

	// Get the target date in the display timezone
	target := base.In(loc).AddDate(0, 0, daysFromNow)

	// Create the time in that timezone, then convert to UTC
	result := time.Date(target.Year(), target.Month(), target.Day(),
		hour, 0, 0, 0, loc)
	return result.UTC(), nil
}

// "Tomorrow at 8am PT" in UTC
tomorrowMorning, _ := createTimestampAt(time.Now(), 8, 1, "America/Los_Angeles")
```

## Database Storage

### SQL Schema

Store as `TIMESTAMPTZ` (PostgreSQL) or `TEXT` (SQLite):

```sql
CREATE TABLE next_actions (
  id TEXT PRIMARY KEY,
  text TEXT NOT NULL,
  resurface_at TIMESTAMPTZ,        -- UTC timestamp
  created_at TIMESTAMPTZ NOT NULL,  -- UTC timestamp
  updated_at TIMESTAMPTZ NOT NULL,  -- UTC timestamp
  done_at TIMESTAMPTZ               -- UTC timestamp or NULL
);

-- Query by time range (works directly with UTC)
SELECT * FROM next_actions
WHERE resurface_at >= '2025-01-15T00:00:00Z'
  AND resurface_at < '2025-01-16T00:00:00Z';

-- Order by time
SELECT * FROM next_actions
WHERE resurface_at IS NOT NULL
ORDER BY resurface_at ASC;
```

### Repository Pattern

```go
// NextActionRepository defines persistence for next actions.
type NextActionRepository interface {
	Save(ctx context.Context, action *NextAction) error
	FindByID(ctx context.Context, id string) (*NextAction, error)
	FindResurfacingBefore(ctx context.Context, before time.Time) ([]*NextAction, error)
}

// SQLNextActionRepository implements NextActionRepository with database/sql.
type SQLNextActionRepository struct {
	db *sql.DB
}

func (r *SQLNextActionRepository) FindResurfacingBefore(ctx context.Context, before time.Time) ([]*NextAction, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, text, resurface_at, created_at, updated_at, done_at FROM next_actions WHERE resurface_at <= $1 AND done_at IS NULL",
		before.UTC())
	if err != nil {
		return nil, fmt.Errorf("querying resurfacing actions: %w", err)
	}
	defer rows.Close()

	var actions []*NextAction
	for rows.Next() {
		a, err := r.scanAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}
```

## Comparing Timestamps

### Direct Comparison

`time.Time` supports direct comparison:

```go
earlier := time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC)
later := time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC)

earlier.Before(later)  // true
later.After(earlier)   // true
earlier.Equal(earlier) // true
```

### String Comparison

RFC 3339 UTC strings also sort lexicographically:

```go
s1 := "2025-01-15T08:00:00Z"
s2 := "2025-01-15T16:00:00Z"
s1 < s2 // true
```

### Duration Between

```go
func isExpired(expiresAt, now time.Time) bool {
	return !expiresAt.After(now)
}

func minutesBetween(start, end time.Time) int {
	return int(end.Sub(start).Minutes())
}
```

### Same Day Check

Check if two UTC timestamps fall on the same day in a specific timezone:

```go
func isSameDayInTimezone(t1, t2 time.Time, tz string) (bool, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return false, err
	}
	lt1 := t1.In(loc)
	lt2 := t2.In(loc)
	return lt1.Year() == lt2.Year() &&
		lt1.Month() == lt2.Month() &&
		lt1.Day() == lt2.Day(), nil
}
```

## Nil Handling

Use `*time.Time` for optional timestamps:

```go
type NextAction struct {
	ResurfaceAt *time.Time // nil = no scheduled resurface
	DoneAt      *time.Time // nil = not completed
}

// Creating with no resurface
action := NextAction{
	ID:        "act-123",
	Text:      "Review PR",
	CreatedAt: now,
	UpdatedAt: now,
}

// Completing action
func markComplete(action *NextAction, now time.Time) {
	utcNow := now.UTC()
	action.DoneAt = &utcNow
	action.UpdatedAt = utcNow
}
```

## Validation

Validate RFC 3339 format:

```go
func parseUTC(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid RFC 3339 timestamp: %w", err)
	}
	return t.UTC(), nil
}

// Usage
t, err := parseUTC(input.ResurfaceAt)
if err != nil {
	return fmt.Errorf("resurfaceAt must be a valid RFC 3339 UTC string: %w", err)
}
```

## Anti-Patterns

### Don't store timezone offsets

```go
// BAD - timezone offset can become stale (DST changes)
stored := "2025-01-15T08:00:00-08:00"

// GOOD - always UTC
stored := "2025-01-15T16:00:00Z"
```

### Don't store local time strings

```go
// BAD - ambiguous, not portable
stored := "2025-01-15 08:00:00"

// GOOD - unambiguous UTC
stored := "2025-01-15T16:00:00Z"
```

### Don't store Unix timestamps as primary format

```go
// BAD - less readable, harder to debug
stored := int64(1736956800)

// GOOD - human readable, still sortable
stored := "2025-01-15T16:00:00Z"
```

## Summary

- Store all timestamps as `time.Time` in UTC or RFC 3339 strings ending in `Z`
- Use `time.Now().UTC()` to get current time
- Use `time.AddDate` / `time.Add` for date math
- Direct comparison with `Before`/`After`/`Equal` works correctly
- Use `*time.Time` for optional timestamps
- Validate input with `time.Parse(time.RFC3339, s)`
