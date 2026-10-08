package server

import (
	"bytes"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/isobmff"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
	"github.com/MikeO7/kinosail/packages/transcodehardware"
	"github.com/MikeO7/kinosail/packages/workload"
)

// The hosted origin proof uses fresh corrected caches. It cannot exercise old
// cached assets reaching HTTP delivery without consulting encoder arguments.
// These isolated HTTP-boundary cases protect that concrete admission gap.
func TestRemainingOldAACAssetsCannotBypassPolicy(t *testing.T) {
	for _, version := range []string{"15", "18"} {
		for _, asset := range []string{"audio/init.mp4", "audio/segment-00000.m4s"} {
			t.Run(version+"/"+asset, func(t *testing.T) {
				remainingAACAssertOldCache(t, version, asset)
			})
		}
	}
}

func TestRemainingAACCorrectionHasCacheIdentity(t *testing.T) {
	for _, extension := range []string{".flac", ".ogg", ".wav"} {
		t.Run(extension, func(t *testing.T) {
			remainingAACAssertIdentity(t, extension)
		})
	}
}

// Mocked asset delivery is an admission control, not native AAC media proof.
// The terminal ordinal is copied from the qualified real-media timeline;
// hosted decode and resource measurements still qualify production delivery.
func TestRemainingCurrentAACAssetsRemainReadable(t *testing.T) {
	for _, asset := range []string{"audio/init.mp4", "audio/segment-00000.m4s", "audio/segment-00005.m4s"} {
		t.Run(asset, func(t *testing.T) {
			remainingAACAssertCurrentCache(t, asset)
		})
	}
}

func remainingAACCacheInitialization(t *testing.T) []byte {
	t.Helper()
	// Retain only the AAC sample-description track from the shared mock fixture.
	// Parse proves structural admission; this data does not claim native playback.
	combined := mp4fixture.Initialization(640, 360, "h264", "aac", "")
	videoSize := int(binary.BigEndian.Uint32(combined[8:12]))
	initialization := mp4fixture.Box("moov", combined[8+videoSize:])
	if _, err := isobmff.Parse(initialization); err != nil {
		t.Fatalf("AAC structural initialization fixture rejected: %v", err)
	}
	return initialization
}

func TestRemainingAACIdentityPreservesOtherRecipes(t *testing.T) {
	for _, kind := range []string{"video", "audiobook"} {
		t.Run(kind, func(t *testing.T) {
			manager, item, recipe := remainingAACCacheFixture(t)
			item.Kind = kind
			options, err := manager.hlsSettings(item, recipe)
			if err != nil || strings.Contains(options.Cache, ":aac-origin=1") {
				t.Fatalf("unaffected %s policy changed: error=%v", kind, err)
			}
		})
	}
}

func TestRemainingAACIdentityExcludesUnqualifiedRecipes(t *testing.T) {
	cases := map[string]hlsRecipe{
		"offset":      {mode: "audio-transcode", offset: 8},
		"output-time": {mode: "audio-transcode", outputTime: 8},
		"track":       {mode: "audio-transcode", audio: 1},
		"low-rate":    {mode: "audio-transcode", maxBitrate: 128000},
		"dialogue":    {mode: "audio-transcode", dialogueBoost: true},
		"normalize":   {mode: "audio-transcode", normalizeLoudness: true},
		"omitted":     {mode: "audio-transcode", omitted: []PlaybackRange{{Start: 1, End: 2}}},
	}
	for name, recipe := range cases {
		t.Run(name, func(t *testing.T) {
			manager, item, _ := remainingAACCacheFixture(t)
			options, err := manager.hlsSettings(item, recipe)
			if err != nil || strings.Contains(options.Cache, ":aac-origin=1") {
				t.Fatalf("unqualified %s identity changed: error=%v", name, err)
			}
		})
	}
}

func remainingAACCacheFixture(t *testing.T) (*hlsManager, library.Item, hlsRecipe) {
	t.Helper()
	tools := t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "audio", Path: filepath.Join(tools, "Fixture.flac")}
	writeHLSLoadingFile(t, item.Path, "synthetic-source")
	probe := filepath.Join(tools, "ffprobe")
	script := "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"index\":0,\"codec_type\":\"audio\",\"codec_name\":\"flac\",\"sample_rate\":\"48000\",\"channels\":2,\"channel_layout\":\"stereo\"}],\"format\":{\"format_name\":\"flac\",\"duration\":\"10\"}}'\n"
	if err := os.WriteFile(probe, []byte(script), 0o700); err != nil { //nolint:gosec // Local synthetic probe executable.
		t.Fatal(err)
	}
	settings := &settingsStore{}
	settings.value.Selection = transcodehardware.Selection{Accelerator: "none"}
	manager := newHLS(t.Context(), t.TempDir(), "must-not-run", nil, newMediaProbe(probe), settings, workload.New(1))
	manager.startup = newStartupPreparation(manager, nil, 0)
	return manager, item, hlsRecipe{mode: "audio-transcode"}
}

