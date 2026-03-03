package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// binderLockRegistry maintains per-path exclusive mutexes for binder files,
// serializing concurrent read-modify-write cycles to prevent lost updates.
type binderLockRegistry struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newBinderLockRegistry() *binderLockRegistry {
	return &binderLockRegistry{locks: make(map[string]*sync.Mutex)}
}

// getLock returns the exclusive mutex for path, creating it if necessary.
func (r *binderLockRegistry) getLock(path string) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.locks[path]; !ok {
		r.locks[path] = &sync.Mutex{}
	}
	return r.locks[path]
}

// lock acquires the exclusive lock for path, respecting ctx cancellation.
// Returns an unlock function and nil on success, or a non-nil error if ctx
// is cancelled before the lock is acquired.
func (r *binderLockRegistry) lock(ctx context.Context, path string) (func() error, error) {
	mu := r.getLock(path)
	if mu.TryLock() {
		return func() error { mu.Unlock(); return nil }, nil
	}
	done := make(chan struct{})
	go func() {
		mu.Lock()
		close(done)
	}()
	select {
	case <-done:
		return func() error { mu.Unlock(); return nil }, nil
	case <-ctx.Done():
		// Release the lock once the background goroutine acquires it.
		go func() { <-done; mu.Unlock() }()
		return nil, fmt.Errorf("acquiring binder lock: %w", ctx.Err())
	}
}

// globalBinderLocks is the package-level registry for binder file locks.
var globalBinderLocks = newBinderLockRegistry()

// binderLocker provides LockBinder backed by the global lock registry.
// Embed this in file-IO structs to satisfy the LockBinder contract without repetition.
type binderLocker struct{}

// LockBinder acquires an exclusive per-file lock for the binder at path.
// The returned function releases the lock. ctx cancellation aborts the wait.
func (binderLocker) LockBinder(ctx context.Context, path string) (func() error, error) {
	return globalBinderLocks.lock(ctx, path)
}

// mergeBinderLines merges concurrent add-child writes that started from the
// same stale snapshot. Lines from incoming appear first (preserving the
// caller's computed insertion order), followed by any lines from current that
// are not already accounted for by incoming (counted by multiplicity).
//
// Using multiset counts instead of a simple seen-set means that structurally
// identical lines (e.g. two "- [Chapter](chapter.md)" nodes, or the same
// child appended to multiple matched parents) are both preserved rather than
// collapsed into one.
func mergeBinderLines(current, incoming []byte) []byte {
	currentLines := splitLines(current)
	incomingLines := splitLines(incoming)

	// Count occurrences of each line in incoming.
	incomingCount := make(map[string]int, len(incomingLines))
	for _, line := range incomingLines {
		incomingCount[line]++
	}

	// Start with all incoming lines in order.
	result := make([]string, len(incomingLines))
	copy(result, incomingLines)

	// Append lines from current that are not sufficiently covered by incoming.
	// These represent entries added by concurrent commands after our snapshot.
	currentSeen := make(map[string]int, len(currentLines))
	for _, line := range currentLines {
		currentSeen[line]++
		if currentSeen[line] > incomingCount[line] {
			result = append(result, line)
		}
	}

	return []byte(strings.Join(result, "\n") + "\n")
}

// splitLines splits b on newlines, dropping the empty string produced by a
// trailing newline. Returns nil for empty or all-whitespace input.
func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	s := string(b)
	if s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
