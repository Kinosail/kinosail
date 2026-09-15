package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"

	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestVariantReadyWithOneSegment(t *testing.T) {
	root := t.TempDir()
	source, variant := filepath.Join(root, "source.mkv"), filepath.Join(root, "360p")
	if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(variant, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(variant, "init.mp4"), mp4fixture.Initialization(640, 360, "h264", "aac", ""), 0o600); err != nil {
		t.Fatal(err)
	}
	playlist := filepath.Join(variant, "index.m3u8")
	oneSegment := "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:4,\nsegment-00000.m4s\n"
	if err := os.WriteFile(playlist, []byte(oneSegment), 0o600); err != nil {
		t.Fatal(err)
	}
	if playback.VariantReady(source, variant) {
		t.Fatal("variant became ready before its first segment existed")
	}
	if err := os.WriteFile(filepath.Join(variant, "segment-00000.m4s"), []byte("segment"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !playback.VariantReady(source, variant) {
		t.Fatal("variant did not become ready with one segment")
	}
}

func TestSeekUsesShortSegments(t *testing.T) {
	used := strings.Join(hlsSegmentArguments("transcode", "/cache/1080p", "/cache/seek/1080p/index.m3u8", 75), " ")
	if !strings.Contains(used, "-hls_time 2") || !strings.Contains(used, "-start_number 75") {
		t.Fatalf("seek arguments = %q", used)
	}
}

func TestJellyfinHLSRecipeKeepsRemoteAdaptiveAndMakesLANSingleQuality(t *testing.T) {
	recipe := hlsRecipe{mode: "transcode"}
	local := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Videos/item/index.m3u8", nil)
	if got := testJellyfinHLSRecipe(local, recipe); !got.singleQuality {
		t.Fatal("LAN Jellyfin transcode did not select one quality")
	}
	var remote *http.Request
	Remote(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) { remote = request })).ServeHTTP(httptest.NewRecorder(), local)
	if got := testJellyfinHLSRecipe(remote, recipe); got.singleQuality {
		t.Fatal("public Jellyfin transcode lost its adaptive ladder")
	}
	if hlsRecipeKey("item", testJellyfinHLSRecipe(local, recipe)) == hlsRecipeKey("item", testJellyfinHLSRecipe(remote, recipe)) {
		t.Fatal("LAN and public Jellyfin transcodes share a cache key")
	}
}

func testJellyfinHLSRecipe(request *http.Request, recipe hlsRecipe) hlsRecipe {
	return localHLSRecipe(sharedjellyfin.HLSRecipe(request, sharedHLSRecipe(recipe), true, publicInternetRequest))
}

func TestJellyfinPlaylistPublishesCompleteVODTimeline(t *testing.T) {
	manifest := []byte("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:2\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:2.004000,\nsegment-00000.m4s\n")
	result := string(hlsPlaylistWithSession(manifest, "playback-session", "", 0, 9))
	if !strings.Contains(result, "#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-START:TIME-OFFSET=0,PRECISE=YES\n") || !strings.Contains(result, "segment-00004.m4s?playSessionId=playback-session\n#EXT-X-ENDLIST\n") {
		t.Fatalf("VOD playlist = %q", result)
	}
}

func TestJellyfinVODPlaylistStartsAtTheRequestedResumePosition(t *testing.T) {
	manifest := []byte("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:2,\nsegment-00000.m4s\n")
	result := string(hlsPlaylistWithSession(manifest, "playback-session", "", 1, 2))
	if !strings.Contains(result, "#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-START:TIME-OFFSET=1,PRECISE=YES\n") || !strings.Contains(result, "segment-00000.m4s?playSessionId=playback-session&start=1") {
		t.Fatalf("VOD playlist resume start = %q", result)
	}
}

