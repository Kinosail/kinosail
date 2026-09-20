package servertest

import (
	"bytes"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// APIParityFixture binds populated and empty real-app handlers.
type APIParityFixture struct {
	Server    func(*testing.T) (http.Handler, string)
	Bootstrap func(*testing.T) http.Handler
	TOTP      func(*testing.T, string, time.Time) string
}

// APIParityContract exercises shared browser and API capabilities.
func APIParityContract(t *testing.T, fixture APIParityFixture) {
	t.Helper()
	t.Run("ViewerMediaStateAndCurationAreAvailableThroughAPI", func(t *testing.T) { viewerMediaStateAndCurationAreAvailableThroughAPI(t, fixture) })
	t.Run("OwnerCanManagePlaybackMarkersThroughAPI", func(t *testing.T) { ownerCanManagePlaybackMarkersThroughAPI(t, fixture) })
	t.Run("OwnerConfigurationAndIdentityAreAvailableThroughAPI", func(t *testing.T) { ownerConfigurationAndIdentityAreAvailableThroughAPI(t, fixture) })
	t.Run("CurationAndBrowseLifecycleAPIRoutes", func(t *testing.T) { curationAndBrowseLifecycleAPIRoutes(t, fixture) })
	t.Run("PlaybackCollaborationAndOperationsAreAvailableThroughAPI", func(t *testing.T) { playbackCollaborationAndOperationsAreAvailableThroughAPI(t, fixture) })
	t.Run("APIRejectsNegativeProgressAndBootstrapsTheViewer", func(t *testing.T) { aPIRejectsNegativeProgressAndBootstrapsTheViewer(t, fixture) })
	t.Run("PlaybackAPIReturnsAutomaticNextEpisode", func(t *testing.T) { playbackAPIReturnsAutomaticNextEpisode(t, fixture) })
	t.Run("OwnerCanBootstrapThroughAPI", func(t *testing.T) { ownerCanBootstrapThroughAPI(t, fixture) })
}

func viewerMediaStateAndCurationAreAvailableThroughAPI(t *testing.T, fixture APIParityFixture) {
	t.Parallel()

	handler, token := fixture.Server(t)
	library := APICall(t, handler, token, http.MethodGet, "/api/v1/library", nil)
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	MustJSON(t, library, &catalog)
	id := catalog.Items[0].ID

	AssertAPICalls(t, handler, token, []APITestCall{
		{Method: http.MethodPut, Path: "/api/v1/items/" + id + "/progress", Body: map[string]any{"seconds": 42}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/items/" + id + "/list", Body: map[string]any{"listed": true}, Status: http.StatusOK},
		{Method: http.MethodPost, Path: "/api/v1/playlists", Body: map[string]any{"name": "Favorites"}, Status: http.StatusCreated},
		{Method: http.MethodPut, Path: "/api/v1/playlists/Favorites/items/" + id, Body: map[string]any{"included": true}, Status: http.StatusOK},
		{Method: http.MethodPost, Path: "/api/v1/collections", Body: map[string]any{"name": "Weekend"}, Status: http.StatusCreated},
		{Method: http.MethodPut, Path: "/api/v1/collections/Weekend/items/" + id, Body: map[string]any{"included": true}, Status: http.StatusOK},
	})

	state := APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id, nil)
	if state.Code != http.StatusOK || !bytes.Contains(state.Body.Bytes(), []byte(`"seconds":42`)) || !bytes.Contains(state.Body.Bytes(), []byte(`"listed":true`)) {
		t.Fatalf("item state = %d %q", state.Code, state.Body.String())
	}
	for _, path := range []string{"/api/v1/playlists/Favorites", "/api/v1/collections/Weekend"} {
		response := APICall(t, handler, token, http.MethodGet, path, nil)
		if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"title":"Arrival"`)) {
			t.Fatalf("%s = %d %q", path, response.Code, response.Body.String())
		}
	}
	collections := APICall(t, handler, token, http.MethodGet, "/api/v1/collections", nil)
	AssertAPIBody(t, collections, http.StatusOK, `"name":"Weekend"`, `"itemCount":1`, `"artworkIds":`)
	playlists := APICall(t, handler, token, http.MethodGet, "/api/v1/playlists", nil)
	AssertAPIBody(t, playlists, http.StatusOK, `"name":"Favorites"`, `"itemCount":1`, `"mode":"Manual"`, `"artworkIds":`)
	listed := APICall(t, handler, token, http.MethodGet, "/api/v1/library?view=list", nil)
	AssertAPIBody(t, listed, http.StatusOK, `"view":"list"`, `"total":1`, `"title":"Arrival"`)
	listedPage := APICall(t, handler, token, http.MethodGet, "/?view=list", nil)
	AssertAPIBody(t, listedPage, http.StatusOK, `aria-current="page" href="/?view=list">My List`, "Arrival", "Titles you have saved for later.")
}

func ownerCanManagePlaybackMarkersThroughAPI(t *testing.T, fixture APIParityFixture) {
	t.Parallel()
	handler, token := fixture.Server(t)
	library := APICall(t, handler, token, http.MethodGet, "/api/v1/library", nil)
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	MustJSON(t, library, &catalog)
	id := catalog.Items[0].ID
	saved := APICall(t, handler, token, http.MethodPut, "/api/v1/items/"+id+"/markers", map[string]any{"type": "recap", "start": 2, "end": 42})
	AssertAPIBody(t, saved, http.StatusOK, `"type":"recap"`, `"start":2`, `"end":42`)
	invalidForm := WebFormCall(t, handler, token, "/markers/"+id, url.Values{"type": {"recap"}, "start": {"not-a-number"}, "end": {"43"}})
	AssertAPIBody(t, invalidForm, http.StatusBadRequest, "playback marker is invalid")
	playback := APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	AssertAPIBody(t, playback, http.StatusOK, `"type":"recap"`, `"start":2`, `"end":42`, `"source":"manual"`)
	updated := WebFormCall(t, handler, token, "/markers/"+id, url.Values{"type": {"recap"}, "start": {"3"}, "end": {"43"}})
	if updated.Code != http.StatusSeeOther {
		t.Fatalf("web update = %d %q", updated.Code, updated.Body.String())
	}
	player := APICall(t, handler, token, http.MethodGet, "/watch/"+id, nil)
	AssertAPIBody(t, player, http.StatusOK, `value="recap" selected`, `name="start" type="number" min="0" step="0.1" value="3"`, `name="end" type="number" min="0" step="0.1" value="43"`, "Remove Recap marker", "Add skip marker")
	removedWeb := WebFormCall(t, handler, token, "/markers/"+id+"/remove", url.Values{"type": {"recap"}})
	if removedWeb.Code != http.StatusSeeOther {
		t.Fatalf("web remove = %d %q", removedWeb.Code, removedWeb.Body.String())
	}
	var afterRemove struct {
		Markers []struct{ Type string } `json:"markers"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id+"/playback", nil), &afterRemove)
	if len(afterRemove.Markers) != 0 {
		t.Fatalf("markers after web remove = %#v", afterRemove.Markers)
	}
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/marker-analysis", nil), http.StatusOK, `"state":`, `"items":`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodPost, "/api/v1/marker-analysis", nil), http.StatusAccepted, `"state":"queued"`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodPut, "/api/v1/items/"+id+"/markers", map[string]any{"type": "recap", "start": 4, "end": 44}), http.StatusOK, `"start":4`, `"end":44`)
	invalid := APICall(t, handler, token, http.MethodDelete, "/api/v1/items/"+id+"/markers/unknown", nil)
	AssertAPIBody(t, invalid, http.StatusBadRequest, `"error"`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id+"/playback", nil), http.StatusOK, `"type":"recap"`, `"start":4`, `"end":44`)
	removed := APICall(t, handler, token, http.MethodDelete, "/api/v1/items/"+id+"/markers/recap", nil)
	if removed.Code != http.StatusNoContent {
		t.Fatalf("remove = %d %q", removed.Code, removed.Body.String())
	}
}

