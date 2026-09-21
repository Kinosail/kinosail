package downloads

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestDownloadIdentityIsAuthorizedAndBoundToTheServer(t *testing.T) {
	manager := New(Config{Context: t.Context(), Cache: t.TempDir(), ServerID: "server-one"})
	mux := http.NewServeMux()
	RegisterAPI(mux, manager, testAccess{})
	for _, tc := range []struct {
		query  string
		allow  bool
		status int
	}{{"", true, 200}, {"?extra=1", true, 403}, {"", false, 403}} {
		response := requestAPI(t, mux, http.MethodGet, "/api/v1/downloads/identity"+tc.query, "", tc.allow)
		if response.Code != tc.status {
			t.Fatalf("identity status = %d", response.Code)
		}
		if tc.status == 200 && (!strings.Contains(response.Body.String(), `"serverId":"server-one"`) || !strings.Contains(response.Body.String(), `"profileId":"viewer"`)) {
			t.Fatalf("identity = %s", response.Body.String())
		}
	}
	manager.serverID = ""
	if response := requestAPI(t, mux, http.MethodGet, "/api/v1/downloads/identity", "", true); response.Code != 403 {
		t.Fatal("unconfigured identity accepted")
	}
}

func TestDownloadTrackDiscoveryPreservesSafeLabels(t *testing.T) {
	facts := playback.MediaFacts{Audio: []playback.AudioFacts{{Index: 2, Language: "en", Role: "commentary"}}, Subtitles: []playback.SubtitleFacts{{Index: 5, Language: "bad\nvalue", Role: strings.Repeat("x", 65)}}}
	manager := New(Config{Context: t.Context(), Cache: t.TempDir(), Inspect: func(context.Context, library.Item) playback.MediaFacts { return facts }})
	mux := http.NewServeMux()
	RegisterAPI(mux, manager, testAccess{library.Item{ID: "item", Kind: "video"}})
	response := requestAPI(t, mux, http.MethodGet, "/api/v1/items/item/download-tracks", "", true)
	var tracks TrackOptions
	if err := json.Unmarshal(response.Body.Bytes(), &tracks); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || len(tracks.Audio) != 1 || len(tracks.Subtitles) != 1 {
		t.Fatalf("tracks = %d %#v", response.Code, tracks)
	}
	if tracks.Audio[0].Index != 2 || tracks.Audio[0].Label != "Audio 3 · en · commentary" || tracks.Subtitles[0].Label != "Subtitles 6" {
		t.Fatalf("labels = %#v", tracks)
	}
	assertTrackDiscoveryAccess(t, mux)

	facts.Audio = make([]playback.AudioFacts, 33)
	if response := requestAPI(t, mux, http.MethodGet, "/api/v1/items/item/download-tracks", "", true); response.Code != 400 {
		t.Fatal("unbounded tracks accepted")
	}
}

func TestTrackDiscoveryRejectsUnsupportedSources(t *testing.T) {
	manager := New(Config{Context: t.Context(), Cache: t.TempDir()})
	if _, err := manager.Tracks(library.Item{Kind: "video"}); err == nil {
		t.Fatal("missing inspector accepted")
	}
	manager.inspect = func(context.Context, library.Item) playback.MediaFacts {
		return playback.MediaFacts{Subtitles: make([]playback.SubtitleFacts, 257)}
	}
	for _, kind := range []string{"audio", "video"} {
		if _, err := manager.Tracks(library.Item{Kind: kind}); err == nil {
			t.Fatalf("unsupported %s tracks accepted", kind)
		}
	}
}

func assertTrackDiscoveryAccess(t *testing.T, mux *http.ServeMux) {
	t.Helper()
	for _, path := range []string{"/api/v1/items/item/download-tracks?extra=1", "/api/v1/items/missing/download-tracks"} {
		if response := requestAPI(t, mux, http.MethodGet, path, "", true); response.Code != 404 {
			t.Fatalf("invalid track request = %d", response.Code)
		}
	}
	if response := requestAPI(t, mux, http.MethodGet, "/api/v1/items/item/download-tracks", "", false); response.Code != 404 {
		t.Fatal("unauthorized tracks exposed")
	}
}
