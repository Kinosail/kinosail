package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestViewerCannotReachContentOutsideGrantedLibraries(t *testing.T) { //nolint:cyclop,funlen // One known item is probed through every public content adapter.
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	setupPolicyMedia(t, mediaDir, dataDir)
	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")

	catalog := apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/library", nil)
	var library struct {
		Items []struct {
			ID, Title string
		} `json:"items"`
	}
	mustJSON(t, catalog, &library)
	ids := make(map[string]string, len(library.Items))
	for _, item := range library.Items {
		ids[item.Title] = item.ID
	}
	restrictedID := ids["S01E01"]
	if ids["Arrival"] == "" || restrictedID == "" {
		t.Fatalf("fixture library = %q", catalog.Body.String())
	}

	created := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/profiles", map[string]any{
		"name": "Sam", "password": "viewer-password", "rating": "all", "libraries": []string{"Movies"},
		"downloads": true, "transcode": true, "remote": true,
	})
	var viewer struct {
		ID string `json:"id"`
	}
	mustJSON(t, created, &viewer)
	login := apiCall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]string{"name": "Sam", "password": "viewer-password"})
	var session struct {
		Token string `json:"token"`
	}
	mustJSON(t, login, &session)
	enrollTestAPIFactor(t, handler, session.Token)

	for _, path := range []string{"/", "/api/v1/library", "/Items", "/Items/Latest"} {
		response := apiCall(t, handler, session.Token, http.MethodGet, path, nil)
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), restrictedID) || strings.Contains(response.Body.String(), "Pilot") {
			t.Fatalf("restricted browse %s = %d %q", path, response.Code, response.Body.String())
		}
	}

	cases := []struct {
		method, path string
		body         any
		status       int
	}{
		{http.MethodGet, "/watch/" + restrictedID, nil, http.StatusNotFound},
		{http.MethodGet, "/media/" + restrictedID, nil, http.StatusNotFound},
		{http.MethodGet, "/download/" + restrictedID, nil, http.StatusNotFound},
		{http.MethodGet, "/api/v1/items/" + restrictedID, nil, http.StatusNotFound},
		{http.MethodGet, "/api/v1/items/" + restrictedID + "/playback", nil, http.StatusNotFound},
		{http.MethodPut, "/api/v1/items/" + restrictedID + "/progress", map[string]any{"seconds": 30}, http.StatusNotFound},
		{http.MethodPut, "/api/v1/items/" + restrictedID + "/list", map[string]any{"listed": true}, http.StatusNotFound},
		{http.MethodPost, "/api/v1/items/" + restrictedID + "/downloads", map[string]any{"quality": "original"}, http.StatusNotFound},
		{http.MethodGet, "/Items/" + restrictedID, nil, http.StatusNotFound},
		{http.MethodGet, "/Items/" + restrictedID + "/PlaybackInfo", nil, http.StatusNotFound},
		{http.MethodPost, "/Items/" + restrictedID + "/PlaybackInfo", map[string]any{}, http.StatusNotFound},
		{http.MethodGet, "/Items/" + restrictedID + "/File", nil, http.StatusNotFound},
		{http.MethodGet, "/Items/" + restrictedID + "/Download", nil, http.StatusNotFound},
		{http.MethodGet, "/UserItems/" + restrictedID + "/UserData", nil, http.StatusNotFound},
		{http.MethodPost, "/UserFavoriteItems/" + restrictedID, nil, http.StatusNotFound},
		{http.MethodPost, "/Users/" + viewer.ID + "/PlayedItems/" + restrictedID, nil, http.StatusNotFound},
		{http.MethodPost, "/Sessions/Playing/Progress", map[string]any{"ItemId": restrictedID, "PositionTicks": 300000000}, http.StatusBadRequest},
		{http.MethodGet, "/MediaSegments/" + restrictedID, nil, http.StatusNotFound},
	}
	for _, test := range cases {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response := apiCall(t, handler, session.Token, test.method, test.path, test.body)
			if response.Code != test.status || strings.Contains(response.Body.String(), "Pilot") {
				t.Fatalf("restricted content = %d %q, want %d", response.Code, response.Body.String(), test.status)
			}
		})
	}

	grant := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/profiles/"+viewer.ID, map[string]any{
		"rating": "all", "libraries": []string{"Movies", "Shows"}, "downloads": true, "transcode": true, "remote": true,
	})
	if grant.Code != http.StatusOK {
		t.Fatalf("grant second library = %d %q", grant.Code, grant.Body.String())
	}
	userData := apiCall(t, handler, session.Token, http.MethodGet, "/UserItems/"+restrictedID+"/UserData", nil)
	var state struct {
		PlaybackPositionTicks int64
		IsFavorite            bool
	}
	if json.Unmarshal(userData.Body.Bytes(), &state) != nil || userData.Code != http.StatusOK || state.PlaybackPositionTicks != 0 || state.IsFavorite {
		t.Fatalf("rejected writes changed hidden item state = %d %q", userData.Code, userData.Body.String())
	}
}

