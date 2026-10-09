package server

import (
	"context"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

type remainingNonKeyRootedFixture struct {
	manager        *hlsManager
	item           library.Item
	recipe         hlsRecipe
	policy         string
	directory      string
	source         [32]byte
	initialization []byte
	first          []byte
}

func remainingNonKeyRootedTools(t *testing.T) (context.Context, string, string, []string) {
	t.Helper()
	if os.Getenv("KINOSAIL_COPIED_RECOVERY_MEDIA") != "1" {
		t.Skip("The pinned hosted job owns real rooted private acquisition")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("rooted private pinned FFmpeg missing")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("rooted private pinned FFprobe missing")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	t.Cleanup(cancel)
	return ctx, ffmpeg, ffprobe, remainingNonKeyCollectorSources(t, ctx, ffmpeg)
}

func remainingNonKeyRootedFixtureFor(t *testing.T, ctx context.Context, ffmpeg, ffprobe, source string, offset float64) remainingNonKeyRootedFixture {
	t.Helper()
	private := remainingNonKeyCollectorGeneratePrivate(t, ctx, ffmpeg, source, offset)
	initialization := remainingNonKeyCollectorRead(t, filepath.Join(private, "init.mp4"), 2<<20)
	first := remainingNonKeyCollectorRead(t, filepath.Join(private, "segment-00000.m4s"), 64<<20)
	return remainingNonKeyRootedFixtureFrom(t, ctx, ffmpeg, ffprobe, source, offset, initialization, first)
}

func remainingNonKeyRootedFixtureFrom(t *testing.T, ctx context.Context, ffmpeg, ffprobe, source string, offset float64, initialization, first []byte) remainingNonKeyRootedFixture {
	t.Helper()
	manager, item, _, _ := hlsLoadingFixtureContext(t, ctx)
	item.Path = source
	manager.index = &libraryIndex{Index: catalog.NewMemoryIndex(nil, true)}
	manager.index.SetRoots([]catalog.ScanRoot{{Path: filepath.Dir(source)}})
	manager.probe, manager.ffmpeg = newMediaProbe(ffprobe), ffmpeg
	recipe := hlsRecipe{mode: "remux", offset: offset}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal("rooted private fixture policy")
	}
	directory := filepath.Join(manager.cache, ".copy-private-test")
	remainingNonKeyRootedWriteStage(t, directory, initialization, first)
	return remainingNonKeyRootedFixture{
		manager:        manager,
		item:           item,
		recipe:         recipe,
		policy:         options.Cache,
		directory:      directory,
		source:         remainingNonKeyCollectorHash(t, source),
		initialization: initialization,
		first:          first,
	}
}

func remainingNonKeyRootedWriteStage(t *testing.T, directory string, initialization, first []byte) {
	t.Helper()
	if os.Mkdir(directory, 0o700) != nil ||
		os.WriteFile(filepath.Join(directory, "init.mp4"), initialization, 0o600) != nil ||
		os.WriteFile(filepath.Join(directory, "segment-00000.m4s"), first, 0o600) != nil {
		t.Fatal("rooted private owned stage")
	}
}

func (fixture remainingNonKeyRootedFixture) unchanged(t *testing.T) {
	t.Helper()
	if remainingNonKeyCollectorHash(t, fixture.item.Path) != fixture.source ||
		remainingNonKeyCollectorHash(t, filepath.Join(fixture.directory, "init.mp4")) != sha256.Sum256(fixture.initialization) ||
		remainingNonKeyCollectorHash(t, filepath.Join(fixture.directory, "segment-00000.m4s")) != sha256.Sum256(fixture.first) {
		t.Fatal("rooted private operation changed source or private asset bytes")
	}
	entries, err := os.ReadDir(fixture.directory)
	cache, cacheErr := os.ReadDir(fixture.manager.cache)
	if err != nil || cacheErr != nil || len(entries) != 2 || len(cache) != 1 ||
		cache[0].Name() != filepath.Base(fixture.directory) || len(fixture.manager.jobs) != 0 {
		t.Fatal("rooted private operation published metadata, media or readiness")
	}
}

func remainingNonKeyRootedFDCount(t *testing.T, directory string) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal("rooted private descriptor witness unavailable")
	}
	count := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err == nil && (target == directory || strings.HasPrefix(target, directory+string(os.PathSeparator))) {
			count++
		}
	}
	return count
}

func remainingNonKeyRootedLeaseReleased(t *testing.T, manager *hlsManager) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, release, err := manager.copiedHLSClockAdmission(ctx)
	if err != nil {
		t.Fatal("rooted private operation retained the metadata lease after return")
	}
	release()
}
