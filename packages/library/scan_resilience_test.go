package library_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestScanSkipsUnreadableSubdirectoryAndKeepsTheRestOfTheRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not restrict root")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "Locked")
	for _, path := range []string{filepath.Join(root, "Visible (2020).mp4"), filepath.Join(locked, "Hidden (2021).mp4")} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("media"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) }) //nolint:gosec // Cleanup restores owner-only directory traversal in the test sandbox.
	items, err := library.ScanContext(t.Context(), root, "Movies")
	if err != nil || len(items) != 1 || items[0].Title != "Visible" {
		t.Fatalf("scan with unreadable subdirectory = %#v, %v", items, err)
	}
}

func TestScanTreatsEntriesRemovedDuringScanAsRemoved(t *testing.T) {
	root := t.TempDir()
	stable := filepath.Join(root, "Stable (2020).mp4")
	if err := os.WriteFile(stable, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	churn := filepath.Join(root, "Imports")
	if err := os.Mkdir(churn, 0o700); err != nil {
		t.Fatal(err)
	}
	const files = 200
	for index := range files {
		if err := os.WriteFile(filepath.Join(churn, fmt.Sprintf("Episode %03d.mkv", index)), []byte("media"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stop := make(chan struct{})
	var churning sync.WaitGroup
	churning.Go(func() {
		for round := 0; ; round++ {
			select {
			case <-stop:
				return
			default:
			}
			index := round % files
			current := filepath.Join(churn, fmt.Sprintf("Episode %03d.mkv", index))
			moved := filepath.Join(churn, fmt.Sprintf("Episode %03d.partial", index))
			_ = os.Rename(current, moved)
			_ = os.Rename(moved, current)
			nested := filepath.Join(churn, fmt.Sprintf("Season %d", round%4))
			_ = os.MkdirAll(filepath.Join(nested, "Extras"), 0o700)
			_ = os.WriteFile(filepath.Join(nested, "Extras", "Clip.mkv"), []byte("media"), 0o600)
			_ = os.RemoveAll(nested)
		}
	})
	defer func() {
		close(stop)
		churning.Wait()
	}()
	for range 100 {
		items, err := library.ScanContext(t.Context(), root, "Shows")
		if err != nil {
			t.Fatalf("scan during imports failed: %v", err)
		}
		found := false
		for _, item := range items {
			found = found || item.Path == stable
		}
		if !found || len(items) > files+5 {
			t.Fatalf("scan during imports = %d items, stable item found = %t", len(items), found)
		}
	}
}
