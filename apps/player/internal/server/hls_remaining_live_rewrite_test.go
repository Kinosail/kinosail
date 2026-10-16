package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestRemainingAACSegmentsSurviveLivePlaylistRewrites(t *testing.T) {
	manager, item, recipe := remainingAACCacheFixture(t)
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))
	master := []byte("#EXTM3U\n#KINOSAIL-TRANSCODER:" + options.Cache + "\n#EXT-X-STREAM-INF:BANDWIDTH=192000\naudio/index.m3u8\n")
	manifest := []byte(remainingInitialAACPrefix)
	writeHLSLoadingFile(t, filepath.Join(directory, "index.m3u8"), string(master))
	writeHLSLoadingFile(t, filepath.Join(directory, ".source"), options.Cache)
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/index.m3u8"), string(manifest))
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/init.mp4"), string(remainingAACCacheInitialization(t)))
	payload := []byte("published-fragment")
	writeHLSLoadingFile(t, filepath.Join(directory, "audio/segment-00000.m4s"), string(payload))
	ctx, stop := context.WithCancel(t.Context())
	rewritten := make(chan struct{})
	go func() {
		defer close(rewritten)
		for ctx.Err() == nil {
			_ = writeAtomicFile(filepath.Join(directory, "index.m3u8"), master)
			_ = writeAtomicFile(filepath.Join(directory, "audio/index.m3u8"), manifest)
		}
	}()
	failures := 0
	for range 200 {
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+item.ID+"/p/fixture/audio/segment-00000.m4s", nil)
		manager.serveRecipe(response, request, item, recipe, "audio/segment-00000.m4s")
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), payload) {
			failures++
		}
	}
	stop()
	<-rewritten
	if failures != 0 {
		t.Fatalf("%d of 200 published segment requests failed while the encoder rewrote its playlists", failures)
	}
}
