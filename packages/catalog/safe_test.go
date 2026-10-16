package catalog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSafeFixture(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func symlinkSafeFixture(t *testing.T, target, link string) string {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	return link
}

func TestIndexSafeRejectsEscapesFromLibraryRoots(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root, cache, outside := filepath.Join(base, "media"), filepath.Join(base, "cache"), filepath.Join(base, "outside")
	inside := writeSafeFixture(t, filepath.Join(root, "Movie.mp4"))
	cached := writeSafeFixture(t, filepath.Join(cache, "segment.ts"))
	secret := writeSafeFixture(t, filepath.Join(outside, "Secret.mp4"))
	sibling := writeSafeFixture(t, filepath.Join(base, "media-evil", "Movie.mp4"))
	fileLink := symlinkSafeFixture(t, secret, filepath.Join(root, "Linked.mp4"))
	directoryLink := symlinkSafeFixture(t, outside, filepath.Join(root, "Linked"))
	internalLink := symlinkSafeFixture(t, inside, filepath.Join(root, "Alias.mp4"))
	index := NewIndex(t.Context(), []ScanRoot{{Path: root, Namespace: "Movies"}}, cache, time.Hour, nil)
	for path, want := range map[string]bool{
		inside: true, cached: true, internalLink: true,
		secret: false, sibling: false, fileLink: false,
		filepath.Join(directoryLink, "Secret.mp4"):         false,
		filepath.Join(root, "..", "outside", "Secret.mp4"): false,
		filepath.Join(root, "Missing.mp4"):                 false,
		root:                                               true,
		base:                                               false,
		"":                                                 false,
	} {
		for range 2 {
			if got := index.Safe(path); got != want {
				t.Errorf("Safe(%q) = %t, want %t", path, got, want)
			}
		}
	}
}

func TestIndexSafeFollowsRootChanges(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	first, second, late := filepath.Join(base, "first"), filepath.Join(base, "second"), filepath.Join(base, "late")
	firstMedia := writeSafeFixture(t, filepath.Join(first, "Movie.mp4"))
	secondMedia := writeSafeFixture(t, filepath.Join(second, "Movie.mp4"))
	link := symlinkSafeFixture(t, first, filepath.Join(base, "library"))
	index := NewIndex(t.Context(), []ScanRoot{{Path: link, Namespace: "Movies"}}, "", time.Hour, nil)
	if !index.Safe(firstMedia) || !index.Safe(filepath.Join(link, "Movie.mp4")) || index.Safe(secondMedia) {
		t.Fatal("symlinked root did not resolve to its target")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	symlinkSafeFixture(t, second, link)
	if err := index.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	if index.Safe(firstMedia) || !index.Safe(secondMedia) {
		t.Fatal("retargeted root kept the previous target after a scan")
	}
	index.SetRoots([]ScanRoot{{Path: first, Namespace: "Movies"}, {Path: late, Namespace: "Late"}})
	if !index.Safe(firstMedia) || index.Safe(secondMedia) {
		t.Fatal("changed roots kept the previous root")
	}
	lateMedia := writeSafeFixture(t, filepath.Join(late, "Movie.mp4"))
	if !index.Safe(lateMedia) {
		t.Fatal("root created after configuration was not allowed")
	}
	index.SetRoots(nil)
	if index.Safe(firstMedia) || index.Safe(lateMedia) {
		t.Fatal("removed roots stayed allowed")
	}
}

func BenchmarkIndexSafe(b *testing.B) {
	base := b.TempDir()
	roots := []ScanRoot{{Path: filepath.Join(base, "movies"), Namespace: "Movies"}, {Path: filepath.Join(base, "shows"), Namespace: "Shows"}, {Path: filepath.Join(base, "music"), Namespace: "Music"}}
	for _, root := range roots {
		if err := os.MkdirAll(root.Path, 0o700); err != nil {
			b.Fatal(err)
		}
	}
	media := filepath.Join(roots[2].Path, "Album", "Track.mp3")
	if err := os.MkdirAll(filepath.Dir(media), 0o700); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(media, []byte("media"), 0o600); err != nil {
		b.Fatal(err)
	}
	index := NewMemoryIndex(nil, true)
	index.SetRoots(roots)
	b.ReportAllocs()
	for b.Loop() {
		if !index.Safe(media) {
			b.Fatal("media outside roots")
		}
	}
}
