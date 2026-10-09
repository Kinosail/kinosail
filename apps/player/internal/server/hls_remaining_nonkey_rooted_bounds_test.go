package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Opening only the directory postpones file validation until after unrelated
// work. A partial-open failure must settle every descriptor before returning.
func TestRemainingNonKeyRootedPrivateFileBounds(t *testing.T) {
	ctx, ffmpeg, ffprobe, sources := remainingNonKeyRootedTools(t)
	base := remainingNonKeyRootedFixtureFor(t, ctx, ffmpeg, ffprobe, sources[0], 12.5)
	for _, asset := range []string{"init.mp4", "segment-00000.m4s"} {
		for _, damage := range []string{"missing", "empty", "directory", "symlink", "fifo", "oversized"} {
			t.Run(asset+"-"+damage, func(t *testing.T) {
				fixture := remainingNonKeyRootedFixtureFrom(t, ctx, ffmpeg, ffprobe, sources[0], 12.5, base.initialization, base.first)
				remainingNonKeyRootedFileBound(t, ctx, fixture, asset, damage)
			})
		}
	}
}

func remainingNonKeyRootedFileBound(t *testing.T, ctx context.Context, fixture remainingNonKeyRootedFixture, asset, damage string) {
	t.Helper()
	name := filepath.Join(fixture.directory, asset)
	if os.Remove(name) != nil {
		t.Fatal("rooted private invalid file fixture removal")
	}
	var err error
	switch damage {
	case "empty":
		err = os.WriteFile(name, nil, 0o600)
	case "directory":
		err = os.Mkdir(name, 0o700)
	case "symlink":
		err = os.Symlink(fixture.item.Path, name)
	case "fifo":
		err = syscall.Mkfifo(name, 0o600)
	case "oversized":
		remainingNonKeyRootedOversize(t, name, asset)
	}
	if err != nil {
		t.Fatal("rooted private invalid file fixture construction")
	}
	before := remainingNonKeyRootedFDCount(t, fixture.manager.cache)
	value, openErr := fixture.manager.openCopiedHLSPrivateAssets(ctx, fixture.directory)
	if value != nil {
		value.close()
	}
	if openErr == nil || value != nil {
		t.Fatal("nonkey rooted acquisition opened missing, irregular or oversized private files")
	}
	if remainingNonKeyRootedFDCount(t, fixture.manager.cache) != before {
		t.Fatal("nonkey partial private acquisition leaked its retained descriptors")
	}
	if remainingNonKeyCollectorHash(t, fixture.item.Path) != fixture.source {
		t.Fatal("nonkey invalid private file acquisition changed source")
	}
	t.Logf("nonkey rooted-private-bound asset=%s case=%s descriptors_released=true", asset, damage)
}

func remainingNonKeyRootedOversize(t *testing.T, name, asset string) {
	t.Helper()
	limit := int64(64 << 20)
	if asset == "init.mp4" {
		limit = 2 << 20
	}
	file, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal("rooted private oversize construction")
	}
	truncateErr := file.Truncate(limit + 1)
	closeErr := file.Close()
	if truncateErr != nil || closeErr != nil {
		t.Fatal("rooted private oversize bound")
	}
}

func TestRemainingNonKeyRootedPrivateDirectoryBounds(t *testing.T) {
	ctx, ffmpeg, ffprobe, sources := remainingNonKeyRootedTools(t)
	base := remainingNonKeyRootedFixtureFor(t, ctx, ffmpeg, ffprobe, sources[0], 12.5)
	for _, damage := range []string{"outside-cache", "stage-symlink", "cache-symlink"} {
		t.Run(damage, func(t *testing.T) {
			fixture := remainingNonKeyRootedFixtureFrom(t, ctx, ffmpeg, ffprobe, sources[0], 12.5, base.initialization, base.first)
			directory := remainingNonKeyRootedInvalidDirectory(t, fixture, damage)
			assets, err := fixture.manager.openCopiedHLSPrivateAssets(ctx, directory)
			if assets != nil {
				assets.close()
			}
			if err == nil || assets != nil {
				t.Fatal("nonkey rooted acquisition opened an unbound private directory")
			}
		})
	}
}

func remainingNonKeyRootedInvalidDirectory(t *testing.T, fixture remainingNonKeyRootedFixture, damage string) string {
	t.Helper()
	switch damage {
	case "outside-cache":
		directory := filepath.Join(t.TempDir(), "private")
		remainingNonKeyRootedWriteStage(t, directory, fixture.initialization, fixture.first)
		return directory
	case "stage-symlink":
		directory := filepath.Join(fixture.manager.cache, "private-link")
		if os.Symlink(filepath.Base(fixture.directory), directory) != nil {
			t.Fatal("rooted private stage symlink")
		}
		return directory
	case "cache-symlink":
		if os.Rename(fixture.manager.cache, fixture.manager.cache+".held") != nil ||
			os.Symlink(fixture.manager.cache+".held", fixture.manager.cache) != nil {
			t.Fatal("rooted private cache symlink")
		}
	}
	return fixture.directory
}

func TestRemainingNonKeyRootedPrivateRetainsBothFiles(t *testing.T) {
	ctx, ffmpeg, ffprobe, sources := remainingNonKeyRootedTools(t)
	fixture := remainingNonKeyRootedFixtureFor(t, ctx, ffmpeg, ffprobe, sources[0], 12.5)
	assets, err := fixture.manager.openCopiedHLSPrivateAssets(ctx, fixture.directory)
	if assets != nil {
		t.Cleanup(assets.close)
	}
	if err != nil || assets == nil {
		t.Fatal("nonkey valid rooted private files were not opened")
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal("rooted private opened file witness")
	}
	found := map[string]bool{}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err == nil && strings.HasPrefix(target, fixture.directory+string(os.PathSeparator)) {
			found[filepath.Base(target)] = true
		}
	}
	if !found["init.mp4"] || !found["segment-00000.m4s"] {
		t.Fatal("nonkey private acquisition did not retain both file descriptors before reading")
	}
	assets.close()
	if remainingNonKeyRootedFDCount(t, fixture.manager.cache) != 0 {
		t.Fatal("nonkey rooted private close retained a descriptor")
	}
}
