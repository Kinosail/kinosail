package metadata

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeBackdrop(t *testing.T, path string, width int, modified time.Time) {
	t.Helper()
	var data bytes.Buffer
	if err := jpeg.Encode(&data, image.NewGray(image.Rect(0, 0, width, 10)), nil); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyProviderBackdropFollowsReplacedArtwork(t *testing.T) {
	cache := t.TempDir()
	path := filepath.Join(cache, "metadata", "movie-backdrop.jpg")
	now := time.Now()
	writeBackdrop(t, path, 780, now.Add(-time.Hour))
	for range 2 {
		if !LegacyProviderBackdrop(defaultImageURL, cache, path) {
			t.Fatal("780-pixel backdrop was not recognized as legacy")
		}
	}
	writeBackdrop(t, path, 1280, now)
	if LegacyProviderBackdrop(defaultImageURL, cache, path) {
		t.Fatal("replaced original-size backdrop was still reported as legacy")
	}
	if LegacyProviderBackdrop("https://example.com", cache, path) || LegacyProviderBackdrop(defaultImageURL, cache, filepath.Join(cache, "missing.jpg")) {
		t.Fatal("custom providers or missing artwork were reported as legacy")
	}
}
