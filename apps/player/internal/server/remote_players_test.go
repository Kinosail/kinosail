package server_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestRemotePlayerStateAndCommands(t *testing.T) {
	t.Parallel()
	handler, owner := apiServer(t)
	const id = "a7d345e0-12ab-4cde-8123-123456789abc"
	path := "/api/v1/remote-players/" + id
	state := map[string]any{"name": "Living Room", "title": "Scary Movie", "artist": "", "itemId": "movie-1", "state": "playing", "position": 120, "duration": 3600, "audio": false}
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/remote-players", nil), http.StatusUnauthorized)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, path, state), http.StatusOK, `"command":null`)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodGet, "/api/v1/remote-players", nil), http.StatusOK, `"title":"Scary Movie"`)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPost, path+"/commands", map[string]any{"command": "pause", "itemId": "movie-1"}), http.StatusAccepted)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, path, state), http.StatusOK, `"command":"pause"`)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, path, state), http.StatusOK, `"command":null`)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPost, path+"/commands", map[string]any{"command": "pause", "itemId": "movie-1"}), http.StatusAccepted)
	state["itemId"] = "movie-2"
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, path, state), http.StatusOK, `"command":null`)
}

func TestRemotePlayerRejectsBadInputsWithoutChangingState(t *testing.T) {
	t.Parallel()
	handler, owner := apiServer(t)
	const id = "a7d345e0-12ab-4cde-8123-123456789abc"
	path := "/api/v1/remote-players/" + id
	good := `{"name":"Living Room","title":"Film","artist":"","itemId":"movie-1","state":"paused","position":12,"duration":120,"audio":false}`
	assertAPIBody(t, rawAPIRequest(t, handler, owner, http.MethodPut, path, good), http.StatusOK)
	for _, body := range []string{
		`{}`, `{"name":"Living Room","state":"unknown"}`,
		strings.Replace(good, `"position":12`, `"position":121`, 1),
		strings.Replace(good, `"title":"Film"`, `"title":"`+strings.Repeat("x", 257)+`"`, 1),
		strings.Replace(good, `"audio":false`, `"audio":false,"extra":true`, 1),
		strings.Replace(good, `"audio":false`, `"audio":false,"id":""`, 1),
		strings.Replace(good, `"name":"Living Room"`, `"name":"  "`, 1),
		strings.Replace(good, `"itemId":"movie-1"`, `"itemId":"movie/1"`, 1),
		strings.Replace(good, `"title":"Film"`, `"title":"Film\nTwo"`, 1),
		strings.Replace(good, `"position":12`, `"position":12,"position":13`, 1),
	} {
		assertAPIBody(t, rawAPIRequest(t, handler, owner, http.MethodPut, path, body), http.StatusBadRequest)
	}
	for _, body := range []string{
		`{}`, `{"command":"pause"}`, `{"command":"pause","itemId":"other"}`,
		`{"command":"launch","itemId":"movie-1"}`, `{"command":"next","itemId":"movie-1"}`, `{"command":"seek","itemId":"movie-1","position":121}`,
		`{"command":"pause","itemId":"movie-1","position":1}`, `{"command":"pause","itemId":"movie-1","position":0}`,
		`{"command":"seek","itemId":"movie-1"}`, `{"command":"pause","itemId":"movie-1","extra":true}`,
	} {
		assertAPIBody(t, rawAPIRequest(t, handler, owner, http.MethodPost, path+"/commands", body), http.StatusBadRequest)
	}
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, path, map[string]any{"name": "Living Room", "title": "Film", "artist": "", "itemId": "movie-1", "state": "paused", "position": 12, "duration": 120, "audio": false}), http.StatusOK, `"command":null`)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodGet, "/api/v1/remote-players", nil), http.StatusOK, `"position":12`)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPost, "/api/v1/remote-players/bad/commands", map[string]any{"command": "pause", "itemId": "movie-1"}), http.StatusBadRequest)
}
