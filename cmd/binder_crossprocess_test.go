package cmd

// binder_crossprocess_test.go — RED tests for cross-process concurrent write protection.
//
// The existing concurrent tests (binder_concurrent_test.go, binder_concurrent_integration_test.go)
// verify in-process concurrency, where all goroutines share the same globalBinderLocks registry.
// Those tests pass because sync.Mutex serializes the read-merge-write cycle within a single process.
//
// These tests simulate CROSS-PROCESS concurrency: each goroutine uses its own independent
// binderLockRegistry (modeling separate OS processes with separate address spaces).
// Without OS-level file locking (flock/fcntl), the in-memory mutexes don't coordinate
// across processes, and concurrent writers can read stale disk state, causing lost updates.

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

// simulateCrossProcessWrite mimics what writeBinderAtomicMergeImpl does, but uses
// an independent binderLockRegistry (modeling a separate OS process address space).
// Each "process" acquires its own in-memory lock (no contention from other "processes"),
// reads current disk state, merges, and writes atomically.
func simulateCrossProcessWrite(registry *binderLockRegistry, path string, data []byte) error {
	unlock, err := registry.lock(context.Background(), path)
	if err != nil {
		return fmt.Errorf("acquiring binder lock: %w", err)
	}
	defer func() { _ = unlock() }()

	current, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("reading current binder: %w", readErr)
	}
	merged := mergeBinderLines(current, data)
	return writeFileAtomicDirectImpl(path, ".binder", merged)
}

// TestCrossProcess_ConcurrentAdds_AllEntriesPreserved simulates N separate OS processes
// each performing an add-child operation against the same binder file. Each goroutine
// uses its own binderLockRegistry (separate address space), so in-memory mutexes provide
// no cross-process coordination.
//
// A synchronization barrier ensures all goroutines read the same stale snapshot before
// any begins its write cycle — maximizing the chance of lost updates.
//
// EXPECTED TO FAIL: without OS-level file locking, the lock-read-merge-write cycle
// is not atomic across processes, and entries are silently lost.
func TestCrossProcess_ConcurrentAdds_AllEntriesPreserved(t *testing.T) {
	const n = 10

	dir := t.TempDir()
	binderPath := filepath.Join(dir, "_binder.md")
	initial := []byte("<!-- prosemark-binder:v1 -->\n")
	if err := os.WriteFile(binderPath, initial, 0600); err != nil {
		t.Fatal(err)
	}

	// Create target node files.
	targets := make([]string, n)
	for i := range targets {
		targets[i] = fmt.Sprintf("xproc%02d.md", i)
		if err := os.WriteFile(filepath.Join(dir, targets[i]), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}

	// Synchronization barrier: all goroutines complete read + modify before
	// any goroutine begins its write cycle.
	var readMu sync.Mutex
	readCount := 0
	allRead := make(chan struct{})

	errs := make([]error, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// ── Read (stale snapshot, before any writes) ─────────────────────
			data, err := os.ReadFile(binderPath)
			if err != nil {
				errs[idx] = fmt.Errorf("process %d read: %w", idx, err)
				return
			}

			// ── Modify ───────────────────────────────────────────────────────
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
				errs[idx] = fmt.Errorf("process %d ops.AddChild: diags=%v", idx, diags)
				return
			}

			// Signal read complete, wait for all to finish reading.
			readMu.Lock()
			readCount++
			if readCount == n {
				close(allRead)
			}
			readMu.Unlock()
			<-allRead

			// ── Write via independent lock registry (simulating separate process) ─
			// Each "process" has its own binderLockRegistry — no shared mutex.
			// This is the critical difference from in-process tests.
			registry := newBinderLockRegistry()
			if werr := simulateCrossProcessWrite(registry, binderPath, modified); werr != nil {
				errs[idx] = fmt.Errorf("process %d write: %w", idx, werr)
			}
		}(i)
	}

	wg.Wait()

	// Report unexpected I/O or operation errors.
	for idx, err := range errs {
		if err != nil {
			t.Errorf("process %d: unexpected error: %v", idx, err)
		}
	}

	// All N entries must survive in the final binder.
	final, err := os.ReadFile(binderPath)
	if err != nil {
		t.Fatal(err)
	}
	finalStr := string(final)

	var lost []string
	for _, target := range targets {
		if !strings.Contains(finalStr, target) {
			lost = append(lost, target)
		}
	}
	if len(lost) > 0 {
		t.Errorf("cross-process race: %d of %d entries silently lost: %v\nfinal binder:\n%s",
			len(lost), n, lost, finalStr)
	}
}

// TestCrossProcess_ConcurrentAdds_HighContention runs a higher-contention scenario
// with more concurrent "processes" to increase the probability of exposing the race.
// Uses 20 concurrent writers to stress the non-atomic read-merge-write cycle.
//
// EXPECTED TO FAIL: same cross-process race condition as above, but with higher
// probability of data loss due to increased contention.
func TestCrossProcess_ConcurrentAdds_HighContention(t *testing.T) {
	const n = 20

	dir := t.TempDir()
	binderPath := filepath.Join(dir, "_binder.md")
	initial := []byte("<!-- prosemark-binder:v1 -->\n")
	if err := os.WriteFile(binderPath, initial, 0600); err != nil {
		t.Fatal(err)
	}

	targets := make([]string, n)
	for i := range targets {
		targets[i] = fmt.Sprintf("hicon%02d.md", i)
		if err := os.WriteFile(filepath.Join(dir, targets[i]), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}

	var readMu sync.Mutex
	readCount := 0
	allRead := make(chan struct{})

	errs := make([]error, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			data, err := os.ReadFile(binderPath)
			if err != nil {
				errs[idx] = fmt.Errorf("process %d read: %w", idx, err)
				return
			}

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
				errs[idx] = fmt.Errorf("process %d ops.AddChild: diags=%v", idx, diags)
				return
			}

			readMu.Lock()
			readCount++
			if readCount == n {
				close(allRead)
			}
			readMu.Unlock()
			<-allRead

			registry := newBinderLockRegistry()
			if werr := simulateCrossProcessWrite(registry, binderPath, modified); werr != nil {
				errs[idx] = fmt.Errorf("process %d write: %w", idx, werr)
			}
		}(i)
	}

	wg.Wait()

	for idx, err := range errs {
		if err != nil {
			t.Errorf("process %d: unexpected error: %v", idx, err)
		}
	}

	final, err := os.ReadFile(binderPath)
	if err != nil {
		t.Fatal(err)
	}
	finalStr := string(final)

	var lost []string
	for _, target := range targets {
		if !strings.Contains(finalStr, target) {
			lost = append(lost, target)
		}
	}
	if len(lost) > 0 {
		t.Errorf("cross-process high contention: %d of %d entries silently lost: %v\nfinal binder:\n%s",
			len(lost), n, lost, finalStr)
	}
}
