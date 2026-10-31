package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanNamesTheShowAboveItsSeasonFolderInsideACategoryFolder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"Shows/Signal House/Season 01/Signal House - S01E01.mkv":    "Signal House",
		"Shows/Signal House/Season 01/Signal House - S01E02.mkv":    "Signal House",
		"Shows/The Quiet Coast/Season 2/The Quiet Coast S02E01.mkv": "The Quiet Coast",
		"Shows/The Quiet Coast/Specials/The Quiet Coast S00E01.mkv": "The Quiet Coast",
		"Tumble Leaf (2014)/Season 01/Tumble Leaf - S01E01.mkv":     "Tumble Leaf (2014)",
		"Flat Show/Flat Show - S01E01.mkv":                          "Flat Show",
	}
	for path := range files {
		path = filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "Shows", "Signal House", "poster.jpg"), []byte("jpg"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := ScanContext(t.Context(), root, "media")
	if err != nil || len(items) != len(files) {
		t.Fatalf("scanned = %d items, error = %v", len(items), err)
	}
	for _, item := range items {
		relative, _ := filepath.Rel(root, item.Path)
		if want := files[filepath.ToSlash(relative)]; item.Show != want {
			t.Errorf("%s: show = %q, want %q", relative, item.Show, want)
		}
		if item.Show == "Signal House" && item.ShowArtwork == "" {
			t.Errorf("%s: show poster was not found in the show folder", relative)
		}
	}
	_, shows := Organize(items)
	if len(shows) != 4 {
		t.Fatalf("shows = %d, want 4", len(shows))
	}
}
