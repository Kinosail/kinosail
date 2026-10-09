//go:build linux

package server

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Caller bytes cannot reveal a file swap or a content rewrite concealed by
// restored mtime. Each control first proves that the actual private pair parses.
func TestRemainingNonKeyRootedPrivateAssetChanges(t *testing.T) {
	ctx, ffmpeg, ffprobe, sources := remainingNonKeyRootedTools(t)
	base := remainingNonKeyRootedFixtureFor(t, ctx, ffmpeg, ffprobe, sources[0], 12.5)
	initialization := remainingNonKeyPrivateAudioPad(t, base.initialization, len(base.initialization)+32)
	first := remainingNonKeyPrivateAudioPad(t, base.first, len(base.first)+32)
	for _, name := range []string{
		"replace-init", "replace-first", "replace-stage", "replace-cache",
		"truncate-init", "grow-first", "mutate-init-restored-mtime", "mutate-first-restored-mtime",
		"cancel-after-open", "closed-root",
	} {
		t.Run(name, func(t *testing.T) {
			fixture := remainingNonKeyRootedFixtureFrom(t, ctx, ffmpeg, ffprobe, sources[0], 12.5, initialization, first)
			remainingNonKeyRootedAssetChange(t, ctx, fixture, name)
		})
	}
}

func remainingNonKeyRootedAssetChange(t *testing.T, ctx context.Context, fixture remainingNonKeyRootedFixture, name string) {
	t.Helper()
	before := remainingNonKeyRootedFDCount(t, fixture.manager.cache)
	assets, facts := remainingNonKeyRootedOpenValid(t, ctx, fixture)
	request, cancel := context.WithCancel(ctx)
	defer cancel()
	remainingNonKeyRootedAssetMutation(t, fixture, assets, name, cancel)
	remainingNonKeyRootedChangedMetadataValid(t, ctx, fixture, name, facts)
	if name == "mutate-init-restored-mtime" || name == "mutate-first-restored-mtime" {
		if assets.current(request, facts) {
			t.Fatal("nonkey private content rewrite with restored metadata acquired identity")
		}
	} else if value, err := assets.audio(request); err == nil || value != nil {
		t.Fatal("nonkey changed rooted private generation acquired identity")
	}
	assets.close()
	assets.close()
	if remainingNonKeyRootedFDCount(t, fixture.manager.cache) != before ||
		remainingNonKeyRootedFDCount(t, fixture.manager.cache+".held") != 0 {
		t.Fatal("nonkey rejected private assets retained descriptors after explicit close")
	}
	if remainingNonKeyCollectorHash(t, fixture.item.Path) != fixture.source {
		t.Fatal("nonkey private asset rejection changed source media")
	}
	t.Logf("nonkey rooted-private-rejection case=%s descriptors_released=true source_unchanged=true", name)
}

func remainingNonKeyRootedAssetMutation(t *testing.T, fixture remainingNonKeyRootedFixture, assets *copiedHLSPrivateAssets, name string, cancel context.CancelFunc) {
	t.Helper()
	switch name {
	case "replace-init":
		remainingNonKeyRootedReplaceFile(t, filepath.Join(fixture.directory, "init.mp4"))
	case "replace-first":
		remainingNonKeyRootedReplaceFile(t, filepath.Join(fixture.directory, "segment-00000.m4s"))
	case "replace-stage", "replace-cache":
		remainingNonKeyRootedReplaceDirectory(t, fixture, name)
	case "truncate-init":
		remainingNonKeyRootedTruncate(t, filepath.Join(fixture.directory, "init.mp4"), int64(len(fixture.initialization)-32))
	case "grow-first":
		remainingNonKeyRootedGrow(t, filepath.Join(fixture.directory, "segment-00000.m4s"))
	case "mutate-init-restored-mtime":
		remainingNonKeyRootedRewrite(t, filepath.Join(fixture.directory, "init.mp4"))
	case "mutate-first-restored-mtime":
		remainingNonKeyRootedRewrite(t, filepath.Join(fixture.directory, "segment-00000.m4s"))
	case "cancel-after-open":
		cancel()
	case "closed-root":
		assets.close()
	}
}

func remainingNonKeyRootedReplaceFile(t *testing.T, name string) {
	t.Helper()
	before, err := os.Stat(name)
	if err != nil {
		t.Fatal("rooted private replacement original identity")
	}
	data := remainingNonKeyCollectorRead(t, name, 64<<20)
	replacement := name + ".replacement"
	if os.WriteFile(replacement, data, 0o600) != nil ||
		os.Chtimes(replacement, before.ModTime(), before.ModTime()) != nil ||
		os.Rename(replacement, name) != nil {
		t.Fatal("rooted private same-byte replacement")
	}
	after, err := os.Stat(name)
	if err != nil || os.SameFile(before, after) || before.Size() != after.Size() ||
		!before.ModTime().Equal(after.ModTime()) || !bytes.Equal(data, remainingNonKeyCollectorRead(t, name, 64<<20)) {
		t.Fatal("rooted private replacement witness")
	}
}

