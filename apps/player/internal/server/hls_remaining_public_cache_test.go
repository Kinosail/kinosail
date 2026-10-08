package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

// Real public HTTP complements the isolated stale-asset and replacement races.
// Only disposable generated media/cache metadata are changed. Native devices
// and worker/process observations remain separate hosted origin-proof evidence.
func TestRemainingPublicAACOldCacheRebuildAndColdReuse(t *testing.T) { //nolint:funlen,gocognit,cyclop // One bounded public cache lifecycle retains its before/after evidence.
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for real public AAC cache proof")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for real public AAC cache proof")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	publication := remainingPublicObservePublication(t)
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	source := filepath.Join(media, "Fixture.flac")
	fixture := "aevalsrc='0.1*sin(2*PI*(440*t+20*t*t))|0.1*sin(2*PI*(670*t+31*t*t))':s=48000:d=10"
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", fixture, "-c:a", "flac", source) //nolint:gosec // Fixed synthetic fixture and discovered codec; no original media.
	if err := command.Run(); err != nil {
		t.Fatalf("generate public cache fixture: %v", err)
	}
	originalSource := remainingPublicRead(t, source)
	marker, adapter := filepath.Join(tools, "calls"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, adapter, fmt.Sprintf("#!/bin/sh\nprintf 'call %%s\\n' \"$$\" >> %s\nexec %s \"$@\"\n", remainingPublicQuote(marker), remainingPublicQuote(ffmpeg)))
	ownerContext, stopOwner := context.WithCancel(ctx)
	defer stopOwner()
	config := server.Config{Lifecycle: ownerContext, MediaDir: media, CacheDir: cache, FFmpeg: adapter, FFprobe: ffprobe}
	handler, id := formatTestItem(t, config)
	var info struct {
		Compatible     string
		CompatiblePlan playback.PlaybackPlan
	}
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?audioCodecs=aac", nil)
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &info) != nil || info.CompatiblePlan.Mode != "audio-transcode" {
		t.Fatal("public fixture did not select AAC compatibility")
	}
	root := filepath.Join(cache, playback.HLSRecipeKey(id, playback.RecipeFor(info.CompatiblePlan)))
	base := strings.TrimSuffix(info.Compatible, "index.m3u8") + "audio/"
	remainingPublicGet(t, ctx, handler, info.Compatible, "", http.StatusOK)
	remainingPublicWaitEOF(t, ctx, root)
	remainingPublicWaitWorkers(t, ctx, handler, marker, publication, 1)
	original := remainingPublicAudio(t, ctx, handler, base)
	pcm := remainingPublicPCM(t, ctx, ffmpeg, original)
	if len(pcm) != 481280*2*2 {
		t.Fatalf("complete native AAC samples = %d, expected481280", len(pcm)/4)
	}
	master := remainingPublicRead(t, filepath.Join(root, "index.m3u8"))
	policy := remainingPublicRead(t, filepath.Join(root, ".source"))
	if !bytes.Contains(policy, []byte(":aac-origin=1")) {
		t.Fatal("public corrected AAC cache lacks its correction identity")
	}
	legacy := remainingPublicLegacyRefill(t, ctx, ffmpeg, source)
	receipt := map[string]any{
		"fixtureSHA256": remainingPublicHash(originalSource), "nativeSamples": len(pcm) / 4,
		"publicSHA256": remainingPublicHash(original), "nativePCMSHA256": remainingPublicHash(pcm), "staleCases": []string{}, "coldReopens": 0,
	}
	for index, version := range []string{"15", "18"} {
		remainingPublicWaitWorkers(t, ctx, handler, marker, publication, index+1)
		legacyPolicy := bytes.ReplaceAll(policy, []byte(":aac-origin=1"), nil)
		legacyPolicy = bytes.ReplaceAll(legacyPolicy, []byte(":hls=18"), []byte(":hls="+version))
		staleMaster := bytes.ReplaceAll(master, policy, legacyPolicy)
		receipt["historicalCollision"] = remainingPublicInstallLegacy(t, ctx, ffmpeg, root, legacy, pcm)
		remainingPublicWrite(t, filepath.Join(root, ".source"), legacyPolicy)
		remainingPublicWrite(t, filepath.Join(root, "index.m3u8"), staleMaster)
		assets := remainingPublicSnapshot(t, root)
		calls := remainingPublicRead(t, marker)
		for _, asset := range []string{"init.mp4", "segment-00000.m4s"} {
			for _, header := range []string{"", "bytes=0-31"} {
				remainingPublicGet(t, ctx, handler, base+asset, header, http.StatusNotFound)
			}
		}
		if !bytes.Equal(calls, remainingPublicRead(t, marker)) || !remainingPublicSameSnapshot(t, root, assets) {
			t.Fatal("rejected old direct AAC request started encoding or changed retained cache")
		}
		remainingPublicGet(t, ctx, handler, info.Compatible, "", http.StatusOK)
		remainingPublicWaitEOF(t, ctx, root)
		remainingPublicWaitWorkers(t, ctx, handler, marker, publication, index+2)
		rebuilt := remainingPublicAudio(t, ctx, handler, base)
		if !bytes.Equal(original, rebuilt) || !bytes.Equal(pcm, remainingPublicPCM(t, ctx, ffmpeg, rebuilt)) ||
			!bytes.Equal(policy, remainingPublicRead(t, filepath.Join(root, ".source"))) ||
			bytes.Count(remainingPublicRead(t, marker), []byte("call ")) != bytes.Count(calls, []byte("call "))+1 {
			t.Fatal("old AAC cache rebuild did not restore the exact full-EOF control with one encoder")
		}
		receipt["staleCases"] = append(receipt["staleCases"].([]string), "hls"+version)
	}
	remainingPublicWaitWorkers(t, ctx, handler, marker, publication, 3)
	stopOwner()
	for round := 1; round <= 2; round++ {
		assets := remainingPublicSnapshot(t, root)
		calls := remainingPublicRead(t, marker)
		reopenedConfig, stopReopen := remainingPublicReopenedLifecycle(t, ctx, config)
		reopened, reopenedID := formatTestItem(t, reopenedConfig)
		if reopenedID != id {
			t.Fatal("cold public cache reopen changed the fixture identity")
		}
		remainingPublicGet(t, ctx, reopened, info.Compatible, "", http.StatusOK)
		actual := remainingPublicAudio(t, ctx, reopened, base)
		if !bytes.Equal(original, actual) || !bytes.Equal(pcm, remainingPublicPCM(t, ctx, ffmpeg, actual)) ||
			!bytes.Equal(calls, remainingPublicRead(t, marker)) || !remainingPublicSameSnapshot(t, root, assets) {
			t.Fatal("cold public cache reuse changed complete media, retained files or encoder count")
		}
		remainingPublicWaitWorkers(t, ctx, reopened, marker, publication, 3)
		stopReopen()
		receipt["coldReopens"] = round
	}
	if !bytes.Equal(originalSource, remainingPublicRead(t, source)) {
		t.Fatal("public cache proof changed its source fixture")
	}
	receipt["sourceUnchanged"] = true
	receipt["completedPublications"] = 3
	receipt["ownedWorkerZeroObservations"] = 2
	receipt["result"] = "passed"
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("remaining-public-cache-receipt %s", data)
}

func remainingPublicReopenedLifecycle(t *testing.T, ctx context.Context, config server.Config) (server.Config, context.CancelFunc) {
	t.Helper()
	lifecycle, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	config.Lifecycle = lifecycle
	return config, cancel
}
