package node

import "time"

// NowUTC returns the current UTC time formatted as RFC3339 with second-level
// precision and a "Z" suffix, e.g. "2006-01-02T15:04:05Z".
func NowUTC() string {
	return time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
}
