package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestBinderLockRegistry_Lock_FlockAcquireError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	registry := newBinderLockRegistry()
	registry.acquireFlock = func(_ string) (flockResult, error) {
		return flockResult{}, fmt.Errorf("flock error")
	}

	_, err := registry.lock(context.Background(), path)
	if err == nil {
		t.Fatal("expected error from flock acquire failure, got nil")
	}
	if err.Error() != "flock error" {
		t.Errorf("got error %q, want %q", err, "flock error")
	}

	// Verify the in-memory mutex was released (can re-acquire).
	registry.acquireFlock = func(_ string) (flockResult, error) {
		return flockResult{release: func() error { return nil }}, nil
	}
	unlock, err := registry.lock(context.Background(), path)
	if err != nil {
		t.Fatalf("re-acquiring lock after flock failure: %v", err)
	}
	if err := unlock(); err != nil {
		t.Fatalf("unlock: %v", err)
	}
}
