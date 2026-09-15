package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestProgressRejectsStaleEventsWithinAPlaybackSession(t *testing.T) {
	t.Parallel()
	handler, token := apiServer(t)
	library := apiCall(t, handler, token, http.MethodGet, "/api/v1/library", nil)
	var catalog struct {
		Items []struct{ ID string }
	}
	if err := json.Unmarshal(library.Body.Bytes(), &catalog); err != nil || len(catalog.Items) == 0 {
		t.Fatalf("library = %d %q: %v", library.Code, library.Body.String(), err)
	}
	path := "/api/v1/items/" + catalog.Items[0].ID + "/progress"
	newer := apiCall(t, handler, token, http.MethodPut, path, map[string]any{"seconds": 120, "session": "phone-playback", "revision": 2})
	stale := apiCall(t, handler, token, http.MethodPut, path, map[string]any{"seconds": 10, "session": "phone-playback", "revision": 1})
	item := apiCall(t, handler, token, http.MethodGet, "/api/v1/items/"+catalog.Items[0].ID, nil)
	if newer.Code != http.StatusOK || stale.Code != http.StatusConflict || !json.Valid(stale.Body.Bytes()) || !containsJSON(item.Body.String(), `"seconds":120`, `"revision":2`) {
		t.Fatalf("newer=%d %q stale=%d %q item=%d %q", newer.Code, newer.Body.String(), stale.Code, stale.Body.String(), item.Code, item.Body.String())
	}
}

func containsJSON(value string, expected ...string) bool {
	for _, part := range expected {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
