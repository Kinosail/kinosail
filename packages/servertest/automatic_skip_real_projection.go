package servertest

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/playback"
)

func assertCopiedHLSFutureAbsent(t *testing.T, cache, variantURL, manifest string) string {
	t.Helper()
	address, err := url.Parse(copiedHLSReference(t, variantURL, manifest, true))
	if err != nil {
		t.Fatal(err)
	}
	quality := filepath.Base(filepath.Dir(address.Path))
	var matches []string
	//nolint:gosec // The cache root is created by this bounded test fixture.
	err = filepath.WalkDir(cache, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Name() == "index.m3u8" && filepath.Base(filepath.Dir(path)) == quality {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil || len(matches) != 1 {
		t.Fatalf("cached variant matches = %v, %v", matches, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(matches[0]), filepath.Base(address.Path))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("future-segment fixture was already materialized or unreadable: %v", err)
	}
	return matches[0]
}

func assertCopiedHLSFinalCount(t *testing.T, cached, projected string, expected int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	if !playback.WaitHLSReady(ctx, func() error {
		if playback.FinalizedVariant(filepath.Dir(cached)) {
			return nil
		}
		return os.ErrNotExist
	}) {
		t.Fatal("copied-video variant did not finalize")
	}
	actual, err := os.ReadFile(cached)
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(projected, "#EXTINF:")
	if actualCount := strings.Count(string(actual), "#EXTINF:"); count != actualCount {
		t.Fatalf("projected segments = %d; actual finalized segments = %d", count, actualCount)
	}
	if expected > 0 && count != expected {
		t.Fatalf("copied-video segments = %d; captured fixture requires %d", count, expected)
	}
	t.Logf("copied-video projected and finalized segment count: %d", count)
}
