package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	source := filepath.Join(media, "Fixture.flac")
	fixture := "aevalsrc='0.1*sin(2*PI*(440*t+20*t*t))|0.1*sin(2*PI*(670*t+31*t*t))':s=48000:d=10"
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", fixture, "-c:a", "flac", source) //nolint:gosec // Fixed synthetic fixture and discovered codec; no original media.
	if err := command.Run(); err != nil {
		t.Fatalf("generate public cache fixture: %v", err)
	}
	originalSource := remainingPublicRead(t, source)
	marker, adapter := filepath.Join(tools, "calls"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, adapter, fmt.Sprintf("#!/bin/sh\nprintf 'call\\n' >> %s\nexec %s \"$@\"\n", remainingPublicQuote(marker), remainingPublicQuote(ffmpeg)))
	config := server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: cache, FFmpeg: adapter, FFprobe: ffprobe}
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
	receipt := map[string]any{"fixtureSHA256": remainingPublicHash(originalSource), "nativeSamples": len(pcm) / 4,
		"publicSHA256": remainingPublicHash(original), "nativePCMSHA256": remainingPublicHash(pcm), "staleCases": []string{}, "coldReopens": 0}
	for _, version := range []string{"15", "18"} {
		legacy := bytes.ReplaceAll(policy, []byte(":aac-origin=1"), nil)
		legacy = bytes.ReplaceAll(legacy, []byte(":hls=15"), []byte(":hls="+version))
		staleMaster := bytes.ReplaceAll(master, policy, legacy)
		remainingPublicWrite(t, filepath.Join(root, ".source"), legacy)
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
		rebuilt := remainingPublicAudio(t, ctx, handler, base)
		if !bytes.Equal(original, rebuilt) || !bytes.Equal(pcm, remainingPublicPCM(t, ctx, ffmpeg, rebuilt)) ||
			!bytes.Equal(policy, remainingPublicRead(t, filepath.Join(root, ".source"))) ||
			bytes.Count(remainingPublicRead(t, marker), []byte("call\n")) != bytes.Count(calls, []byte("call\n"))+1 {
			t.Fatal("old AAC cache rebuild did not restore the exact full-EOF control with one encoder")
		}
		receipt["staleCases"] = append(receipt["staleCases"].([]string), "hls"+version)
	}
	for round := 1; round <= 2; round++ {
		assets := remainingPublicSnapshot(t, root)
		calls := remainingPublicRead(t, marker)
		reopened, reopenedID := formatTestItem(t, config)
		if reopenedID != id {
			t.Fatal("cold public cache reopen changed the fixture identity")
		}
		remainingPublicGet(t, ctx, reopened, info.Compatible, "", http.StatusOK)
		actual := remainingPublicAudio(t, ctx, reopened, base)
		if !bytes.Equal(original, actual) || !bytes.Equal(pcm, remainingPublicPCM(t, ctx, ffmpeg, actual)) ||
			!bytes.Equal(calls, remainingPublicRead(t, marker)) || !remainingPublicSameSnapshot(t, root, assets) {
			t.Fatal("cold public cache reuse changed complete media, retained files or encoder count")
		}
		receipt["coldReopens"] = round
	}
	if !bytes.Equal(originalSource, remainingPublicRead(t, source)) {
		t.Fatal("public cache proof changed its source fixture")
	}
	receipt["sourceUnchanged"] = true
	receipt["result"] = "passed"
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("remaining-public-cache-receipt %s", data)
}

type remainingPublicAsset struct {
	info os.FileInfo
	data []byte
}

func remainingPublicSnapshot(t *testing.T, root string) map[string]remainingPublicAsset {
	t.Helper()
	assets := map[string]remainingPublicAsset{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		assets[path] = remainingPublicAsset{info: info, data: remainingPublicRead(t, path)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return assets
}

func remainingPublicSameSnapshot(t *testing.T, root string, before map[string]remainingPublicAsset) bool {
	t.Helper()
	after := remainingPublicSnapshot(t, root)
	if len(before) != len(after) {
		return false
	}
	for path, asset := range before {
		current, ok := after[path]
		if !ok || !os.SameFile(asset.info, current.info) || !asset.info.ModTime().Equal(current.info.ModTime()) ||
			asset.info.Size() != current.info.Size() || !bytes.Equal(asset.data, current.data) {
			return false
		}
	}
	return true
}

func remainingPublicAudio(t *testing.T, ctx context.Context, handler http.Handler, base string) []byte {
	t.Helper()
	variant := remainingPublicGet(t, ctx, handler, base+"index.m3u8", "", http.StatusOK)
	if !bytes.Contains(variant, []byte("#EXT-X-ENDLIST")) || !bytes.Contains(variant, []byte("#EXT-X-PLAYLIST-TYPE:VOD")) {
		t.Fatal("public AAC timeline was not completed VOD")
	}
	data := remainingPublicGet(t, ctx, handler, base+"init.mp4", "", http.StatusOK)
	names := speedTestURIs(variant)
	if len(names) < 5 || len(names) > 6 {
		t.Fatalf("public AAC fragment count = %d", len(names))
	}
	for _, name := range names {
		fragment := remainingPublicGet(t, ctx, handler, base+name, "", http.StatusOK)
		partial := remainingPublicGet(t, ctx, handler, base+name, "bytes=0-31", http.StatusPartialContent)
		if len(fragment) < 32 || !bytes.Equal(partial, fragment[:32]) {
			t.Fatal("public fragment Range did not return the same retained bytes")
		}
		data = append(data, fragment...)
	}
	return data
}

func remainingPublicGet(t *testing.T, ctx context.Context, handler http.Handler, route, byteRange string, status int) []byte {
	t.Helper()
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, route, nil)
	if byteRange != "" {
		request.Header.Set("Range", byteRange)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("public cache asset returned%d, expected%d", response.Code, status)
	}
	return response.Body.Bytes()
}

func remainingPublicPCM(t *testing.T, ctx context.Context, ffmpeg string, data []byte) []byte {
	t.Helper()
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-threads", "1", "-i", "pipe:0", "-map", "0:a:0", "-f", "s16le", "-") //nolint:gosec // Discovered codec and actual bounded generated public bytes.
	command.Stdin = bytes.NewReader(data)
	pcm, err := command.Output()
	if err != nil || len(pcm) > 2<<20 {
		t.Fatalf("full public AAC decode failed: %v", err)
	}
	return pcm
}

func remainingPublicWaitEOF(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	for {
		data, err := os.ReadFile(filepath.Join(root, "audio/index.m3u8"))
		if err == nil && bytes.Contains(data, []byte("#EXT-X-ENDLIST")) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("physical AAC EOF was not published within the bounded cache proof")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func remainingPublicRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 2<<20 {
		t.Fatalf("bounded fixture read failed: %v", err)
	}
	return data
}

func remainingPublicWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func remainingPublicHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func remainingPublicQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
