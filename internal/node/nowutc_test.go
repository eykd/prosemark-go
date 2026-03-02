package node_test

import (
	"strings"
	"testing"
	"time"

	node "github.com/eykd/prosemark-go/internal/node"
)

// TestNowUTC verifies that NowUTC returns a valid RFC3339 timestamp
// with a "Z" suffix and a time close to the current UTC time.
// This test fails to compile until NowUTC is implemented in the node package.
func TestNowUTC(t *testing.T) {
	got := node.NowUTC()

	if got == "" {
		t.Fatal("NowUTC() returned empty string")
	}

	if !strings.HasSuffix(got, "Z") {
		t.Errorf("NowUTC() = %q, want suffix 'Z'", got)
	}

	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("NowUTC() = %q, not a valid RFC3339 timestamp: %v", got, err)
	}

	now := time.Now().UTC()
	diff := now.Sub(parsed).Abs()
	if diff > 5*time.Second {
		t.Errorf("NowUTC() = %q is not within 5s of current time (diff = %v)", got, diff)
	}
}

// TestNowUTC_SecondPrecision verifies that NowUTC produces second-level precision
// (no sub-second component), matching the RFC3339 format "2006-01-02T15:04:05Z".
func TestNowUTC_SecondPrecision(t *testing.T) {
	got := node.NowUTC()

	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("NowUTC() = %q is not a valid RFC3339 timestamp: %v", got, err)
	}

	// Second-level precision: nanoseconds component must be zero.
	if parsed.Nanosecond() != 0 {
		t.Errorf("NowUTC() = %q has sub-second precision; want second-level precision only", got)
	}
}

// TestNowUTC_IsUTC verifies that NowUTC produces a UTC timestamp (not local time).
func TestNowUTC_IsUTC(t *testing.T) {
	got := node.NowUTC()

	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("NowUTC() = %q is not a valid RFC3339 timestamp: %v", got, err)
	}

	if parsed.Location() != time.UTC {
		t.Errorf("NowUTC() = %q, location = %v, want UTC", got, parsed.Location())
	}
}