func ownerConfigurationAndIdentityAreAvailableThroughAPI(t *testing.T, fixture APIParityFixture) {
	t.Parallel()

	handler, token := fixture.Server(t)
	AssertAPICalls(t, handler, token, []APITestCall{
		{Method: http.MethodPut, Path: "/api/v1/settings/server", Body: map[string]any{"name": "Cinema"}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/settings/playback", Body: map[string]any{"mode": "compatible", "autoplay": true, "autoSkip": []string{"intro"}, "subtitles": "off"}, Status: http.StatusOK},
		{Method: http.MethodPost, Path: "/api/v1/profiles", Body: map[string]any{"name": "Family", "password": "family-password", "rating": "family", "downloads": true}, Status: http.StatusCreated},
		{Method: http.MethodPost, Path: "/api/v1/tasks/scan", Body: nil, Status: http.StatusNoContent},
	})
	key := APICall(t, handler, token, http.MethodPost, "/api/v1/api-keys", map[string]any{"name": "Automation", "scopes": "library, admin"})
	AssertAPIBody(t, key, http.StatusCreated, `"secret":"ks_`)

	settings := APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil)
	AssertAPIBody(t, settings, http.StatusOK, `"name":"Cinema"`, `"playbackMode":"compatible"`, `"autoplay":true`, `"autoSkip":["intro"]`, `"libraryMonitoring":`, `"sso":false`, `"metadataProvider":"Not configured"`)
	profiles := APICall(t, handler, token, http.MethodGet, "/api/v1/profiles", nil)
	AssertAPIBody(t, profiles, http.StatusOK, `"name":"Family"`)
	if bytes.Contains(profiles.Body.Bytes(), []byte("credential")) {
		t.Fatalf("profiles = %d %q", profiles.Code, profiles.Body.String())
	}
	devices := APICall(t, handler, token, http.MethodGet, "/api/v1/devices", nil)
	AssertAPIBody(t, devices, http.StatusOK, "API test")
}

