package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestViewingImportAPIAndWebRejectSameInvalidInput(t *testing.T) {
	handler, token := apiServer(t)
	apiResponse := apiCall(t, handler, token, http.MethodPost, "/api/v1/viewing-imports/preview", map[string]any{"source": "unknown"})
	form := url.Values{"source": {"unknown"}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/viewing-imports/preview", strings.NewReader(form.Encode()))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	webResponse := httptest.NewRecorder()
	handler.ServeHTTP(webResponse, request)
	if apiResponse.Code != http.StatusBadRequest || webResponse.Code != http.StatusBadRequest || !strings.Contains(apiResponse.Body.String(), "source must be plex or jellyfin") || !strings.Contains(webResponse.Body.String(), "source must be plex or jellyfin") {
		t.Fatalf("API = %d %q, web = %d %q", apiResponse.Code, apiResponse.Body.String(), webResponse.Code, webResponse.Body.String())
	}
}

func TestViewingActivityAPIAndWebUseTheSameImportOperation(t *testing.T) { //nolint:cyclop,funlen,gocognit // One adapter-parity scenario checks the full preview and apply journey.
	const sourceToken = "source-token-that-must-stay-secret"
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Emby-Token") != sourceToken {
			http.Error(writer, "missing token", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/Users/Me":
			_, _ = writer.Write([]byte(`{"Id":"viewer"}`))
		case "/Items":
			if request.URL.Query().Get("IncludeItemTypes") == "Playlist" {
				_, _ = writer.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"sci-fi","Name":"Sci-Fi","Type":"Playlist"}]}`))
				return
			}
			_, _ = writer.Write([]byte(`{"TotalRecordCount":2,"Items":[{"Id":"arrival","Name":"Arrival","Type":"Movie","ProductionYear":2016,"Path":"/source/Arrival.mp4","UserData":{"Played":true,"IsFavorite":true,"LastPlayedDate":"2026-08-22T12:00:00Z"}},{"Id":"severance","Name":"Good News","Type":"Episode","SeriesName":"Severance","ParentIndexNumber":1,"IndexNumber":1,"UserData":{}}]}`))
		case "/Playlists/sci-fi/Items":
			_, _ = writer.Write([]byte(`{"TotalRecordCount":2,"Items":[{"Id":"severance"},{"Id":"arrival"}]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer upstream.Close()

	handler, token := apiServer(t)
	profilesResponse := apiCall(t, handler, token, http.MethodGet, "/api/v1/profiles", nil)
	var profiles struct {
		Profiles []struct {
			ID string `json:"id"`
		} `json:"profiles"`
	}
	mustJSON(t, profilesResponse, &profiles)
	if len(profiles.Profiles) != 1 {
		t.Fatalf("profiles = %#v", profiles.Profiles)
	}
	input := map[string]any{"source": "jellyfin", "url": upstream.URL, "token": sourceToken, "profileId": profiles.Profiles[0].ID}
	previewResponse := apiCall(t, handler, token, http.MethodPost, "/api/v1/viewing-imports/preview", input)
	if previewResponse.Code != http.StatusOK || strings.Contains(previewResponse.Body.String(), sourceToken) {
		t.Fatalf("API preview = %d %q", previewResponse.Code, previewResponse.Body.String())
	}
	var preview struct {
		ID      string `json:"id"`
		Summary struct {
			Importable int `json:"importable"`
			Favorites  int `json:"favorites"`
		} `json:"summary"`
	}
	mustJSON(t, previewResponse, &preview)
	if preview.ID == "" || preview.Summary.Importable != 2 || preview.Summary.Favorites != 1 {
		t.Fatalf("preview = %#v: %s", preview, previewResponse.Body.String())
	}
	apply := apiCall(t, handler, token, http.MethodPost, "/api/v1/viewing-imports/"+preview.ID+"/apply", nil)
	assertAPIBody(t, apply, http.StatusOK, `"applied":1`, `"listsApplied":3`)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/library", nil), http.StatusOK, `"title":"Arrival"`, `"watched":true`)
	playlist := apiCall(t, handler, token, http.MethodGet, "/api/v1/playlists/Sci-Fi", nil)
	if playlist.Code != http.StatusOK || strings.Index(playlist.Body.String(), "Good News") > strings.Index(playlist.Body.String(), "Arrival") {
		t.Fatalf("imported playlist order = %d %q", playlist.Code, playlist.Body.String())
	}

	settingsRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil)
	settingsRequest.Header.Set("Authorization", "Bearer "+token)
	settingsResponse := httptest.NewRecorder()
	handler.ServeHTTP(settingsResponse, settingsRequest)
	if settingsResponse.Code != http.StatusOK || !strings.Contains(settingsResponse.Body.String(), `id="viewing-imports"`) || !strings.Contains(settingsResponse.Body.String(), "never writes back") {
		t.Fatalf("settings = %d %q", settingsResponse.Code, settingsResponse.Body.String())
	}

	form := url.Values{"source": {"jellyfin"}, "url": {upstream.URL}, "token": {sourceToken}, "profileId": {profiles.Profiles[0].ID}}
	webRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/viewing-imports/preview", strings.NewReader(form.Encode()))
	webRequest.Header.Set("Authorization", "Bearer "+token)
	webRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	webResponse := httptest.NewRecorder()
	handler.ServeHTTP(webResponse, webRequest)
	if webResponse.Code != http.StatusOK || !strings.Contains(webResponse.Body.String(), "Import and sync automatically") || strings.Contains(webResponse.Body.String(), sourceToken) {
		t.Fatalf("web preview = %d %q", webResponse.Code, webResponse.Body.String())
	}
	match := regexp.MustCompile(`name="id" value="([^"]+)"`).FindStringSubmatch(webResponse.Body.String())
	if len(match) != 2 {
		t.Fatalf("web preview has no confirmation ID: %q", webResponse.Body.String())
	}
	webSyncForm := url.Values{"id": {match[1]}, "interval": {"1h"}}
	webSyncRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/viewing-syncs", strings.NewReader(webSyncForm.Encode()))
	webSyncRequest.Header.Set("Authorization", "Bearer "+token)
	webSyncRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	webSyncResponse := httptest.NewRecorder()
	handler.ServeHTTP(webSyncResponse, webSyncRequest)
	if webSyncResponse.Code != http.StatusSeeOther {
		t.Fatalf("web sync create = %d %q", webSyncResponse.Code, webSyncResponse.Body.String())
	}

	apiSyncPreview := apiCall(t, handler, token, http.MethodPost, "/api/v1/viewing-imports/preview", input)
	mustJSON(t, apiSyncPreview, &preview)
	apiSync := apiCall(t, handler, token, http.MethodPost, "/api/v1/viewing-syncs", map[string]any{"previewId": preview.ID, "interval": "6h"})
	if apiSync.Code != http.StatusCreated || strings.Contains(apiSync.Body.String(), sourceToken) {
		t.Fatalf("API sync create = %d %q", apiSync.Code, apiSync.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	mustJSON(t, apiSync, &created)
	listed := apiCall(t, handler, token, http.MethodGet, "/api/v1/viewing-syncs", nil)
	if listed.Code != http.StatusOK || strings.Count(listed.Body.String(), `"profileId"`) != 2 || strings.Contains(listed.Body.String(), sourceToken) {
		t.Fatalf("API sync list = %d %q", listed.Code, listed.Body.String())
	}
	if pulled := apiCall(t, handler, token, http.MethodPost, "/api/v1/viewing-syncs/"+created.ID+"/run", nil); pulled.Code != http.StatusOK {
		t.Fatalf("API sync run = %d %q", pulled.Code, pulled.Body.String())
	}
	if removed := apiCall(t, handler, token, http.MethodDelete, "/api/v1/viewing-syncs/"+created.ID, nil); removed.Code != http.StatusNoContent {
		t.Fatalf("API sync delete = %d %q", removed.Code, removed.Body.String())
	}
}