func TestJellyfinVODPlaylistRejectsInvalidSegmentDuration(t *testing.T) {
	for _, value := range []string{"invalid", "NaN", "0", "61"} {
		manifest := []byte("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:" + value + ",\nsegment-00000.m4s\n")
		if result := completeHLSVOD(manifest, 10); string(result) != string(manifest) {
			t.Fatalf("invalid source playlist changed: %q", result)
		}
	}
}

func TestJellyfinVODPlaylistUsesObservedSegmentCadence(t *testing.T) {
	manifest := []byte("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:2.1,\nsegment-00000.m4s\n#EXTINF:1.9,\nsegment-00001.m4s\n")
	result := string(completeHLSVOD(manifest, 4.1))
	if !strings.Contains(result, "segment-00002.m4s\n#EXT-X-ENDLIST\n") {
		t.Fatalf("VOD playlist used only its first segment duration: %q", result)
	}
}

func TestHLSSeekUsesThePublishedSegmentTimeline(t *testing.T) {
	manifest := []byte("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:2.1,\nsegment-00000.m4s\n#EXTINF:1.9,\nsegment-00001.m4s\n")
	if offset, valid := hlsSegmentOffset(manifest, "segment-00075.m4s", 400); !valid || offset != 150 {
		t.Fatalf("segment offset = %v, %t", offset, valid)
	}
	for _, invalid := range []string{"segment-00075.mp4", "00075.m4s", "segment-100000.m4s"} {
		if offset, valid := hlsSegmentOffset(manifest, invalid, 400); valid {
			t.Fatalf("invalid segment %q offset = %v", invalid, offset)
		}
	}
	malformed := []byte("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:NaN,\nsegment-00075.m4s\n#EXT-X-ENDLIST\n")
	if offset, valid := hlsSegmentOffset(malformed, "segment-00075.m4s", 400); valid {
		t.Fatalf("malformed timeline offset = %v", offset)
	}
}

func TestJellyfinVODPlaylistPreservesCompletedTranscoderTimeline(t *testing.T) {
	manifest := []byte("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.1,\nsegment-00000.m4s\n#EXT-X-DISCONTINUITY\n#EXTINF:1.2,\nsegment-00001.m4s\n#EXT-X-ENDLIST\n")
	result := string(completeHLSVOD(manifest, 9))
	if !strings.Contains(result, "#EXT-X-PLAYLIST-TYPE:VOD\n") || !strings.Contains(result, "#EXT-X-DISCONTINUITY\n") || !strings.Contains(result, "#EXTINF:1.2,\nsegment-00001.m4s\n#EXT-X-ENDLIST\n") || strings.Contains(result, "segment-00002.m4s") {
		t.Fatalf("completed VOD playlist changed: %q", result)
	}
}

func TestWaitForHLSFileFollowsThePublishedVODTimeline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "segment-00002.m4s")
	go func() {
		time.Sleep(25 * time.Millisecond)
		_ = os.WriteFile(path, []byte("segment"), 0o600)
	}()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if !waitForHLSFile(ctx, path) {
		t.Fatal("ready VOD segment was not observed")
	}
	canceled, stop := context.WithCancel(t.Context())
	stop()
	if waitForHLSFile(canceled, filepath.Join(t.TempDir(), "missing.m4s")) {
		t.Fatal("canceled segment wait succeeded")
	}
}

func TestServeRecipeRejectsUnknownPathsWithoutCacheSideEffects(t *testing.T) {
	cache := t.TempDir()
	manager := &hlsManager{cache: cache}
	for _, name := range []string{"../index.m3u8", "360p/../../index.m3u8", `360p\\index.m3u8`, "360p/segment-12x45.m4s"} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/item/"+name, nil)
			manager.serveRecipe(response, request, library.Item{ID: "item"}, hlsRecipe{}, name)
			if response.Code != http.StatusNotFound {
				t.Fatalf("invalid HLS path status = %d", response.Code)
			}
			entries, err := os.ReadDir(cache)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid HLS path changed cache: %v, %v", entries, err)
			}
		})
	}
}
