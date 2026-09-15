package playback

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHLSCachePropagatesTraversalAndRemovalFailures(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission failures are not observable as root")
	}
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	root := t.TempDir()
	owned := filepath.Join(root, "0123456789abcdef")
	blocked := filepath.Join(owned, "blocked")
	if err := os.MkdirAll(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) }) //nolint:gosec // Cleanup must restore directory traversal before TempDir removal.
	if _, err := HLSCacheStats(root, policy); err == nil {
		t.Fatal("cache stats ignored traversal failure")
	}
	if _, err := PruneHLSCache(root, 0, func(string) bool { return false }, policy); err == nil {
		t.Fatal("cache prune ignored traversal failure")
	}
	if err := os.Chmod(blocked, 0o700); err != nil { //nolint:gosec // The test restores owner traversal before exercising removal failure.
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o500); err != nil { //nolint:gosec // The test needs a non-writable owner directory to exercise removal failure.
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) }) //nolint:gosec // Cleanup must restore directory access before TempDir removal.
	if err := ClearHLSCache(root, policy); err == nil {
		t.Fatal("cache clear ignored removal failure")
	}
}
