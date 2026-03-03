package cmd

// binder_concurrent_test.go — behavioral test for concurrent write protection.
//
// Demonstrates that concurrent pmk mutation commands writing to the same binder
// all preserve their entries. The test is DETERMINISTIC: a synchronization barrier
// forces every goroutine to finish its read before any goroutine begins its write,
// maximising the chance of a lost update. All N entries must survive, protected
// by the writeBinderAtomicMergeImpl lock-and-merge path.

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
// Assertion: all N target entries appear in the final binder, preserved by
// writeBinderAtomicMergeImpl's exclusive lock and union-merge strategy.
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
			modified, diags := ops.AddChild(context.Background(), data, proj, params)
			if hasDiagnosticError(diags) {
				errs[idx] = fmt.Errorf("goroutine %d ops.AddChild: diags=%v", idx, diags)
				return
			}

			// ── Write ─────────────────────────────────────────────────────────
			// Each goroutine independently writes back its single-entry version.
			// Without locking, each write overwrites the previous goroutine's
			// work — last writer wins.
			if werr := writeBinderAtomicMergeImpl(binderPath, modified); werr != nil {
				errs[idx] = fmt.Errorf("goroutine %d write: %w", idx, werr)
			}
		}(i)
	}

	wg.Wait()

	// Report any unexpected I/O or operation errors.
	for idx, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", idx, err)
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
