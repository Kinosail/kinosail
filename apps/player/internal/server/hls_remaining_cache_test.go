package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
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
				writeHLSLoadingFile(t, filepath.Join(directory, asset), "obsolete-aac-asset")
				response := httptest.NewRecorder()
				request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+item.ID+"/p/fixture/"+asset, nil)
				manager.serveRecipe(response, request, item, recipe, asset)
				if response.Code == http.StatusOK && response.Body.String() == "obsolete-aac-asset" {
					t.Fatalf("obsolete hls%s AAC %s was delivered without correction-policy admission", version, asset)
				}
			})
		}
	}
}

func TestRemainingAACCorrectionHasCacheIdentity(t *testing.T) {
	manager, item, recipe := remainingAACCacheFixture(t)
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	if err := playback.ValidateHLSSource(item.Path, options.Cache); err != nil {
		t.Fatalf("correction identity invalidated the canonical source suffix: %v", err)
	}
	if !strings.Contains(options.Cache, ":aac-origin=1") {
		t.Fatal("AAC context correction shares the pre-correction cache policy")
	}
}

// Mocked asset delivery is an admission control, not native AAC media proof.
// The terminal ordinal is copied from the qualified real-media timeline;
// hosted decode and resource measurements still qualify production delivery.
func TestRemainingCurrentAACAssetsRemainReadable(t *testing.T) {
	for _, asset := range []string{"audio/init.mp4", "audio/segment-00000.m4s", "audio/segment-00005.m4s"} {
		t.Run(asset, func(t *testing.T) {
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
			writeHLSLoadingFile(t, filepath.Join(directory, asset), "current-policy-asset")
			path := filepath.Join(directory, asset)
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+item.ID+"/p/fixture/"+asset, nil)
			manager.serveRecipe(response, request, item, recipe, asset)
			after, err := os.Stat(path)
			if response.Code != http.StatusOK || response.Body.String() != "current-policy-asset" || err != nil {
				t.Fatalf("current policy asset %s rejected: status=%d error=%v", asset, response.Code, err)
			}
			if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || len(manager.jobs) != 0 {
				t.Fatal("cached delivery replaced the asset or scheduled encoding")
			}
		})
	}
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
