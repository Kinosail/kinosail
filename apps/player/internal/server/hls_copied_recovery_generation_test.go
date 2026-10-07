package server

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// Real filesystem/probe processes inject races unavailable to ordinary media E2E.
func copiedRecoveryFixture(t *testing.T) (*hlsManager, library.Item, hlsRecipe, string, string, *copiedHLSTimeline) {
	t.Helper()
	manager, item, _, _ := hlsLoadingFixture(t)
	recipe := hlsRecipe{mode: "remux"}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))
	writeHLSLoadingFile(t, filepath.Join(directory, ".source"), options.Cache)
	writeHLSLoadingFile(t, filepath.Join(directory, ".seekable"), options.Cache)
	writeHLSLoadingFile(t, filepath.Join(directory, "index.m3u8"), "#EXTM3U\n#KINOSAIL-TRANSCODER:"+options.Cache+"\n#EXT-X-STREAM-INF:BANDWIDTH=1000000\n360p/index.m3u8\n")
	writeHLSLoadingFile(t, filepath.Join(directory, "360p/index.m3u8"), copiedRecoveryManifest)
	writeHLSLoadingFile(t, filepath.Join(directory, "360p/init.mp4"), "initialization")
	writeHLSLoadingFile(t, filepath.Join(directory, "360p/segment-00000.m4s"), "first fragment")
	writeHLSLoadingFile(t, filepath.Join(directory, "360p/segment-00001.m4s"), "last fragment")
	timeline := &copiedHLSTimeline{Policy: options.Cache, Strategy: "h264-idr-keys-1", Numerator: 1, Denominator: 1000, TimeBase: 0.001, Keys: []copiedHLSKey{{PTS: 0, DTS: 0}, {PTS: 2000, DTS: 2000}}, End: 4}
	if err := manager.writeCopiedHLSTimeline(directory, timeline); err != nil {
		t.Fatal(err)
	}
	return manager, item, recipe, directory, options.Cache, timeline
}

func TestCopiedRecoveryClockRejectsReplacedRenditionGeneration(t *testing.T) {
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	rendering := filepath.Join(directory, "360p")
	action := "mv " + copiedRecoveryQuote(rendering) + " " + copiedRecoveryQuote(rendering+"-retired") + "\nmkdir " + copiedRecoveryQuote(rendering) + "\ncp " + copiedRecoveryQuote(rendering+"-retired/init.mp4") + " " + copiedRecoveryQuote(rendering+"/init.mp4") + "\ncp " + copiedRecoveryQuote(rendering+"-retired/segment-00000.m4s") + " " + copiedRecoveryQuote(rendering+"/segment-00000.m4s")
	copiedRecoveryProbe(t, manager, action)
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err == nil {
		t.Fatal("identical bytes in a replacement rendition acquired old-generation clock")
	}
}

func TestCopiedRecoveryCanceledProbeJoinsAndReleasesAdmission(t *testing.T) {
	manager, item, recipe, directory, policy, _ := copiedRecoveryFixture(t)
	marker := filepath.Join(t.TempDir(), "owned-pid")
	copiedRecoveryProbe(t, manager, "printf '%s' $$ > "+copiedRecoveryQuote(marker)+"\nexec sleep 30")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- manager.ensureCopiedHLSClock(ctx, item, recipe, directory, policy) }()
	deadline := time.Now().Add(time.Second)
	var data []byte
	for time.Now().Before(deadline) {
		data, _ = os.ReadFile(marker)
		if len(data) > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled probe acquired a clock")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled probe did not settle")
	}
	if len(data) == 0 {
		t.Fatal("controlled probe never started")
	}
	copiedRecoveryAssertStopped(t, data)
	copiedRecoveryProbe(t, manager, "")
	if err := manager.ensureCopiedHLSClock(t.Context(), item, recipe, directory, policy); err != nil {
		t.Fatal("cancellation retained metadata admission")
	}
}

func copiedRecoveryAssertStopped(t *testing.T, data []byte) {
	t.Helper()
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	if process.Signal(syscall.Signal(0)) == nil {
		t.Fatal("owned probe remained live after cancellation")
	}
}

const copiedRecoveryManifest = "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:2.000000,\nsegment-00000.m4s\n#EXTINF:2.000000,\nsegment-00001.m4s\n#EXT-X-ENDLIST\n"

func copiedRecoveryQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func copiedRecoveryProbe(t *testing.T, manager *hlsManager, action string) {
	t.Helper()
	probe := filepath.Join(t.TempDir(), "clock-probe")
	body := "#!/bin/sh\nset -eu\ncat >/dev/null\n" + action + "\nprintf '%s' '{\"packets\":[{\"pts_time\":\"0.083333\",\"flags\":\"K\"}]}'\n"
	if err := os.WriteFile(probe, []byte(body), 0o700); err != nil { //nolint:gosec // Owned executable probe stand-in, not data-file permissions.
		t.Fatal(err)
	} //nolint:gosec // Fixed local stand-in, never real media.
	manager.probe.executable = probe
}

func TestCopiedRecoveryClockRejectsReplacedRecipeGeneration(t *testing.T) {
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	before, err := os.ReadFile(filepath.Join(directory, ".copy-timeline"))
	if err != nil {
		t.Fatal(err)
	}
	old := directory + "-retired"
	copiedRecoveryProbe(t, manager, "mv "+copiedRecoveryQuote(directory)+" "+copiedRecoveryQuote(old)+"\nmkdir "+copiedRecoveryQuote(directory)+"\ncp "+copiedRecoveryQuote(old+"/.source")+" "+copiedRecoveryQuote(directory+"/.source")+"\ncp "+copiedRecoveryQuote(old+"/.copy-timeline")+" "+copiedRecoveryQuote(directory+"/.copy-timeline"))
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err == nil {
		t.Error("clock accepted a replaced recipe generation")
	}
	after, err := os.ReadFile(filepath.Join(directory, ".copy-timeline"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("old probe mutated replacement generation")
	}
}

func TestCopiedRecoveryClockRejectsChangedMeasuredAssets(t *testing.T) {
	for _, name := range []string{"init.mp4", "segment-00000.m4s"} {
		t.Run(name, func(t *testing.T) {
			manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
			copiedRecoveryProbe(t, manager, "printf changed > "+copiedRecoveryQuote(filepath.Join(directory, "360p", name)))
			if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err == nil {
				t.Error("clock accepted changed measured bytes")
			}
			data, err := os.ReadFile(filepath.Join(directory, ".copy-timeline"))
			if err != nil {
				t.Fatal(err)
			}
			var stored copiedHLSTimeline
			if json.Unmarshal(data, &stored) != nil || stored.Clock != nil {
				t.Fatal("changed assets acquired a clock certificate")
			}
		})
	}
}
