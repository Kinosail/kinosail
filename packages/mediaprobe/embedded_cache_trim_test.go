package mediaprobe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func writeAged(t *testing.T, path string, size int, modified time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestTrimEmbeddedCacheRemovesOnlyAbandonedOCRWorkspaces(t *testing.T) {
	t.Parallel()
	cache, now := t.TempDir(), time.Now()
	root := filepath.Join(cache, "embedded-subtitles", "film", "0123456789abcdef")
	abandoned := filepath.Join(root, ".ocr-111", "2", "000000001000.png")
	active := filepath.Join(root, ".ocr-222", "2", "000000002000.png")
	subtitle := filepath.Join(root, "3.vtt")
	writeAged(t, abandoned, 100, now.Add(-48*time.Hour))
	writeAged(t, active, 100, now.Add(-time.Minute))
	writeAged(t, subtitle, 10, now.Add(-48*time.Hour))

	result, err := TrimEmbeddedCache(cache, 1<<30, now)
	if err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(root, ".ocr-111")) || !exists(active) || !exists(subtitle) {
		t.Fatalf("abandoned=%t active=%t subtitle=%t", exists(filepath.Join(root, ".ocr-111")), exists(active), exists(subtitle))
	}
	if result.Workspaces != 1 || result.FreedBytes != 100 || result.Groups != 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestTrimEmbeddedCacheEvictsLeastRecentlyUsedVersionsAndKeepsRecentOnes(t *testing.T) {
	t.Parallel()
	cache, now := t.TempDir(), time.Now()
	base := filepath.Join(cache, "embedded-subtitles")
	oldest := filepath.Join(base, "a", "1111111111111111")
	older := filepath.Join(base, "b", "2222222222222222")
	recent := filepath.Join(base, "c", "3333333333333333")
	writeAged(t, filepath.Join(oldest, "2.vtt"), 400, now.Add(-30*24*time.Hour))
	writeAged(t, filepath.Join(older, "2.vtt"), 400, now.Add(-10*24*time.Hour))
	writeAged(t, filepath.Join(recent, "2.vtt"), 400, now.Add(-time.Hour))

	result, err := TrimEmbeddedCache(cache, 900, now)
	if err != nil {
		t.Fatal(err)
	}
	if exists(oldest) || !exists(older) || !exists(recent) || exists(filepath.Join(base, "a")) {
		t.Fatalf("oldest=%t older=%t recent=%t item=%t", exists(oldest), exists(older), exists(recent), exists(filepath.Join(base, "a")))
	}
	if result.Groups != 1 || result.FreedBytes != 400 || result.Bytes != 800 {
		t.Fatalf("result = %#v", result)
	}

	result, err = TrimEmbeddedCache(cache, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if exists(older) || !exists(recent) || result.Bytes != 400 {
		t.Fatalf("tiny limit: older=%t recent=%t result=%#v", exists(older), exists(recent), result)
	}
}

func TestTrimEmbeddedCacheNeverFollowsLinksOutOfTheCache(t *testing.T) {
	t.Parallel()
	cache, outside, now := t.TempDir(), t.TempDir(), time.Now()
	writeAged(t, filepath.Join(outside, "keep.vtt"), 400, now.Add(-30*24*time.Hour))
	group := filepath.Join(cache, "embedded-subtitles", "film", "0123456789abcdef")
	writeAged(t, filepath.Join(group, "2.vtt"), 400, now.Add(-30*24*time.Hour))
	if err := os.Symlink(outside, filepath.Join(group, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cache, "embedded-subtitles", "escape")); err != nil {
		t.Fatal(err)
	}

	if _, err := TrimEmbeddedCache(cache, 1, now); err != nil {
		t.Fatal(err)
	}
	if exists(group) || !exists(filepath.Join(outside, "keep.vtt")) {
		t.Fatalf("group=%t outside=%t", exists(group), exists(filepath.Join(outside, "keep.vtt")))
	}
}

func TestTrimEmbeddedCacheWithoutCacheDirectoryIsANoOp(t *testing.T) {
	t.Parallel()
	if result, err := TrimEmbeddedCache(filepath.Join(t.TempDir(), "missing"), 1, time.Now()); err != nil || result != (EmbeddedTrim{}) {
		t.Fatalf("missing cache = %#v, %v", result, err)
	}
	if result, err := TrimEmbeddedCache("", 1, time.Now()); err != nil || result != (EmbeddedTrim{}) {
		t.Fatalf("unset cache = %#v, %v", result, err)
	}
}

func TestEmbeddedCacheHitRefreshesItsUseTime(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	media, ffmpeg := filepath.Join(root, "film.mkv"), filepath.Join(root, "ffmpeg")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nprintf '%s' '"+testWebVTT+"'\n")
	item := library.Item{ID: "film", Path: media}
	probe := embeddedTestProbe(item)
	cached, err := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: cache})
	if err != nil || cached.Path == "" {
		t.Fatalf("cached subtitle = %#v, %v", cached, err)
	}
	stale := time.Now().Add(-72 * time.Hour)
	if err = os.Chtimes(cached.Path, stale, stale); err != nil {
		t.Fatal(err)
	}
	if _, err = probe.Embedded(t.Context(), item, 3, EmbeddedOptions{CacheDir: cache}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(cached.Path)
	if err != nil || time.Since(info.ModTime()) > time.Minute {
		t.Fatalf("use time = %v, %v", info, err)
	}
}
