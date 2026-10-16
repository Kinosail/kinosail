package server_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func probeInvocations(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "\n")
}

func TestJellyfinListingsUseOnlyCachedMediaFacts(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir, toolsDir := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival (2016).mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls, ffprobe := filepath.Join(toolsDir, "calls"), filepath.Join(toolsDir, "ffprobe")
	writeExecutable(t, ffprobe, fmt.Sprintf(`#!/bin/sh
echo probe >> %q
printf '%%s' '{"streams":[{"codec_type":"video","codec_name":"h264"}],"chapters":[{"start_time":"0","end_time":"60","tags":{"title":"Opening Credits"}}],"format":{"duration":"1260"}}'
`, calls))
	config := server.Config{MediaDir: mediaDir, DataDir: dataDir, CacheDir: t.TempDir(), FFprobe: ffprobe, RequireAuth: true}
	first := newJellyfinServer(t, config)
	token, _ := jellyfinLogin(t, first, signInTestProfile(t, first, "/setup", "name=Owner&password=owner-password"))
	id := jellyfinPageID(t, jellyfinPage(t, first, token, "/Items"), "Arrival")
	if response := jellyfinCall(t, first, http.MethodPost, "/Sessions/Playing/Progress", fmt.Sprintf(`{"ItemId":%q,"PositionTicks":1200000000}`, id), token); response.Code != http.StatusNoContent {
		t.Fatalf("progress = %d %q", response.Code, response.Body.String())
	}
	config.CacheDir = t.TempDir()
	restarted := newJellyfinServer(t, config)
	before := probeInvocations(t, calls)
	for _, path := range []string{"/Items", "/Items?IncludeItemTypes=Movie&searchTerm=arr", "/Items/Latest", "/UserItems/Resume", "/Items?ParentId=00000000000000000000000000000001"} {
		response := jellyfinCall(t, restarted, http.MethodGet, path, "", token)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"PlaybackPositionTicks":1200000000`) {
			t.Fatalf("%s = %d %q", path, response.Code, response.Body.String())
		}
	}
	if after := probeInvocations(t, calls); after != before {
		t.Fatalf("Jellyfin listings ran ffprobe %d times", after-before)
	}
	item := jellyfinCall(t, restarted, http.MethodGet, "/Items/"+id, "", token)
	if item.Code != http.StatusOK || !strings.Contains(item.Body.String(), `"PlaybackPositionTicks":600000000`) {
		t.Fatalf("item = %d %q", item.Code, item.Body.String())
	}
	listing := jellyfinCall(t, restarted, http.MethodGet, "/Items", "", token)
	if !strings.Contains(listing.Body.String(), `"PlaybackPositionTicks":600000000`) {
		t.Fatalf("listing after probe = %d %q", listing.Code, listing.Body.String())
	}
}
