package playback

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestHLSCacheControlMeasuresAndDiscardsWithoutHoldingTheJobLock(t *testing.T) {
	root := t.TempDir()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	for _, name := range []string{"0123456789abcdef", "fedcba9876543210", "notes"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(root, name, "segment-00000.m4s"), "0123456789")
	}
	var lock sync.Mutex
	control := NewHLSCacheControl(root, &lock, func(string) bool { return false }, func() bool { return false }, policy)
	lock.Lock()
	measured := make(chan int64, 1)
	go func() {
		size, err := control.Stats()
		if err != nil {
			size = -1
		}
		measured <- size
	}()
	select {
	case size := <-measured:
		if size != 20 {
			t.Fatalf("stats = %d", size)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cache stats waited for the job lock")
	}
	lock.Unlock()
	if err := os.Chtimes(filepath.Join(root, "0123456789abcdef"), time.Unix(1, 0), time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	if remaining, err := control.Prune(10); err != nil || remaining != 10 {
		t.Fatalf("prune = %d, %v", remaining, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 || entries[0].Name() != "fedcba9876543210" || entries[1].Name() != "notes" {
		t.Fatalf("cache after prune = %v, %v", entries, err)
	}
	if err := control.Clear(); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "notes" {
		t.Fatalf("cache after clear = %v, %v", entries, err)
	}
}

func discardHLSCacheEntry(cache string) func(string) (string, bool, error) {
	return func(name string) (string, bool, error) {
		trash, err := detachHLSCacheEntry(cache, name)
		return trash, true, err
	}
}

func neverActive(string) bool { return false }

func testHLSCache(cache string, active func(string) bool, policy HLSRecipePolicy) HLSCacheControl {
	return NewHLSCacheControl(cache, new(sync.Mutex), active, func() bool { return false }, policy)
}