func remainingAACAssertOldCache(t *testing.T, version, asset string) {
	t.Helper()
	manager, item, recipe := remainingAACCacheFixture(t)
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	policy := strings.ReplaceAll(options.Cache, ":aac-origin=1", "")
	policy = strings.ReplaceAll(policy, ":hls=15", ":hls="+version)
	policy = strings.ReplaceAll(policy, ":hls=18", ":hls="+version)
	key := hlsRecipeKey(item.ID, recipe)
	directory := filepath.Join(manager.cache, key)
	writeHLSLoadingFile(t, filepath.Join(directory, "index.m3u8"), "#EXTM3U\n#KINOSAIL-TRANSCODER:"+policy+"\n#EXT-X-STREAM-INF:BANDWIDTH=192000\naudio/index.m3u8\n")
	writeHLSLoadingFile(t, filepath.Join(directory, ".source"), policy)
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/index.m3u8"), remainingInitialAACPrefix)
	payload := []byte("obsolete-aac-asset")
	if strings.HasSuffix(asset, "init.mp4") {
		payload = remainingAACCacheInitialization(t)
	}
	writeHLSLoadingFile(t, filepath.Join(directory, asset), string(payload))
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+item.ID+"/p/fixture/"+asset, nil)
	manager.serveRecipe(response, request, item, recipe, asset)
	if response.Code == http.StatusOK && bytes.Equal(response.Body.Bytes(), payload) {
		t.Fatalf("obsolete hls%s AAC %s was delivered without correction-policy admission", version, asset)
	}
}

func remainingAACAssertIdentity(t *testing.T, extension string) {
	t.Helper()
	manager, item, recipe := remainingAACCacheFixture(t)
	renamed := strings.TrimSuffix(item.Path, ".flac") + extension
	if err := os.Rename(item.Path, renamed); err != nil {
		t.Fatal(err)
	}
	item.Path = renamed
	facts := mediaFactsFor(item, manager.probe.facts(t.Context(), item))
	if !remainingOriginSource(facts, 8) {
		t.Fatal("renamed source did not retain the observed FLAC correction eligibility")
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	if err := playback.ValidateHLSSource(item.Path, options.Cache); err != nil {
		t.Fatalf("correction identity invalidated the canonical source suffix: %v", err)
	}
	if !strings.Contains(options.Cache, ":aac-origin=1") {
		t.Fatal("eligible AAC context correction shares the pre-correction cache policy")
	}
}

func remainingAACAssertCurrentCache(t *testing.T, asset string) {
	t.Helper()
	manager, item, recipe := remainingAACCacheFixture(t)
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	key := hlsRecipeKey(item.ID, recipe)
	directory := filepath.Join(manager.cache, key)
	writeHLSLoadingFile(t, filepath.Join(directory, "index.m3u8"), "#EXTM3U\n#KINOSAIL-TRANSCODER:"+options.Cache+"\n#EXT-X-STREAM-INF:BANDWIDTH=192000\naudio/index.m3u8\n")
	writeHLSLoadingFile(t, filepath.Join(directory, ".source"), options.Cache)
	manifest := remainingInitialAACPrefix + "#EXTINF:0.021333,\nsegment-00005.m4s\n#EXT-X-ENDLIST\n"
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/index.m3u8"), manifest)
	initialization := remainingAACCacheInitialization(t)
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/init.mp4"), string(initialization))
	payload := []byte("current-policy-asset")
	if strings.HasSuffix(asset, "init.mp4") {
		payload = initialization
	}
	writeHLSLoadingFile(t, filepath.Join(directory, asset), string(payload))
	path := filepath.Join(directory, asset)
	before := remainingAACAssetInfo(t, path)
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+item.ID+"/p/fixture/"+asset, nil)
	manager.serveRecipe(response, request, item, recipe, asset)
	after, err := os.Stat(path)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), payload) || err != nil {
		t.Fatalf("current policy asset %s rejected: status=%d error=%v", asset, response.Code, err)
	}
	manager.mu.Lock()
	jobs := len(manager.jobs)
	manager.mu.Unlock()
	if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || jobs != 0 {
		t.Fatal("cached delivery replaced the asset or scheduled encoding")
	}
}

func remainingAACAssetInfo(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}
