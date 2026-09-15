package servertest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// ArtworkExpiryFixture binds projection and pruning on the same real store.
type ArtworkExpiryFixture struct {
	Apply func([]library.Item) []library.Item
	Prune func() error
}

// ArtworkExpiresAndIsNotApplied checks both cache visibility and file cleanup.
func ArtworkExpiresAndIsNotApplied(t *testing.T, maximumAge time.Duration, build func(string, string) ArtworkExpiryFixture) {
	t.Helper()
	cache := t.TempDir()
	path := filepath.Join(cache, "metadata", "item.jpg")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("poster"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now().Add(-maximumAge-time.Hour), time.Now().Add(-maximumAge-time.Hour)); err != nil {
		t.Fatal(err)
	}
	fixture := build(cache, path)
	items := fixture.Apply([]library.Item{{ID: "item", Path: "item.mp4"}})
	if items[0].Artwork != "" || items[0].ShowArtwork != "" {
		t.Fatalf("expired artwork was applied: %#v", items[0])
	}
	if err := fixture.Prune(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expired artwork remains: %v", err)
	}
}
