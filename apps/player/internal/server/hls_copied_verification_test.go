package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func boundCopiedPlaylistFixture(t *testing.T) (*hlsManager, string, func(string) *httptest.ResponseRecorder) {
	t.Helper()
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	copiedRecoveryProbe(t, manager, "")
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
		t.Fatal(err)
	}
	settled := time.Now().Add(-time.Hour)
	for _, name := range []string{"init.mp4", "segment-00000.m4s"} {
		if err := os.Chtimes(filepath.Join(directory, "360p", name), settled, settled); err != nil {
			t.Fatal(err)
		}
	}
	return manager, directory, func(name string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+item.ID+"/p/"+recipe.token()+"/"+name, nil)
		manager.serveRecipe(response, request, item, recipe, name)
		return response
	}
}

func TestCopiedHLSPlaylistsReuseVerifiedTimelineAndRetryWhenVerificationIsBusy(t *testing.T) {
	manager, directory, playlist := boundCopiedPlaylistFixture(t)
	if response := playlist("360p/index.m3u8"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "#EXT-X-PLAYLIST-TYPE:VOD") {
		t.Fatalf("cold rendition = %d %q", response.Code, response.Body.String())
	}
	preserved := copiedRecoveryPreserved(t, directory)
	manager.copiedMetadata.mu.Lock()
	gate := manager.copiedMetadata.gate
	manager.copiedMetadata.mu.Unlock()
	for range cap(gate) {
		gate <- struct{}{}
	}
	for _, name := range []string{"index.m3u8", "360p/index.m3u8"} {
		if response := playlist(name); response.Code != http.StatusOK {
			t.Fatalf("verified %s waited for hashing admission: %d %q", name, response.Code, response.Body.String())
		}
	}
	first := filepath.Join(directory, "360p", "segment-00000.m4s")
	replacement := first + ".replacement"
	writeHLSLoadingFile(t, replacement, "first fragment")
	if err := os.Rename(replacement, first); err != nil {
		t.Fatal(err)
	}
	busy := playlist("360p/index.m3u8")
	if busy.Code != http.StatusServiceUnavailable || busy.Header().Get("Retry-After") == "" {
		t.Fatalf("busy verification = %d %v %q", busy.Code, busy.Header(), busy.Body.String())
	}
	manager.mu.Lock()
	jobs := len(manager.jobs)
	manager.mu.Unlock()
	if jobs != 0 {
		t.Fatal("busy verification started encoding")
	}
	preserved()
	for range cap(gate) {
		<-gate
	}
	if response := playlist("360p/index.m3u8"); response.Code != http.StatusOK {
		t.Fatalf("released verification = %d %q", response.Code, response.Body.String())
	}
}
