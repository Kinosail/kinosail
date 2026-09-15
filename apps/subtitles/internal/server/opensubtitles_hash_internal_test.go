package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSubtitlesHashUsesSizeAndBoundaryBlocks(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "movie.mkv")
	data := make([]byte, openSubtitlesHashBlock*2)
	for index := range data {
		data[index] = byte(index % 251)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	hash, err := openSubtitlesHash(path)
	if err != nil || hash != "bf7b30eba75ef876" {
		t.Fatalf("hash = %q, error = %v", hash, err)
	}
	data[len(data)-1]++
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := openSubtitlesHash(path)
	if err != nil || changed == hash {
		t.Fatalf("changed hash = %q, error = %v", changed, err)
	}
}

func TestOpenSubtitlesHashRejectsSmallAndMissingFiles(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	small := filepath.Join(directory, "small.mkv")
	if err := os.WriteFile(small, make([]byte, openSubtitlesHashBlock), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{small, filepath.Join(directory, "missing.mkv")} {
		if hash, err := openSubtitlesHash(path); err == nil {
			t.Fatalf("hash for %q = %q", path, hash)
		}
	}
}
