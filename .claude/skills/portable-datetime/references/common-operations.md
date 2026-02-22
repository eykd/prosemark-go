# Common Datetime Operations

Recipes for everyday datetime tasks using UTC storage and Go's `time` package.

## Adding Time Durations

### Add Hours

```go
func addHours(t time.Time, hours int) time.Time {
	return t.Add(time.Duration(hours) * time.Hour)
}

// 2 hours from now
later := addHours(time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC), 2)
// 2025-01-15T18:00:00Z
```

### Add Days

```go
func addDays(t time.Time, days int) time.Time {
	return t.AddDate(0, 0, days)
}

// Tomorrow same time
tomorrow := addDays(time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC), 1)
// 2025-01-16T16:00:00Z
```

### Add Weeks

```go
func addWeeks(t time.Time, weeks int) time.Time {
	return t.AddDate(0, 0, weeks*7)
}

// Next week
nextWeek := addWeeks(time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC), 1)
// 2025-01-22T16:00:00Z
```

### Add Minutes

```go
func addMinutes(t time.Time, minutes int) time.Time {
	return t.Add(time.Duration(minutes) * time.Minute)
}

// 30 minutes from now
later := addMinutes(time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC), 30)
// 2025-01-15T16:30:00Z
```

## Finding Specific Days

### Next Occurrence of Day of Week

```go
func getNextDayOfWeek(from time.Time, targetDay time.Weekday, tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, err
	}

	local := from.In(loc)
	currentDay := local.Weekday()

	// Calculate days until target
	daysUntil := int(targetDay-currentDay+7) % 7
	if daysUntil == 0 {
		daysUntil = 7 // Always go to next occurrence
	}

	return from.AddDate(0, 0, daysUntil), nil
}

// Next Saturday from Wednesday Jan 15
nextSat, _ := getNextDayOfWeek(
	time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC),
	time.Saturday,
	"America/Los_Angeles",
)
// Points to Jan 18
```

### Next Monday

```go
func getNextMonday(from time.Time, tz string) (time.Time, error) {
	return getNextDayOfWeek(from, time.Monday, tz)
}
```

### Next Weekday (Mon-Fri)

```go
func getNextWeekday(from time.Time, tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, err
	}

	local := from.In(loc)
	dayOfWeek := local.Weekday()

	var daysToAdd int
	switch dayOfWeek {
	case time.Friday:
		daysToAdd = 3 // Friday -> Monday
	case time.Saturday:
		daysToAdd = 2 // Saturday -> Monday
	default:
		daysToAdd = 1 // Sun-Thu -> next day
	}

	return from.AddDate(0, 0, daysToAdd), nil
}
```

## Setting Specific Times

### Set Time in Display Timezone

```go
func setTimeInTimezone(base time.Time, hour, minute int, tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, err
	}

	local := base.In(loc)
	result := time.Date(local.Year(), local.Month(), local.Day(),
		hour, minute, 0, 0, loc)
	return result.UTC(), nil
}

// Set to 8am PT
morning, _ := setTimeInTimezone(
	time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC),
	8, 0, "America/Los_Angeles",
)
// 2025-01-15T16:00:00Z
```

### Tomorrow at Specific Time

```go
func tomorrowAt(from time.Time, hour, minute int, tz string) (time.Time, error) {
	tomorrow := from.AddDate(0, 0, 1)
	return setTimeInTimezone(tomorrow, hour, minute, tz)
}

// Tomorrow at 8am PT
tomorrowMorning, _ := tomorrowAt(
	time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC),
	8, 0, "America/Los_Angeles",
)
// 2025-01-16T16:00:00Z
```

### Start of Day in Timezone

```go
func startOfDayInTimezone(t time.Time, tz string) (time.Time, error) {
	return setTimeInTimezone(t, 0, 0, tz)
}

// Start of Jan 15 in PT
startOfDay, _ := startOfDayInTimezone(
	time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC),
	"America/Los_Angeles",
)
// 2025-01-15T08:00:00Z (midnight PT = 8am UTC)
```

### End of Day in Timezone

```go
func endOfDayInTimezone(t time.Time, tz string) (time.Time, error) {
	return setTimeInTimezone(t, 23, 59, tz)
}
```

## Comparing Dates

### Is Before/After

```go
// time.Time has built-in comparison methods
t1.Before(t2)      // true if t1 is before t2
t1.After(t2)       // true if t1 is after t2
t1.Equal(t2)       // true if t1 equals t2
!t1.After(t2)      // before or equal
```

### Is Same Day in Timezone

```go
func isSameDay(t1, t2 time.Time, tz string) (bool, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return false, err
	}
	l1 := t1.In(loc)
	l2 := t2.In(loc)
	return l1.Year() == l2.Year() &&
		l1.Month() == l2.Month() &&
		l1.Day() == l2.Day(), nil
}

// Both on Jan 15 in PT?
same, _ := isSameDay(
	time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC),
	time.Date(2025, 1, 15, 23, 0, 0, 0, time.UTC),
	"America/Los_Angeles",
)
// true (9am PT and 3pm PT are same day)
```

### Is Today / Is Tomorrow in Timezone

```go
func isToday(t, now time.Time, tz string) (bool, error) {
	return isSameDay(t, now, tz)
}

func isTomorrow(t, now time.Time, tz string) (bool, error) {
	tomorrow := now.AddDate(0, 0, 1)
	return isSameDay(t, tomorrow, tz)
}
```

### Is Weekend in Timezone

```go
func isWeekend(t time.Time, tz string) (bool, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return false, err
	}
	day := t.In(loc).Weekday()
	return day == time.Saturday || day == time.Sunday, nil
}
```

