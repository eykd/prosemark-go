package cmd

// binder_lock_test.go — RED tests for exclusive binder file locking.
//
// These tests reference LockBinder methods that do not yet exist on
// fileAddChildIO, fileDeleteIO, or fileMoveIO. They fail to compile in
// the RED phase; the compile errors are the failing signal.
//
// When GREEN: LockBinder is added to each file IO implementation and to
// the corresponding IO interfaces (AddChildIO, DeleteIO, MoveIO).
// The IO interfaces must also be updated so mock implementations satisfy
// the interface in existing tests.

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestFileAddChildIO_LockBinder_AcquiresExclusiveLock verifies that fileAddChildIO
// implements LockBinder, that the returned unlock releases the lock, and that a
// second concurrent caller blocks while the first lock is held.
func TestFileAddChildIO_LockBinder_AcquiresExclusiveLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	if err := os.WriteFile(path, acBinder(), 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultAddChildIO()
	ctx := context.Background()

	unlock, err := fio.LockBinder(ctx, path)
	if err != nil {
		t.Fatalf("first LockBinder: unexpected error: %v", err)
	}

	// While the first lock is held, a second caller must block.
	second := make(chan error, 1)
	go func() {
		unlock2, err2 := fio.LockBinder(ctx, path)
		if err2 == nil {
			_ = unlock2()
		}
		second <- err2
	}()

	// Give the goroutine enough time to reach the lock acquisition attempt.
	time.Sleep(50 * time.Millisecond)

	select {
	case err2 := <-second:
		t.Errorf("second LockBinder returned while first lock is still held (err=%v)", err2)
	default:
		// expected: second caller is blocking
	}

	// Release the first lock.
	if err := unlock(); err != nil {
		t.Fatalf("unlock: unexpected error: %v", err)
	}

	// Now the second caller must be able to proceed.
	select {
	case err2 := <-second:
		if err2 != nil {
			t.Errorf("second LockBinder failed after first was released: %v", err2)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("second LockBinder did not acquire lock within 500ms of first being released")
	}
}

// TestFileAddChildIO_LockBinder_PerFileIsolation verifies that locks on different
// binder paths do not block each other.
func TestFileAddChildIO_LockBinder_PerFileIsolation(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	path1 := filepath.Join(dir1, "_binder.md")
	path2 := filepath.Join(dir2, "_binder.md")
	for _, p := range []string{path1, path2} {
		if err := os.WriteFile(p, acBinder(), 0600); err != nil {
			t.Fatal(err)
		}
	}

	fio := newDefaultAddChildIO()
	ctx := context.Background()

	unlock1, err := fio.LockBinder(ctx, path1)
	if err != nil {
		t.Fatalf("LockBinder(path1): %v", err)
	}
	defer func() { _ = unlock1() }()

	// Locking a different file must succeed immediately while path1 is locked.
	var wg sync.WaitGroup
	wg.Add(1)
	acquired := make(chan bool, 1)
	go func() {
		defer wg.Done()
		unlock2, err2 := fio.LockBinder(ctx, path2)
		acquired <- (err2 == nil)
		if err2 == nil {
			_ = unlock2()
		}
	}()

	select {
	case ok := <-acquired:
		if !ok {
			t.Error("LockBinder(path2) failed while path1 is locked — locks must be per-file")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("LockBinder(path2) blocked while path1 is locked — locks must be per-file")
	}
	wg.Wait()
}

// TestFileDeleteIO_LockBinder_AcquiresExclusiveLock verifies that fileDeleteIO
// also implements LockBinder with the same exclusive-lock semantics.
func TestFileDeleteIO_LockBinder_AcquiresExclusiveLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	if err := os.WriteFile(path, acBinder(), 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultDeleteIO()
	ctx := context.Background()

	unlock, err := fio.LockBinder(ctx, path)
	if err != nil {
		t.Fatalf("first LockBinder: unexpected error: %v", err)
	}

	second := make(chan error, 1)
	go func() {
		unlock2, err2 := fio.LockBinder(ctx, path)
		if err2 == nil {
			_ = unlock2()
		}
		second <- err2
	}()

	time.Sleep(50 * time.Millisecond)

	select {
	case err2 := <-second:
		t.Errorf("second LockBinder returned while first lock is still held (err=%v)", err2)
	default:
		// expected
	}

	if err := unlock(); err != nil {
		t.Fatalf("unlock: unexpected error: %v", err)
	}

	select {
	case err2 := <-second:
		if err2 != nil {
			t.Errorf("second LockBinder failed after first was released: %v", err2)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("second LockBinder did not acquire lock within 500ms")
	}
}

// TestFileMoveIO_LockBinder_AcquiresExclusiveLock verifies that fileMoveIO
// also implements LockBinder with the same exclusive-lock semantics.
func TestFileMoveIO_LockBinder_AcquiresExclusiveLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	if err := os.WriteFile(path, acBinder(), 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultMoveIO()
	ctx := context.Background()

	unlock, err := fio.LockBinder(ctx, path)
	if err != nil {
		t.Fatalf("first LockBinder: unexpected error: %v", err)
	}

	second := make(chan error, 1)
	go func() {
		unlock2, err2 := fio.LockBinder(ctx, path)
		if err2 == nil {
			_ = unlock2()
		}
		second <- err2
	}()

	time.Sleep(50 * time.Millisecond)

	select {
	case err2 := <-second:
		t.Errorf("second LockBinder returned while first lock is still held (err=%v)", err2)
	default:
		// expected
	}

	if err := unlock(); err != nil {
		t.Fatalf("unlock: unexpected error: %v", err)
	}

	select {
	case err2 := <-second:
		if err2 != nil {
			t.Errorf("second LockBinder failed after first was released: %v", err2)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("second LockBinder did not acquire lock within 500ms")
	}
}

// TestFileAddChildIO_LockBinder_ContextCancellationAborts verifies that
// LockBinder respects context cancellation: if ctx is cancelled while waiting
// for the lock, the call returns a non-nil error.
func TestFileAddChildIO_LockBinder_ContextCancellationAborts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	if err := os.WriteFile(path, acBinder(), 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultAddChildIO()

	// Hold the lock indefinitely in the background.
	unlock, err := fio.LockBinder(context.Background(), path)
	if err != nil {
		t.Fatalf("first LockBinder: %v", err)
	}
	defer func() { _ = unlock() }()

	// Cancel the context before a second caller can acquire.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = fio.LockBinder(ctx, path)
	if err == nil {
		t.Error("LockBinder returned nil error when context was cancelled while waiting")
	}
}
