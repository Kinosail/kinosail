//go:build linux

package server

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/workload"
)

func remainingNonKeyRootedOperationFiles(t *testing.T, fixture remainingNonKeyRootedFixture, name string, originalSource, originalFirst os.FileInfo) {
	t.Helper()
	if name == "replaced-source" {
		remainingNonKeyRootedSourceReplaced(t, fixture, originalSource)
	}
	if name == "private-restored-mtime" {
		remainingNonKeyRootedFirstRewritten(t, fixture, originalFirst)
	} else {
		fixture.unchanged(t)
	}
	entries, err := os.ReadDir(fixture.directory)
	if err != nil || len(entries) != 2 || len(fixture.manager.jobs) != 0 ||
		remainingNonKeyCollectorHash(t, fixture.item.Path) != fixture.source {
		t.Fatal("nonkey private operation rejection published metadata or changed source bytes")
	}
}

func TestRemainingNonKeyRootedPrivateRejectsBeforeProbe(t *testing.T) {
	ctx, ffmpeg, ffprobe, sources := remainingNonKeyRootedTools(t)
	base := remainingNonKeyRootedFixtureFor(t, ctx, ffmpeg, ffprobe, sources[0], 12.5)
	for _, damage := range []string{"malformed-init", "malformed-first", "changed-policy"} {
		t.Run(damage, func(t *testing.T) {
			fixture := remainingNonKeyRootedFixtureFrom(t, ctx, ffmpeg, ffprobe, sources[0], 12.5, base.initialization, base.first)
			manager := fixture.manager
			release, err := manager.workloads.Acquire(ctx, workload.Playback)
			if err != nil {
				t.Fatal("rooted private reject owner reservation")
			}
			defer release()
			marker := filepath.Join(t.TempDir(), "owned-pids")
			manager.probe.executable = remainingNonKeyRootedProcess(t, ffprobe, marker, "")
			manager.ffmpeg = remainingNonKeyRootedProcess(t, ffmpeg, marker, "")
			policy := remainingNonKeyRootedInvalidInput(t, fixture, damage)
			proof, facts, err := manager.measureCopiedHLSPrivateSourceAudio(ctx, fixture.item, fixture.recipe, policy, fixture.directory)
			if err == nil || proof != nil || facts != nil {
				t.Fatal("nonkey malformed or unbound private input acquired source identity")
			}
			if _, err := os.Lstat(marker); !os.IsNotExist(err) {
				t.Fatal("nonkey invalid private input launched a source process")
			}
			remainingNonKeyCollectorReserved(t, manager)
			remainingNonKeyRootedLeaseReleased(t, manager)
			if remainingNonKeyRootedFDCount(t, manager.cache) != 0 {
				t.Fatal("nonkey invalid private input leaked a descriptor")
			}
			t.Logf("nonkey rooted-private-input case=%s source_processes=0 descriptors_released=true", damage)
		})
	}
}

func remainingNonKeyRootedInvalidInput(t *testing.T, fixture remainingNonKeyRootedFixture, damage string) string {
	t.Helper()
	if damage == "changed-policy" {
		return fixture.policy + ":changed"
	}
	name := "init.mp4"
	if damage == "malformed-first" {
		name = "segment-00000.m4s"
	}
	if os.WriteFile(filepath.Join(fixture.directory, name), []byte("not media"), 0o600) != nil {
		t.Fatal("rooted private malformed fixture")
	}
	return fixture.policy
}

func remainingNonKeyRootedSourceReplaced(t *testing.T, fixture remainingNonKeyRootedFixture, originalSource os.FileInfo) {
	t.Helper()
	current, err := os.Stat(fixture.item.Path)
	if err != nil || os.SameFile(originalSource, current) ||
		remainingNonKeyCollectorHash(t, fixture.item.Path) != fixture.source {
		t.Fatal("nonkey actual source replacement did not retain bytes on another inode")
	}
}

func remainingNonKeyRootedFirstRewritten(t *testing.T, fixture remainingNonKeyRootedFixture, originalFirst os.FileInfo) {
	t.Helper()
	path := filepath.Join(fixture.directory, "segment-00000.m4s")
	current, err := os.Stat(path)
	if err != nil || !sameCopiedHLSFile(originalFirst, current) ||
		bytes.Equal(fixture.first, remainingNonKeyCollectorRead(t, path, 64<<20)) {
		t.Fatal("nonkey actual private rewrite did not retain inode, length and mtime")
	}
}