## Time Differences

### Minutes / Hours / Days Between

```go
func minutesBetween(start, end time.Time) int {
	return int(end.Sub(start).Minutes())
}

func hoursBetween(start, end time.Time) int {
	return int(end.Sub(start).Hours())
}

func daysBetween(start, end time.Time) int {
	return int(end.Sub(start).Hours() / 24)
}
```

### Human-Readable Duration

```go
func formatDuration(start, end time.Time) string {
	minutes := int(end.Sub(start).Minutes())

	if minutes < 60 {
		return fmt.Sprintf("%d min", minutes)
	}

	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%dh %dm", hours, minutes%60)
	}

	days := hours / 24
	return fmt.Sprintf("%dd %dh", days, hours%24)
}
```

## Working Hours

### Is Within Working Hours

```go
// WorkingHours defines business hours in a timezone.
type WorkingHours struct {
	StartHour int
	EndHour   int
	Timezone  string
	WorkDays  []time.Weekday
}

var DefaultWorkingHours = WorkingHours{
	StartHour: 9,
	EndHour:   17,
	Timezone:  "America/Los_Angeles",
	WorkDays:  []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
}

func isWithinWorkingHours(t time.Time, hours WorkingHours) (bool, error) {
	loc, err := time.LoadLocation(hours.Timezone)
	if err != nil {
		return false, err
	}
	lt := t.In(loc)

	// Check day
	dayMatch := false
	for _, d := range hours.WorkDays {
		if lt.Weekday() == d {
			dayMatch = true
			break
		}
	}
	if !dayMatch {
		return false, nil
	}

	// Check hour
	return lt.Hour() >= hours.StartHour && lt.Hour() < hours.EndHour, nil
}
```

### Next Working Hour

```go
func getNextWorkingHour(from time.Time, hours WorkingHours) (time.Time, error) {
	loc, err := time.LoadLocation(hours.Timezone)
	if err != nil {
		return time.Time{}, err
	}

	lt := from.In(loc)

	// If currently in working hours, return current time
	inHours, _ := isWithinWorkingHours(from, hours)
	if inHours {
		return from, nil
	}

	// Find next working day
	target := from
	for i := 0; i < 7; i++ {
		tl := target.In(loc)

		isWorkDay := false
		for _, d := range hours.WorkDays {
			if tl.Weekday() == d {
				isWorkDay = true
				break
			}
		}

		if isWorkDay {
			if i > 0 || lt.Hour() >= hours.EndHour {
				if i == 0 {
					target = target.AddDate(0, 0, 1)
				}
				result, err := setTimeInTimezone(target, hours.StartHour, 0, hours.Timezone)
				if err != nil {
					return time.Time{}, err
				}
				return result, nil
			}
			if lt.Hour() < hours.StartHour {
				return setTimeInTimezone(target, hours.StartHour, 0, hours.Timezone)
			}
		}

		target = target.AddDate(0, 0, 1)
	}

	return from, nil // Fallback
}
```

## Scheduling Helpers

### Defer Options

```go
// DeferOption represents a scheduling choice.
type DeferOption int

const (
	DeferLaterToday DeferOption = iota
	DeferThisAfternoon
	DeferTomorrow
	DeferThisWeekend
	DeferNextWeek
)

func computeDeferTime(option DeferOption, from time.Time, tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, err
	}
	lt := from.In(loc)

	switch option {
	case DeferLaterToday:
		// 2 hours from now, but at least 10am
		targetHour := lt.Hour() + 2
		if targetHour < 10 {
			targetHour = 10
		}
		return setTimeInTimezone(from, targetHour, 0, tz)

	case DeferThisAfternoon:
		// 1pm today
		return setTimeInTimezone(from, 13, 0, tz)

	case DeferTomorrow:
		// 8am tomorrow
		return tomorrowAt(from, 8, 0, tz)

	case DeferThisWeekend:
		// Saturday 8am
		saturday, err := getNextDayOfWeek(from, time.Saturday, tz)
		if err != nil {
			return time.Time{}, err
		}
		return setTimeInTimezone(saturday, 8, 0, tz)

	case DeferNextWeek:
		// Monday 8am
		monday, err := getNextDayOfWeek(from, time.Monday, tz)
		if err != nil {
			return time.Time{}, err
		}
		return setTimeInTimezone(monday, 8, 0, tz)
	}

	return from, nil
}
```

## Date Range Queries

### Items Due Today

```go
func findDueToday(items []DueItem, now time.Time, tz string) ([]DueItem, error) {
	var result []DueItem
	for _, item := range items {
		today, err := isToday(item.DueAt, now, tz)
		if err != nil {
			return nil, err
		}
		if today {
			result = append(result, item)
		}
	}
	return result, nil
}
```

### Overdue Items

```go
func findOverdue(items []DueItem, now time.Time) []DueItem {
	var result []DueItem
	for _, item := range items {
		if item.DueAt.Before(now) {
			result = append(result, item)
		}
	}
	return result
}
```

## Summary

Common patterns for datetime operations:

- **Adding time**: Use `time.Add` for durations, `time.AddDate` for calendar math
- **Finding days**: Calculate day-of-week with `time.In(loc).Weekday()`
- **Setting times**: Use `time.Date` with `*Location`, then `.UTC()`
- **Comparing**: Use `Before`/`After`/`Equal` directly on `time.Time`
- **Durations**: Use `time.Sub` to get `time.Duration`, then convert
- **Working hours**: Check day-of-week and hour in display timezone
- **Scheduling**: Combine day-finding with time-setting helpers
