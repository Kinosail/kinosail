package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/servertest"
)

// Canonical before/after checks cannot witness a replacement restored while a
// child opens its input. This real fixture keeps that replacement in place for
// actual FFprobe and independently witnesses which bytes its argument resolves.
func TestRemainingNonKeyActualSourceClockRetainsOpenedSource(t *testing.T) {
	if os.Getenv("KINOSAIL_COPIED_RECOVERY_MEDIA") != "1" {
		t.Skip("The pinned hosted job owns the real retained-source operation")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("retained-source pinned FFmpeg missing")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("retained-source pinned FFprobe missing")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	sources := remainingNonKeyCollectorSources(t, ctx, ffmpeg)
	first, edit := remainingNonKeyCollectorPrivateAudio(t, ctx, ffmpeg, ffprobe, sources[0], 12.5)
	manager, item, _, _ := hlsLoadingFixture(t)
	item.Path = sources[0]
	manager.index = &libraryIndex{Index: catalog.NewMemoryIndex(nil, true)}
	manager.index.SetRoots([]catalog.ScanRoot{{Path: filepath.Dir(item.Path)}})
	marker := filepath.Join(t.TempDir(), "source-open")
	manager.probe.executable = remainingNonKeyRetainedSourceProbe(t, ffprobe, sources[0], sources[1], marker)
	manager.ffmpeg = ffmpeg
	recipe := hlsRecipe{mode: "remux", offset: 12.5}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal("retained-source policy")
	}
	before, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal("retained-source original inode")
	}
	original, replacement := remainingNonKeyCollectorHash(t, sources[0]), remainingNonKeyCollectorHash(t, sources[1])
	started := time.Now()
	proof, err := manager.measureCopiedHLSSourceAudio(ctx, item, recipe, options.Cache, first, 12_500_000, edit)
	elapsed := time.Since(started)
	remainingNonKeyRetainedSourceWitness(t, manager, item.Path, marker, before, original, replacement)
	remainingNonKeyRetainedSourceProof(t, proof, err, first, sources[0], elapsed)
}

func remainingNonKeyRetainedSourceProof(t *testing.T, proof *copiedHLSAudioProof, err error, first [32]byte, source string, elapsed time.Duration) {
	t.Helper()
	if err != nil || proof == nil || elapsed > 2*time.Second {
		t.Fatal("nonkey original opened source did not produce its qualified clock")
	}
	_, expected := remainingNonKeyCollectorExpected(t, source, 12.5)
	remainingNonKeyCollectorCorrespondence(t, proof, expected)
	if proof.FirstPacket != first || proof.SourceClock == [32]byte{} {
		t.Fatal("nonkey retained-source private packet binding")
	}
	t.Logf("nonkey retained-source elapsed_ns=%d canonical_inode_restored=true argument_original=true source_clock=%x", elapsed.Nanoseconds(), proof.SourceClock)
}

func remainingNonKeyRetainedSourceProbe(t *testing.T, ffprobe, source, replacement, marker string) string {
	t.Helper()
	before, err := os.Stat(source)
	if err != nil {
		t.Fatal("retained-source cleanup identity")
	}
	original := remainingNonKeyCollectorHash(t, source)
	t.Cleanup(func() { remainingNonKeyRestoreOwnedSource(t, source, source+".retained", before, original) })
	sourceQuoted, backup := copiedRecoveryQuote(source), copiedRecoveryQuote(source+".retained")
	body := "#!/bin/sh\nset -eu\n" +
		"restore_source() { if [ -f " + backup + " ]; then mv -f " + backup + " " + sourceQuoted + "; fi; }\ntrap restore_source EXIT HUP INT TERM\n" +
		"mv " + sourceQuoted + " " + backup + "\ncp " + copiedRecoveryQuote(replacement) + " " + sourceQuoted + "\n" +
		"printf '%s\\n' $$ > " + copiedRecoveryQuote(marker) + "\n" +
		"sha256sum " + sourceQuoted + " >> " + copiedRecoveryQuote(marker) + "\n" +
		"for argument do input=$argument; done\nsha256sum \"$input\" >> " + copiedRecoveryQuote(marker) + "\n" +
		copiedRecoveryQuote(ffprobe) + " \"$@\"\n"
	executable := filepath.Join(t.TempDir(), "retained-source-probe")
	servertest.WriteExecutable(t, executable, body)
	return executable
}

func remainingNonKeyRetainedSourceWitness(t *testing.T, manager *hlsManager, source, marker string, before os.FileInfo, original, replacement [32]byte) {
	t.Helper()
	rows := remainingNonKeyRetainedSourceRows(t, marker)
	copiedRecoveryAssertStopped(t, []byte(rows[0]))
	remainingNonKeyAssertRestoredSource(t, source, before, original)
	remainingNonKeyCollectorCacheEmpty(t, manager)
	substitution, argument := strings.Fields(rows[1])[0], strings.Fields(rows[2])[0]
	t.Logf("nonkey actual-retained-source replacement_sha=%s argument_sha=%s original_sha=%x restored_same_inode=true source_unchanged=true", substitution, argument, original)
	if original == replacement || substitution != fmt.Sprintf("%x", replacement) {
		t.Fatal("nonkey retained-source replacement never existed at actual probe open")
	}
	if argument != fmt.Sprintf("%x", original) {
		t.Fatal("nonkey source-clock process consumed a replacement pathname")
	}
}

func remainingNonKeyRetainedSourceRows(t *testing.T, marker string) []string {
	t.Helper()
	data := remainingNonKeyCollectorRead(t, marker, 2048)
	rows := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(rows) != 3 || len(strings.Fields(rows[1])) < 1 || len(strings.Fields(rows[2])) < 1 {
		t.Fatal("nonkey retained-source actual-open witnesses missing")
	}
	return rows
}

func remainingNonKeyAssertRestoredSource(t *testing.T, source string, before os.FileInfo, original [32]byte) {
	t.Helper()
	after, err := os.Stat(source)
	if err != nil || !sameCopiedHLSFile(before, after) || remainingNonKeyCollectorHash(t, source) != original {
		t.Fatal("nonkey retained-source fixture did not restore the exact original inode and bytes")
	}
}

// This cleanup owns only generated test media and runs after joined probes,
// before the source temporary directory's separately registered cleanup.
func remainingNonKeyRestoreOwnedSource(t *testing.T, source, backup string, before os.FileInfo, original [32]byte) {
	t.Helper()
	retained, err := os.Lstat(backup)
	if err == nil {
		if !sameCopiedHLSFile(before, retained) || remainingNonKeyCollectorHash(t, backup) != original {
			t.Fatal("nonkey retained-source cleanup found a different backup")
		}
		if os.Rename(backup, source) != nil {
			t.Fatal("nonkey retained-source owned restoration failed")
		}
	} else if !os.IsNotExist(err) {
		t.Fatal("nonkey retained-source owned backup observation failed")
	}
	remainingNonKeyAssertRestoredSource(t, source, before, original)
}
