package cmd

// binder_concurrent_test.go — behavioral RED test for the concurrent write race.
//
// This test demonstrates the lost-update bug: concurrent pmk mutation commands
// all read the same binder state, each modify it independently, and then each
// write back — last writer wins, silently discarding all other writers' changes.
//
// The test is DETERMINISTIC: a synchronization barrier forces every goroutine to
// finish its read before any goroutine begins its write, guaranteeing all N
// goroutines work from the same stale snapshot and that N-1 entries are lost.
//
// Expected behaviour after the fix: all N entries survive, protected by the
// LockBinder exclusive lock that serialises the read-modify-write cycle.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
	"github.com/eykd/prosemark-go/internal/binder/ops"
)

// TestConcurrentAddChild_AllEntriesPreserved runs N concurrent add-child
// operations against the same _binder.md file using a synchronization barrier
// that guarantees all reads happen before any write (worst-case race scenario).
//
// Assertion: all N target entries appear in the final binder.
//
// This test FAILS in the RED phase because the current implementation has no
// locking; the last writer wins and N-1 entries are silently discarded.
func TestConcurrentAddChild_AllEntriesPreserved(t *testing.T) {
	const n = 5

	dir := t.TempDir()
	binderPath := filepath.Join(dir, "_binder.md")
	initial := []byte("<!-- prosemark-binder:v1 -->\n")
	if err := os.WriteFile(binderPath, initial, 0600); err != nil {
		t.Fatal(err)
	}

	// Create the target node files that will be added.
	targets := make([]string, n)
	for i := range targets {
		targets[i] = fmt.Sprintf("node%02d.md", i)
		if err := os.WriteFile(filepath.Join(dir, targets[i]), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}

	// Synchronization barrier: all goroutines complete their read before any
	// goroutine begins its write. This maximises the chance of a lost update.
	var readMu sync.Mutex
	readCount := 0
	allRead := make(chan struct{})

	errs := make([]error, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// ── Read ─────────────────────────────────────────────────────────
			data, err := os.ReadFile(binderPath)
			if err != nil {
				errs[idx] = fmt.Errorf("goroutine %d read: %w", idx, err)
				return
			}

			// Signal that this goroutine has read, then wait for all to read.
			readMu.Lock()
			readCount++
			if readCount == n {
				close(allRead)
			}
			readMu.Unlock()
			<-allRead // block until every goroutine has read the stale snapshot

			// ── Modify ────────────────────────────────────────────────────────
			proj := &binder.Project{
				Files:     targets,
				BinderDir: dir,
			}
			params := binder.AddChildParams{
				ParentSelector: ".",
				Target:         targets[idx],
				Position:       "last",
			}
			modified, diags, opErr := ops.AddChild(context.Background(), data, proj, params)
			if opErr != nil || hasDiagnosticError(diags) {
				errs[idx] = fmt.Errorf("goroutine %d ops.AddChild: err=%v diags=%v", idx, opErr, diags)
				return
			}

			// ── Write ─────────────────────────────────────────────────────────
			// Each goroutine independently writes back its single-entry version.
			// Without locking, each write overwrites the previous goroutine's
			// work — last writer wins.
			if werr := writeFileAtomicImpl(binderPath, ".binder", modified); werr != nil {
				errs[idx] = fmt.Errorf("goroutine %d write: %w", idx, werr)
			}
		}(i)
	}

	wg.Wait()

	// Report any unexpected I/O or operation errors.
	for idx, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", idx, err)
		}
	}

	// All N entries must survive in the final binder.
	final, err := os.ReadFile(binderPath)
	if err != nil {
		t.Fatal(err)
	}
	finalStr := string(final)

	for _, target := range targets {
		if !strings.Contains(finalStr, target) {
			t.Errorf("concurrent write race: entry %q was silently lost (last-writer-wins)", target)
		}
	}
}
