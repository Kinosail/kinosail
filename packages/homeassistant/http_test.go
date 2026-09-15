package homeassistant

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func registeredHTTPTest(t *testing.T) (*testState, *Integration[testProfile], func(string, string, string, bool) *httptest.ResponseRecorder) {
	t.Helper()
	state := &testState{
		enabled: true,
		profile: Profile[testProfile]{Source: testProfile{"owner"}, ID: "owner", Name: "Owner", Owner: true},
		items:   map[string]Item{"item": {ID: "item", Path: ".mp4"}, "binary": {ID: "binary", Path: "movie"}},
	}
	integration := newTestIntegration(t, state)
	mux := http.NewServeMux()
	integration.Register(mux, func(next http.Handler) http.Handler { return next }, func(w http.ResponseWriter, _ *http.Request, view Approval) error {
		_, _ = w.Write([]byte(view.RequestID))
		return nil
	})

	call := func(method, target, body string, form bool) *httptest.ResponseRecorder {
		var req *http.Request
		if form {
			req = formRequest(method, target, url.Values{})
		} else {
			req = request(method, target, strings.NewReader(body))
		}
		return response(mux, req)
	}
	return state, integration, call
}

func TestRegisteredPairingAndMediaHTTP(t *testing.T) { //nolint:cyclop,gocognit // One table verifies the registered pairing and media routes.
	state, _, call := registeredHTTPTest(t)
	if got := call(http.MethodGet, "/api/v1/home-assistant", "", false); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"serverId":"server-id"`) {
		t.Fatalf("probe = %d %s", got.Code, got.Body.String())
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/pairings", `{}`, false); got.Code != http.StatusCreated || !strings.Contains(got.Body.String(), `"code":"00000000"`) {
		t.Fatalf("offer = %d %s", got.Code, got.Body.String())
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/pair", `{"code":"00000000","name":"Living room"}`, false); got.Code != http.StatusCreated || !strings.Contains(got.Body.String(), `"token":"token-owner"`) {
		t.Fatalf("pair = %d %s", got.Code, got.Body.String())
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/pair", `{}`, false); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid pair = %d", got.Code)
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/pairings", `{"extra":true}`, false); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid offer = %d", got.Code)
	}

	if got := call(http.MethodGet, "/api/v1/home-assistant/library", "", false); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"view":"all"`) {
		t.Fatalf("library = %d %s", got.Code, got.Body.String())
	}
	state.browseErr = errors.New("browse")
	if got := call(http.MethodGet, "/api/v1/home-assistant/library", "", false); got.Code != http.StatusBadRequest {
		t.Fatalf("library error = %d", got.Code)
	}
	state.browseErr = nil
	if got := call(http.MethodPost, "/api/v1/home-assistant/playback/missing", "", false); got.Code != http.StatusNotFound {
		t.Fatalf("missing playback = %d", got.Code)
	}
	for _, id := range []string{"item", "binary"} {
		got := call(http.MethodPost, "/api/v1/home-assistant/playback/"+id, "", false)
		wantMIME := map[string]string{"item": "video/mp4", "binary": "application/octet-stream"}[id]
		if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"url":"/home-assistant/media/`) || !strings.Contains(got.Body.String(), `"mimeType":"`+wantMIME+`"`) {
			t.Fatalf("playback %s = %d %s", id, got.Code, got.Body.String())
		}
	}
}

