package metadata

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ArtworkMaxAge = 6 * 30 * 24 * time.Hour

// CurrentArtwork returns a regular, unexpired image inside the metadata cache.
func CurrentArtwork(cache, path string) string {
	if path == "" || cache == "" {
		return ""
	}
	root := filepath.Join(cache, "metadata")
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || time.Since(info.ModTime()) > ArtworkMaxAge {
		return ""
	}
	return path
}

// PruneExpiredArtwork removes expired JPEGs without changing other cache files.
func PruneExpiredArtwork(cache string) error {
	return pruneExpiredArtwork(cache, os.ReadDir, os.Remove)
}

func pruneExpiredArtwork(cache string, readDir func(string) ([]os.DirEntry, error), remove func(string) error) error {
	if cache == "" {
		return nil
	}
	root := filepath.Join(cache, "metadata")
	entries, err := readDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := pruneArtworkEntry(root, entry, remove); err != nil {
			return err
		}
	}
	return nil
}

func pruneArtworkEntry(root string, entry os.DirEntry, remove func(string) error) error {
	if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".jpg" {
		return nil
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if time.Since(info.ModTime()) > ArtworkMaxAge {
		if err := remove(filepath.Join(root, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