func remainingNonKeyRootedGrow(t *testing.T, name string) {
	t.Helper()
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal("rooted private growth open")
	}
	_, writeErr := file.Write([]byte{0, 0, 0, 8, 'f', 'r', 'e', 'e'})
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal("rooted private growth write")
	}
}

func remainingNonKeyRootedRewrite(t *testing.T, name string) {
	t.Helper()
	before, err := os.Stat(name)
	if err != nil {
		t.Fatal("rooted private rewrite original identity")
	}
	original := remainingNonKeyCollectorRead(t, name, 64<<20)
	changed := bytes.Clone(original)
	changed[len(changed)-1] ^= 1 // Valid free-box payload, not codec or packet metadata.
	if os.WriteFile(name, changed, 0o600) != nil ||
		os.Chtimes(name, before.ModTime(), before.ModTime()) != nil {
		t.Fatal("rooted private same-length content rewrite")
	}
	after, err := os.Stat(name)
	if err != nil || !sameCopiedHLSFile(before, after) || bytes.Equal(original, remainingNonKeyCollectorRead(t, name, 64<<20)) {
		t.Fatal("rooted private restored-mtime rewrite witness")
	}
}

// Cancellation is tested independently of the actual source tools so it must
// prevent opening any root or descriptor, rather than merely stop a later probe.
func TestRemainingNonKeyRootedPrivateCanceledAcquisition(t *testing.T) {
	ctx, ffmpeg, ffprobe, sources := remainingNonKeyRootedTools(t)
	fixture := remainingNonKeyRootedFixtureFor(t, ctx, ffmpeg, ffprobe, sources[0], 12.5)
	request, cancel := context.WithCancel(ctx)
	cancel()
	started := time.Now()
	assets, err := fixture.manager.openCopiedHLSPrivateAssets(request, fixture.directory)
	if assets != nil {
		assets.close()
	}
	if err == nil || assets != nil || time.Since(started) > 2*time.Second {
		t.Fatal("nonkey canceled rooted acquisition opened private assets")
	}
	fixture.unchanged(t)
}

func remainingNonKeyRootedChangedMetadataValid(t *testing.T, ctx context.Context, fixture remainingNonKeyRootedFixture, name string, before *copiedHLSPrivateAudioFacts) {
	t.Helper()
	switch name {
	case "truncate-init", "grow-first", "mutate-init-restored-mtime", "mutate-first-restored-mtime":
		initialization := remainingNonKeyCollectorRead(t, filepath.Join(fixture.directory, "init.mp4"), 2<<20)
		first := remainingNonKeyCollectorRead(t, filepath.Join(fixture.directory, "segment-00000.m4s"), 64<<20)
		facts, err := parseCopiedHLSPrivateAudio(ctx, initialization, first)
		if err != nil || facts == nil || facts.FirstPacket != before.FirstPacket ||
			facts.OriginalMediaTime != before.OriginalMediaTime {
			t.Fatal("rooted private mutation changed packet or codec metadata instead of only asset identity")
		}
	}
}

func remainingNonKeyRootedReplaceDirectory(t *testing.T, fixture remainingNonKeyRootedFixture, name string) {
	t.Helper()
	directory := fixture.directory
	if name == "replace-cache" {
		directory = fixture.manager.cache
	}
	if os.Rename(directory, directory+".held") != nil {
		t.Fatal("rooted private directory replacement")
	}
	if name == "replace-cache" && os.Mkdir(fixture.manager.cache, 0o700) != nil {
		t.Fatal("rooted private replacement cache")
	}
	remainingNonKeyRootedWriteStage(t, fixture.directory, fixture.initialization, fixture.first)
}

func remainingNonKeyRootedTruncate(t *testing.T, name string, size int64) {
	t.Helper()
	if os.Truncate(name, size) != nil {
		t.Fatal("rooted private truncation")
	}
}

func remainingNonKeyRootedOpenValid(t *testing.T, ctx context.Context, fixture remainingNonKeyRootedFixture) (*copiedHLSPrivateAssets, *copiedHLSPrivateAudioFacts) {
	t.Helper()
	assets, err := fixture.manager.openCopiedHLSPrivateAssets(ctx, fixture.directory)
	if assets != nil {
		t.Cleanup(assets.close)
	}
	if err != nil || assets == nil {
		t.Fatal("nonkey valid private assets could not be retained before mutation")
	}
	facts, err := assets.audio(ctx)
	if err != nil || facts == nil || !assets.current(ctx, facts) {
		t.Fatal("nonkey unmodified rooted private pair failed its positive control")
	}
	return assets, facts
}
