package cmd

// binder_concurrent_integration_test.go — GREEN tests for concurrent write protection.
//
// Test 1: TestWriteBinderAtomic_InterfaceMethodPath_ConcurrentCallsPreserveAllEntries
//   Exercises the production code path: io.WriteBinderAtomic → WriteBinderAtomicImpl →
//   writeBinderAtomicMergeImpl (lock+merge). Verifies all N entries survive concurrent writes.
//
// Test 2: TestAddChildCmd_RunE_ConcurrentCallsPreserveAllEntries
//   Runs N concurrent add-child command executions through newAddChildCmdWithGetCWD.
//   Verifies all N entries survive via the full command handler path.

import (
	"bytes"
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

// TestWriteBinderAtomic_InterfaceMethodPath_ConcurrentCallsPreserveAllEntries runs N
// concurrent add-child operations against the same _binder.md using a synchronization
// barrier (all reads before any write) and writes via the AddChildIO.WriteBinderAtomic
// interface method — the code path taken by the real command at runtime.
//
// This test FAILS in RED because WriteBinderAtomicImpl routes to writeBinderCheckedImpl
// (no lock, no merge) rather than writeBinderAtomicMergeImpl (locked read-merge-write).
// Last-writer-wins silently drops N-1 entries.
func TestWriteBinderAtomic_InterfaceMethodPath_ConcurrentCallsPreserveAllEntries(t *testing.T) {
	const n = 5

	dir := t.TempDir()
	binderPath := filepath.Join(dir, "_binder.md")
	initial := []byte("<!-- prosemark-binder:v1 -->\n")
	if err := os.WriteFile(binderPath, initial, 0600); err != nil {
		t.Fatal(err)
	}

	targets := make([]string, n)
	for i := range targets {
		targets[i] = fmt.Sprintf("iface%02d.md", i)
		if err := os.WriteFile(filepath.Join(dir, targets[i]), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}

	// Synchronization barrier: all goroutines complete their read before any write.
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

			// ── Write via the AddChildIO interface method ──────────────────────
			// Exercises the production path: WriteBinderAtomic → WriteBinderAtomicImpl
			// → writeBinderAtomicMergeImpl (lock + merge). All N entries must survive.
			var io AddChildIO = newDefaultAddChildIO()
			if werr := io.WriteBinderAtomic(context.Background(), binderPath, modified); werr != nil {
				errs[idx] = fmt.Errorf("goroutine %d WriteBinderAtomic: %w", idx, werr)
			}
		}(i)
	}

	wg.Wait()

	for idx, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", idx, err)
		}
	}

	final, err := os.ReadFile(binderPath)
	if err != nil {
		t.Fatal(err)
	}
	finalStr := string(final)

	for _, target := range targets {
		if !strings.Contains(finalStr, target) {
			t.Errorf("interface path race: entry %q lost", target)
		}
	}
}

// commandBarrierIO wraps fileAddChildIO to add a synchronization barrier inside
// ReadBinder. It holds all n callers at the barrier until all have arrived, then
// releases them simultaneously. This guarantees that all concurrent command
// executions start their write from the same stale snapshot — worst-case race.
//
// All fields are shared across instances to coordinate across goroutines.
type commandBarrierIO struct {
	*fileAddChildIO
	mu      *sync.Mutex
	count   *int
	n       int
	allRead chan struct{}
	once    *sync.Once
}

