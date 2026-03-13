package cmd

// binder_lock_cleanup_test.go — tests for lock file cleanup after operations.
//
// These tests verify that _binder.md.lock files are removed from disk after
// successful lock release.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// binderLockable is the interface satisfied by all file-IO types via binderLocker embedding.
type binderLockable interface {
	LockBinder(context.Context, string) (func() error, error)
}

// TestLockFileCleanup_RemovedAfterRelease verifies that the .lock file is
// removed from disk after the lock is released successfully.
func TestLockFileCleanup_RemovedAfterRelease(t *testing.T) {
	tests := []struct {
		name   string
		newIO  func() binderLockable
		ioName string
	}{
		{
			name:   "addchild IO",
			newIO:  func() binderLockable { return newDefaultAddChildIO() },
			ioName: "fileAddChildIO",
		},
		{
			name:   "delete IO",
			newIO:  func() binderLockable { return newDefaultDeleteIO() },
			ioName: "fileDeleteIO",
		},
		{
			name:   "move IO",
			newIO:  func() binderLockable { return newDefaultMoveIO() },
			ioName: "fileMoveIO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			binderPath := filepath.Join(dir, "_binder.md")
			if err := os.WriteFile(binderPath, []byte("<!-- prosemark-binder:v1 -->\n"), 0600); err != nil {
				t.Fatal(err)
			}

			fio := tt.newIO()
			ctx := context.Background()

			unlock, err := fio.LockBinder(ctx, binderPath)
			if err != nil {
				t.Fatalf("LockBinder: unexpected error: %v", err)
			}

			lockPath := binderPath + ".lock"

			// Lock file should exist while lock is held.
			if _, err := os.Stat(lockPath); os.IsNotExist(err) {
				t.Fatal("lock file does not exist while lock is held")
			}

			// Release the lock.
			if err := unlock(); err != nil {
				t.Fatalf("unlock: unexpected error: %v", err)
			}

			// After release, the lock file must be cleaned up.
			if _, err := os.Stat(lockPath); err == nil {
				t.Errorf("%s: lock file %s still exists after release, want removed", tt.ioName, lockPath)
			} else if !os.IsNotExist(err) {
				t.Fatalf("unexpected error checking lock file: %v", err)
			}
		})
	}
}

// TestLockFileCleanup_WriteBinderAtomicMerge verifies that writeBinderAtomicMergeImpl
// does not leave a .lock file after a successful write.
func TestLockFileCleanup_WriteBinderAtomicMerge(t *testing.T) {
	dir := t.TempDir()
	binderPath := filepath.Join(dir, "_binder.md")
	binderContent := []byte("<!-- prosemark-binder:v1 -->\n- [Chapter](chapter.md)\n")
	if err := os.WriteFile(binderPath, binderContent, 0600); err != nil {
		t.Fatal(err)
	}

	newContent := []byte("<!-- prosemark-binder:v1 -->\n- [Chapter](chapter.md)\n- [Scene](scene.md)\n")
	if err := writeBinderAtomicMergeImpl(binderPath, newContent); err != nil {
		t.Fatalf("writeBinderAtomicMergeImpl: unexpected error: %v", err)
	}

	lockPath := binderPath + ".lock"
	if _, err := os.Stat(lockPath); err == nil {
		t.Errorf("lock file %s still exists after writeBinderAtomicMergeImpl, want removed", lockPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking lock file: %v", err)
	}
}
