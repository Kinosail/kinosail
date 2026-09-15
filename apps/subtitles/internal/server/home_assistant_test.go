package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestHomeAssistantIsDarkByDefaultAndAppearsInTheExistingWizard(t *testing.T) {
	libraryAPIFixture.HomeAssistantIsDarkByDefault(t)
}

func TestHomeAssistantBrowserApprovalUsesPKCEAndCreatesOneScopedToken(t *testing.T) {
	libraryAPIFixture.HomeAssistantBrowserApproval(t)
}

func TestOwnerCanEnablePairAndRevokeHomeAssistant(t *testing.T) { //nolint:cyclop // One lifecycle proves the feature gate and credential revocation together.
	t.Parallel()
	handler, owner := apiServer(t)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": true}), http.StatusOK, `"status":"saved"`)
	for name, response := range map[string]*httptest.ResponseRecorder{"settings": apiCall(t, handler, owner, http.MethodGet, "/settings", nil), "wizard": apiCall(t, handler, owner, http.MethodGet, "/onboarding/connection", nil)} {
		body := response.Body.String()
		if !strings.Contains(body, `href="https://my.home-assistant.io/redirect/config_flow_start/?domain=kinosail"`) || !strings.Contains(body, "Local discovery") || !strings.Contains(body, "Connect Home Assistant") || !strings.Contains(body, "Return here.") || !strings.Contains(body, "Manual pairing") {
			t.Fatalf("%s Home Assistant setup actions = %d %q", name, response.Code, body)
		}
	}
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/home-assistant", nil), http.StatusOK, `"name":"Kinosail"`, `"serverId":`)
	remoteRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/home-assistant", nil)
	remoteResponse := httptest.NewRecorder()
	server.Remote(handler).ServeHTTP(remoteResponse, remoteRequest)
	if remoteResponse.Code != http.StatusNotFound {
		t.Fatalf("remote Home Assistant probe = %d %q", remoteResponse.Code, remoteResponse.Body.String())
	}

	pairing := apiCall(t, handler, owner, http.MethodPost, "/api/v1/home-assistant/pairings", map[string]any{})
	var offered struct {
		Code string `json:"code"`
	}
	mustJSON(t, pairing, &offered)
	if pairing.Code != http.StatusCreated || len(offered.Code) != 8 {
		t.Fatalf("pairing offer = %d %#v", pairing.Code, offered)
	}
	pair := apiCall(t, handler, "", http.MethodPost, "/api/v1/home-assistant/pair", map[string]any{"code": offered.Code, "name": "Home"})
	var credential struct {
		Token string `json:"token"`
	}
	mustJSON(t, pair, &credential)
	if pair.Code != http.StatusCreated || !strings.HasPrefix(credential.Token, "ks_") {
		t.Fatalf("pairing result = %d %#v", pair.Code, credential)
	}
	assertAPIBody(t, apiCall(t, handler, "", http.MethodPost, "/api/v1/home-assistant/pair", map[string]any{"code": offered.Code, "name": "Second"}), http.StatusBadRequest, `"error"`)
	assertAPIBody(t, apiCall(t, handler, credential.Token, http.MethodGet, "/api/v1/home-assistant/library", nil), http.StatusOK, `"items"`)
	assertAPIBody(t, apiCall(t, handler, credential.Token, http.MethodGet, "/api/v1/settings", nil), http.StatusForbidden, `"error"`)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": true}), http.StatusOK, `"status":"saved"`)
	assertAPIBody(t, apiCall(t, handler, credential.Token, http.MethodGet, "/api/v1/home-assistant/library", nil), http.StatusOK, `"items"`)

	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": false}), http.StatusOK, `"status":"saved"`)
	assertAPIBody(t, apiCall(t, handler, credential.Token, http.MethodGet, "/api/v1/home-assistant/library", nil), http.StatusNotFound, `"error":"not found"`)
	assertAPIBody(t, apiCall(t, handler, credential.Token, http.MethodGet, "/api/v1/library", nil), http.StatusUnauthorized, `"error"`)
}

func TestHomeAssistantPairingRejectsBadInputWithoutCreatingKeys(t *testing.T) {
	t.Parallel()
	handler, owner := apiServer(t)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": true}), http.StatusOK, `"status":"saved"`)
	before := apiCall(t, handler, owner, http.MethodGet, "/api/v1/api-keys", nil).Body.String()
	for _, body := range []string{
		`{}`,
		`{"code":"123","name":"Home"}`,
		`{"code":"00000000","name":"Home"}`,
		`{"code":"00000000","name":"` + strings.Repeat("x", 81) + `"}`,
		`{"code":"00000000","name":"Home","extra":true}`,
	} {
		response := rawAPIRequest(t, handler, "", http.MethodPost, "/api/v1/home-assistant/pair", body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid pairing %s = %d %q", body, response.Code, response.Body.String())
		}
	}
	after := apiCall(t, handler, owner, http.MethodGet, "/api/v1/api-keys", nil).Body.String()
	if before != after {
		t.Fatalf("invalid pairing changed keys\nbefore: %s\nafter: %s", before, after)
	}
}