func (b *commandBarrierIO) ReadBinder(ctx context.Context, path string) ([]byte, error) {
	data, err := b.fileAddChildIO.ReadBinder(ctx, path)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	*b.count++
	if *b.count >= b.n {
		b.once.Do(func() { close(b.allRead) })
	}
	b.mu.Unlock()

	select {
	case <-b.allRead:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return data, nil
}

// TestAddChildCmd_RunE_ConcurrentCallsPreserveAllEntries runs N concurrent executions
// of the full add-child cobra command handler against the same project directory.
// A commandBarrierIO intercepts ReadBinder to hold all goroutines at the barrier
// until all have read, then releases them — guaranteeing a worst-case write race.
// Asserts all N entries survive via writeBinderAtomicMergeImpl (lock + merge).
func TestAddChildCmd_RunE_ConcurrentCallsPreserveAllEntries(t *testing.T) {
	const n = 5

	dir := t.TempDir()
	binderPath := filepath.Join(dir, "_binder.md")
	initial := []byte("<!-- prosemark-binder:v1 -->\n")
	if err := os.WriteFile(binderPath, initial, 0600); err != nil {
		t.Fatal(err)
	}

	targets := make([]string, n)
	for i := range targets {
		targets[i] = fmt.Sprintf("cmd%02d.md", i)
		if err := os.WriteFile(filepath.Join(dir, targets[i]), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}

	// Shared barrier state.
	var (
		mu      sync.Mutex
		count   int
		allRead = make(chan struct{})
		once    sync.Once
	)

	errs := make([]error, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Each goroutine gets its own IO instance (simulates separate processes)
			// but all share the barrier state to guarantee worst-case race timing.
			bio := &commandBarrierIO{
				fileAddChildIO: newDefaultAddChildIO(),
				mu:             &mu,
				count:          &count,
				n:              n,
				allRead:        allRead,
				once:           &once,
			}

			cmd := newAddChildCmdWithGetCWD(bio, func() (string, error) { return dir, nil })
			cmd.SetArgs([]string{
				"--project", dir,
				"--parent", ".",
				"--target", targets[idx],
			})
			var out, errOut bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)

			if err := cmd.Execute(); err != nil {
				errs[idx] = fmt.Errorf("goroutine %d cmd.Execute: %v (stderr: %s)", idx, err, errOut.String())
			}
		}(i)
	}

	wg.Wait()

	for idx, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", idx, err)
		}
	}

	final, err := os.ReadFile(binderPath)
	if err != nil {
		t.Fatal(err)
	}
	finalStr := string(final)

	for _, target := range targets {
		if !strings.Contains(finalStr, target) {
			t.Errorf("cmd handler race: entry %q lost", target)
		}
	}
}

// TestWriteBinderAtomic_InterfaceMethodUsesLockAndMerge verifies that calling
// WriteBinderAtomic on a fileAddChildIO acquires the global binder lock and merges
// the incoming data with the current on-disk content.
//
// Regression guard: if WriteBinderAtomicImpl stops calling writeBinderAtomicMergeImpl,
// a concurrent caller will overwrite without merging and this test will fail.
func TestWriteBinderAtomic_InterfaceMethodUsesLockAndMerge(t *testing.T) {
	dir := t.TempDir()
	binderPath := filepath.Join(dir, "_binder.md")

	// Start with a binder that has one existing entry.
	existing := []byte("<!-- prosemark-binder:v1 -->\n- [Existing](existing.md)\n")
	if err := os.WriteFile(binderPath, existing, 0600); err != nil {
		t.Fatal(err)
	}

	// Write a new payload that contains a different entry (no existing entry).
	// If WriteBinderAtomic merges, both entries survive.
	// If WriteBinderAtomic does not merge, existing is overwritten and lost.
	incoming := []byte("<!-- prosemark-binder:v1 -->\n- [Incoming](incoming.md)\n")

	var io AddChildIO = newDefaultAddChildIO()
	if err := io.WriteBinderAtomic(context.Background(), binderPath, incoming); err != nil {
		t.Fatalf("WriteBinderAtomic: %v", err)
	}

	final, err := os.ReadFile(binderPath)
	if err != nil {
		t.Fatal(err)
	}
	finalStr := string(final)

	// Both entries must survive if WriteBinderAtomic uses lock+merge.
	if !strings.Contains(finalStr, "existing.md") {
		t.Errorf("WriteBinderAtomic did not merge: existing entry lost (interface method bypasses lock+merge)")
	}
	if !strings.Contains(finalStr, "incoming.md") {
		t.Errorf("WriteBinderAtomic did not write incoming entry")
	}
}