func TestRegisteredPlayerHTTP(t *testing.T) { //nolint:cyclop // The score of 14 remains below the repository ceiling of 22 for the player route matrix.
	_, integration, call := registeredHTTPTest(t)
	if got := call(http.MethodGet, "/api/v1/home-assistant/players", "", false); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"players":[]`) {
		t.Fatalf("players = %d %s", got.Code, got.Body.String())
	}
	if got := call(http.MethodPut, "/api/v1/home-assistant/players/player", `{"name":"Player","state":"idle","position":0,"duration":0,"volume":0}`, false); got.Code != http.StatusOK {
		t.Fatalf("player = %d %s", got.Code, got.Body.String())
	}
	if got := call(http.MethodPut, "/api/v1/home-assistant/players/player", `{`, false); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid player JSON = %d", got.Code)
	}
	if got := call(http.MethodPut, "/api/v1/home-assistant/players/oversized", strings.Repeat(" ", 1<<20)+`{}`, false); got.Code != http.StatusBadRequest || len(integration.players) != 1 {
		t.Fatalf("oversized player JSON = %d, players=%d", got.Code, len(integration.players))
	}
	if got := call(http.MethodPut, "/api/v1/home-assistant/players/player", `{"name":"","state":"idle"}`, false); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid player state = %d", got.Code)
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/player/commands", `{`, false); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid command JSON = %d", got.Code)
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/player/commands", `{"command":"bad"}`, false); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid command = %d", got.Code)
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/missing/commands", `{"command":"pause"}`, false); got.Code != http.StatusNotFound {
		t.Fatalf("missing player command = %d", got.Code)
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/player/commands", `{"command":"pause"}`, false); got.Code != http.StatusAccepted {
		t.Fatalf("command = %d %s", got.Code, got.Body.String())
	}
	if got := call(http.MethodPut, "/api/v1/home-assistant/players/player", `{"name":"Player","state":"idle","position":0,"duration":0,"volume":0}`, false); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"command":"pause"`) {
		t.Fatalf("poll = %d %s", got.Code, got.Body.String())
	}
}

func TestRegisteredOAuthAccessAndAvailabilityHTTP(t *testing.T) {
	state, _, call := registeredHTTPTest(t)
	if got := call(http.MethodPost, "/api/v1/home-assistant/token", "", true); got.Code != http.StatusBadRequest {
		t.Fatalf("token route = %d", got.Code)
	}
	if got := call(http.MethodGet, "/home-assistant/authorize", "", false); got.Code != http.StatusBadRequest {
		t.Fatalf("authorize route = %d", got.Code)
	}
	if got := call(http.MethodPost, "/home-assistant/authorize", "", true); got.Code != http.StatusBadRequest {
		t.Fatalf("authorize POST route = %d", got.Code)
	}
	if got := call(http.MethodGet, "/home-assistant/media/item", "", false); got.Code != http.StatusNotFound {
		t.Fatalf("media route = %d", got.Code)
	}

	state.profile.Owner = false
	if got := call(http.MethodGet, "/api/v1/home-assistant/library", "", false); got.Code != http.StatusForbidden {
		t.Fatalf("forbidden control = %d", got.Code)
	}
	state.profile.APIKey = true
	state.profile.Scopes = []string{"home-assistant"}
	if got := call(http.MethodGet, "/api/v1/home-assistant/library", "", false); got.Code != http.StatusOK {
		t.Fatalf("API key control = %d", got.Code)
	}
	state.enabled = false
	if got := call(http.MethodGet, "/api/v1/home-assistant", "", false); got.Code != http.StatusNotFound {
		t.Fatalf("disabled available = %d", got.Code)
	}
	if got := call(http.MethodGet, "/home-assistant/authorize", "", false); got.Code != http.StatusBadRequest {
		t.Fatalf("authorize remains owner guarded, got %d", got.Code)
	}
}

func TestHTTPErrorBranches(t *testing.T) {
	state := &testState{enabled: true, profile: Profile[testProfile]{ID: "owner", Owner: true}}
	integration := newTestIntegration(t, state)
	integration.config.Random = failingReader{errors.New("entropy")}
	if got := response(http.HandlerFunc(integration.offerHTTP), request(http.MethodPost, "/", strings.NewReader(`{}`))); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("offer error = %d", got.Code)
	}
	if got := response(http.HandlerFunc(integration.pairHTTP), request(http.MethodPost, "/", strings.NewReader(`{`))); got.Code != http.StatusBadRequest {
		t.Fatalf("pair JSON = %d", got.Code)
	}
	if got := response(http.HandlerFunc(integration.pairHTTP), request(http.MethodPost, "/", strings.NewReader(`{"code":"bad"}`))); got.Code != http.StatusBadRequest {
		t.Fatalf("pair input = %d", got.Code)
	}

	integration.players = make(map[string]playerRecord)
	for index := range maxPlayers {
		id := strconv.Itoa(index)
		integration.players[id] = playerRecord{Player: Player{ID: id}, Seen: state.now}
	}
	req := request(http.MethodPut, "/", strings.NewReader(`{"name":"Player","state":"idle","position":0,"duration":0,"volume":0}`))
	req.SetPathValue("id", "limit")
	if got := response(http.HandlerFunc(integration.playerStateHTTP), req); got.Code != http.StatusTooManyRequests {
		t.Fatalf("player limit HTTP = %d", got.Code)
	}

	state.createErr = errors.New("create")
	integration.config.Random = bytes.NewReader(make([]byte, 32))
	code, err := integration.offer(state.profile)
	if err != nil {
		t.Fatal(err)
	}
	if got := response(http.HandlerFunc(integration.pairHTTP), request(http.MethodPost, "/", strings.NewReader(`{"code":"`+code+`","name":"name"}`))); got.Code != http.StatusBadRequest {
		t.Fatalf("pair create error = %d", got.Code)
	}
}