func curationAndBrowseLifecycleAPIRoutes(t *testing.T, fixture APIParityFixture) {
	t.Parallel()

	handler, token := fixture.Server(t)
	AssertAPICalls(t, handler, token, []APITestCall{
		{Method: http.MethodPost, Path: "/api/v1/playlists", Body: map[string]any{"name": "Queue"}, Status: http.StatusCreated},
		{Method: http.MethodGet, Path: "/api/v1/playlists", Body: nil, Status: http.StatusOK},
		{Method: http.MethodDelete, Path: "/api/v1/playlists/Queue", Body: nil, Status: http.StatusNoContent},
		{Method: http.MethodPost, Path: "/api/v1/collections", Body: map[string]any{"name": "Saga"}, Status: http.StatusCreated},
		{Method: http.MethodGet, Path: "/api/v1/collections", Body: nil, Status: http.StatusOK},
		{Method: http.MethodDelete, Path: "/api/v1/collections/Saga", Body: nil, Status: http.StatusNoContent},
	})
	var albums struct {
		Albums []struct {
			ID string `json:"id"`
		} `json:"albums"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/albums", nil), &albums)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/albums/"+albums.Albums[0].ID, nil), http.StatusOK, `"tracks"`, `"title":"Signal"`)
}

func playbackCollaborationAndOperationsAreAvailableThroughAPI(t *testing.T, fixture APIParityFixture) {
	t.Parallel()

	handler, token := fixture.Server(t)
	library := APICall(t, handler, token, http.MethodGet, "/api/v1/library", nil)
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	MustJSON(t, library, &catalog)
	id := catalog.Items[0].ID

	playback := APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	AssertAPIBody(t, playback, http.StatusOK, `"direct":"/media/`, `"compatible":"/hls/`, `"trickplay":"/trickplay/`)
	created := APICall(t, handler, token, http.MethodPost, "/api/v1/watch-rooms", map[string]any{"media": id, "seconds": 12})
	var room struct {
		ID string `json:"id"`
	}
	MustJSON(t, created, &room)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/watch-rooms/"+room.ID, nil), http.StatusOK, `"seconds":12`, `"leader":true`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodPut, "/api/v1/items/"+id+"/metadata", map[string]any{"title": "API title"}), http.StatusOK, `"title":"API title"`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/remote-access", nil), http.StatusOK, `"directOnly":true`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/shows", nil), http.StatusOK, `"shows"`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/albums", nil), http.StatusOK, `"albums"`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/activity", nil), http.StatusOK, `"audit"`, `"playback"`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/openapi.json", nil), http.StatusOK, `"openapi": "3.1.0"`, `/api/v1/items/{id}/playback`)
	backup := APICall(t, handler, token, http.MethodGet, "/api/v1/backup", nil)
	if backup.Code != http.StatusOK || backup.Header().Get("Content-Type") != "application/gzip" {
		t.Fatalf("backup = %d %v", backup.Code, backup.Header())
	}
}
