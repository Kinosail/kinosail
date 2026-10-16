package catalog_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
)

func TestRefreshPublishesLibraryWhenOneSubdirectoryIsUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not restrict root")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Before (2020).mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := catalog.NewIndex(t.Context(), []catalog.ScanRoot{{Path: root, Namespace: "Movies"}}, "", time.Hour, nil)
	locked := filepath.Join(root, "Locked")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(locked, "Hidden (2021).mp4"), filepath.Join(root, "After (2022).mp4")} {
		if err := os.WriteFile(path, []byte("media"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) }) //nolint:gosec // Cleanup restores owner-only directory traversal in the test sandbox.
	if err := index.Refresh(t.Context()); err != nil {
		t.Fatalf("refresh with unreadable subdirectory failed: %v", err)
	}
	snapshot, err := index.Library()
	titles := map[string]bool{}
	for _, item := range snapshot.Items() {
		titles[item.Title] = true
	}
	if err != nil || len(titles) != 2 || !titles["Before"] || !titles["After"] {
		t.Fatalf("published library = %#v, %v", snapshot.Items(), err)
	}
}