func TestHomeAssistantPlayerCommandsAndDirectCapabilitiesAreBounded(t *testing.T) { //nolint:cyclop // State, command, and stream capabilities form one control flow.
	t.Parallel()
	handler, owner := apiServer(t)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": true}), http.StatusOK, `"status":"saved"`)
	pairing := apiCall(t, handler, owner, http.MethodPost, "/api/v1/home-assistant/pairings", map[string]any{})
	var offered struct {
		Code string `json:"code"`
	}
	mustJSON(t, pairing, &offered)
	pair := apiCall(t, handler, "", http.MethodPost, "/api/v1/home-assistant/pair", map[string]any{"code": offered.Code, "name": "Home"})
	var credential struct {
		Token string `json:"token"`
	}
	mustJSON(t, pair, &credential)

	state := map[string]any{"name": "Living Room", "state": "playing", "title": "Arrival", "itemId": "item", "position": 12.5, "duration": 90.0, "volume": 0.5, "muted": false}
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/home-assistant/players/browser-1", state), http.StatusOK, `"command":null`)
	assertAPIBody(t, apiCall(t, handler, credential.Token, http.MethodGet, "/api/v1/home-assistant/players", nil), http.StatusOK, `"id":"browser-1"`, `"name":"Living Room"`)
	assertAPIBody(t, apiCall(t, handler, credential.Token, http.MethodPost, "/api/v1/home-assistant/players/browser-1/commands", map[string]any{"command": "pause"}), http.StatusAccepted, `"status":"queued"`)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/home-assistant/players/browser-1", state), http.StatusOK, `"command":"pause"`)
	for _, body := range []any{
		map[string]any{"command": "unknown"},
		map[string]any{"command": "seek", "position": -1},
		map[string]any{"command": "volume", "volume": 2},
		map[string]any{"command": "play_media", "itemId": strings.Repeat("x", 129)},
	} {
		response := apiCall(t, handler, credential.Token, http.MethodPost, "/api/v1/home-assistant/players/browser-1/commands", body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid command %#v = %d %q", body, response.Code, response.Body.String())
		}
	}

	library := apiCall(t, handler, credential.Token, http.MethodGet, "/api/v1/home-assistant/library", nil)
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	mustJSON(t, library, &catalog)
	resolved := apiCall(t, handler, credential.Token, http.MethodPost, "/api/v1/home-assistant/playback/"+catalog.Items[0].ID, map[string]any{})
	var playback struct {
		URL string `json:"url"`
	}
	mustJSON(t, resolved, &playback)
	if resolved.Code != http.StatusOK || !strings.HasPrefix(playback.URL, "/home-assistant/media/") {
		t.Fatalf("playback capability = %d %#v", resolved.Code, playback)
	}
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, playback.URL, nil), http.StatusOK)
	tampered := playback.URL[:strings.LastIndex(playback.URL, "=")+1] + "invalid"
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, tampered, nil), http.StatusNotFound)
	for _, ambiguous := range []string{
		playback.URL + "&extra=true",
		playback.URL + "&expires=1",
		playback.URL + "&signature=duplicate",
	} {
		assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, ambiguous, nil), http.StatusNotFound)
	}
}

func TestHomeAssistantWebSettingUsesTheSharedOperation(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	handler := server.New(server.Config{DataDir: dataDir})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/home-assistant", strings.NewReader("enabled=true"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("enable Home Assistant = %d %q", response.Code, response.Body.String())
	}
	handler = server.New(server.Config{DataDir: dataDir})
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/home-assistant", nil), http.StatusOK, `"name":"Kinosail"`)
}

func TestHomeAssistantWebSettingRejectsAmbiguousInputWithoutChangingState(t *testing.T) {
	t.Parallel()
	for name, request := range map[string]*http.Request{
		"wrong content type": httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/home-assistant", strings.NewReader("enabled=true")),
		"query":              httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/home-assistant?enabled=true", strings.NewReader("")),
		"unknown field":      httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/home-assistant", strings.NewReader("enabled=true&extra=true")),
		"duplicate":          httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/home-assistant", strings.NewReader("enabled=true&enabled=true")),
	} {
		t.Run(name, func(t *testing.T) {
			handler := server.New(server.Config{DataDir: t.TempDir()})
			if name != "wrong content type" {
				request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid Home Assistant setting = %d %q", response.Code, response.Body.String())
			}
			assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/home-assistant", nil), http.StatusNotFound)
		})
	}
}

func TestHomeAssistantPlayerInputAndCardinalityAreBounded(t *testing.T) {
	t.Parallel()
	handler, owner := apiServer(t)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": true}), http.StatusOK, `"status":"saved"`)
	valid := map[string]any{"name": " Room ", "state": "idle", "position": 0, "duration": 0, "volume": 1, "muted": false}
	for _, path := range []string{"bad%20id", strings.Repeat("x", 65)} {
		assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/home-assistant/players/"+path, valid), http.StatusBadRequest, `"error"`)
	}
	for _, state := range []map[string]any{
		{"name": "", "state": "idle", "position": 0, "duration": 0, "volume": 1},
		{"name": "Room", "state": "unknown", "position": 0, "duration": 0, "volume": 1},
		{"name": "Room", "state": "idle", "position": -1, "duration": 0, "volume": 1},
		{"name": strings.Repeat("x", 81), "state": "idle", "position": 0, "duration": 0, "volume": 1},
	} {
		assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/home-assistant/players/rejected", state), http.StatusBadRequest, `"error"`)
	}
	for index := range 64 {
		assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, fmt.Sprintf("/api/v1/home-assistant/players/browser-%d", index), valid), http.StatusOK, `"command":null`)
	}
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/home-assistant/players/overflow", valid), http.StatusTooManyRequests, `"error"`)
	players := apiCall(t, handler, owner, http.MethodGet, "/api/v1/home-assistant/players", nil)
	if strings.Count(players.Body.String(), `"id":"browser-`) != 64 || strings.Contains(players.Body.String(), `"id":"overflow"`) || !strings.Contains(players.Body.String(), `"name":"Room"`) {
		t.Fatalf("bounded players = %d %q", players.Code, players.Body.String())
	}
}