func TestViewerCapabilitiesGateEveryContentAdapter(t *testing.T) { //nolint:cyclop,funlen,gocognit // The same least-privilege profile is checked through web, API, and Jellyfin routes.
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	setupPolicyMedia(t, mediaDir, dataDir)
	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")

	var catalog struct {
		Items []struct {
			ID, Title string
		} `json:"items"`
	}
	mustJSON(t, apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/library", nil), &catalog)
	itemID := ""
	for _, item := range catalog.Items {
		if item.Title == "Arrival" {
			itemID = item.ID
		}
	}
	if itemID == "" {
		t.Fatalf("fixture library = %+v", catalog.Items)
	}

	created := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/profiles", map[string]any{
		"name": "Least Privilege", "password": "viewer-password", "rating": "all", "libraries": []string{"Movies"},
	})
	var viewer struct {
		ID string `json:"id"`
	}
	mustJSON(t, created, &viewer)
	login := apiCall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]string{"name": "Least Privilege", "password": "viewer-password"})
	var session struct {
		Token string `json:"token"`
	}
	mustJSON(t, login, &session)
	enrollTestAPIFactor(t, handler, session.Token)
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/me", nil), http.StatusOK,
		`"downloads":false`, `"transcode":false`, `"remote":false`)

	denied := []struct {
		method, path string
		body         any
		status       int
	}{
		{http.MethodGet, "/download/" + itemID, nil, http.StatusForbidden},
		{http.MethodGet, "/offline-downloads", nil, http.StatusForbidden},
		{http.MethodPost, "/offline/" + itemID, map[string]any{"quality": "original"}, http.StatusNotFound},
		{http.MethodGet, "/api/v1/downloads", nil, http.StatusForbidden},
		{http.MethodPost, "/api/v1/items/" + itemID + "/downloads", map[string]any{"quality": "original"}, http.StatusNotFound},
		{http.MethodGet, "/Items/" + itemID + "/Download", nil, http.StatusForbidden},
		{http.MethodGet, "/hls/" + itemID + "/index.m3u8", nil, http.StatusForbidden},
		{http.MethodGet, "/Videos/" + itemID + "/master.m3u8", nil, http.StatusForbidden},
	}
	for _, test := range denied {
		t.Run("denied "+test.path, func(t *testing.T) {
			response := apiCall(t, handler, session.Token, test.method, test.path, test.body)
			if response.Code != test.status {
				t.Fatalf("least-privilege route = %d %q, want %d", response.Code, response.Body.String(), test.status)
			}
		})
	}

	playback := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/items/"+itemID+"/playback", nil)
	if playback.Code != http.StatusOK || !strings.Contains(playback.Body.String(), `"directAllowed":true`) || strings.Contains(playback.Body.String(), `"compatible"`) || strings.Contains(playback.Body.String(), `"download"`) {
		t.Fatalf("least-privilege playback contract = %d %q", playback.Code, playback.Body.String())
	}
	jellyfin := apiCall(t, handler, session.Token, http.MethodGet, "/Items/"+itemID+"/PlaybackInfo", nil)
	if jellyfin.Code != http.StatusOK || !strings.Contains(jellyfin.Body.String(), `"SupportsTranscoding":false`) {
		t.Fatalf("least-privilege Jellyfin playback = %d %q", jellyfin.Code, jellyfin.Body.String())
	}

	grant := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/profiles/"+viewer.ID, map[string]any{
		"rating": "all", "libraries": []string{"Movies"}, "downloads": true, "transcode": true, "remote": true,
	})
	if grant.Code != http.StatusOK {
		t.Fatalf("grant Viewer capabilities = %d %q", grant.Code, grant.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/me", nil), http.StatusOK,
		`"downloads":true`, `"transcode":true`, `"remote":true`)

	for _, test := range denied {
		t.Run("granted "+test.path, func(t *testing.T) {
			response := apiCall(t, handler, session.Token, test.method, test.path, test.body)
			if response.Code == http.StatusForbidden {
				t.Fatalf("granted capability remained denied: %d %q", response.Code, response.Body.String())
			}
		})
	}
}
