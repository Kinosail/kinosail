package metadata

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func artworkCacheFixture(t *testing.T) (string, map[string]string) {
	t.Helper()
	cache := t.TempDir()
	root := filepath.Join(cache, "metadata")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, name := range []string{"current.jpg", "expired.JPG", "keep.txt"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("art"), 0o600); err != nil {
			t.Fatal(err)
		}
		if name != "current.jpg" {
			old := time.Now().Add(-ArtworkMaxAge - time.Hour)
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
		}
		paths[name] = path
	}
	return cache, paths
}

func TestArtworkCacheValidity(t *testing.T) {
	t.Parallel()
	cache, paths := artworkCacheFixture(t)
	root := filepath.Join(cache, "metadata")
	if got := CurrentArtwork(cache, paths["current.jpg"]); got != paths["current.jpg"] {
		t.Fatalf("current artwork = %q", got)
	}
	for _, pair := range [][2]string{{"", paths["current.jpg"]}, {cache, ""}, {cache, cache}, {cache, filepath.Join(cache, "outside.jpg")}, {cache, "relative.jpg"}, {cache, root}, {cache, filepath.Join(root, "missing.jpg")}, {cache, paths["expired.JPG"]}} {
		if got := CurrentArtwork(pair[0], pair[1]); got != "" {
			t.Fatalf("invalid artwork accepted: %q", got)
		}
	}
}

func TestArtworkCacheExpiryPreservesOtherEntries(t *testing.T) {
	t.Parallel()
	cache, paths := artworkCacheFixture(t)
	root := filepath.Join(cache, "metadata")
	if err := os.Mkdir(filepath.Join(root, "directory.jpg"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := PruneExpiredArtwork(cache); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths["expired.JPG"]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired image remains: %v", err)
	}
	for _, path := range []string{paths["current.jpg"], paths["keep.txt"], filepath.Join(root, "directory.jpg")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("unrelated cache entry changed: %v", err)
		}
	}
	for _, empty := range []string{"", t.TempDir()} {
		if err := PruneExpiredArtwork(empty); err != nil {
			t.Fatalf("empty cache: %v", err)
		}
	}
}

type unreadableArtwork struct{ os.DirEntry }

func (unreadableArtwork) Info() (os.FileInfo, error) { return nil, os.ErrPermission }

func TestArtworkPruningPropagatesStorageFailures(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	path := filepath.Join(cache, "expired.jpg")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-ArtworkMaxAge - time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		entries                  []os.DirEntry
		readErr, removeErr, want error
		removes                  int
	}{
		{readErr: os.ErrPermission, want: os.ErrPermission},
		{entries: []os.DirEntry{unreadableArtwork{entries[0]}}, want: os.ErrPermission},
		{entries: entries, removeErr: os.ErrPermission, want: os.ErrPermission, removes: 1},
		{entries: entries, removeErr: os.ErrNotExist, removes: 1},
	} {
		calls := 0
		err := pruneExpiredArtwork(cache, func(string) ([]os.DirEntry, error) {
			return test.entries, test.readErr
		}, func(target string) error {
			calls++
			if target != filepath.Join(cache, "metadata", "expired.jpg") {
				t.Fatalf("unexpected removal target: %s", target)
			}
			return test.removeErr
		})
		if !errors.Is(err, test.want) || calls != test.removes {
			t.Fatalf("prune error=%v calls=%d; want %v/%d", err, calls, test.want, test.removes)
		}
	}
}
