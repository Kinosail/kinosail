package servertest

import (
	"net/http"
	"strings"
	"testing"
)

// AgentCanCreateOrderedPlaylist verifies ordering and rejection without side effects.
func (fixture LibraryAPIFixture) AgentCanCreateOrderedPlaylist(t *testing.T) {
	t.Parallel()
	handler, token := fixture.Server(t)
	var catalog struct {
		Items []struct{ ID, Title string } `json:"items"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/library", nil), &catalog)
	ids := make(map[string]string)
	for _, item := range catalog.Items {
		ids[item.Title] = item.ID
	}
	episode := ""
	for title, id := range ids {
		if strings.Contains(title, "Good News") {
			episode = id
		}
	}
	created := APICall(t, handler, token, http.MethodPost, "/api/v1/playlists", map[string]any{"name": "Halloween", "ids": []string{episode, ids["Arrival"]}})
	AssertAPIBody(t, created, http.StatusCreated, `"name":"Halloween"`, episode, ids["Arrival"])
	var playlist struct {
		Items []struct{ ID string } `json:"items"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/playlists/Halloween", nil), &playlist)
	if len(playlist.Items) != 2 || playlist.Items[0].ID != episode || playlist.Items[1].ID != ids["Arrival"] {
		t.Fatalf("ordered playlist = %#v", playlist.Items)
	}
	AssertAPIBody(t, APICall(t, handler, token, http.MethodPost, "/api/v1/playlists", map[string]any{"name": "Unsafe", "ids": []string{"missing"}}), http.StatusBadRequest, "playlist item is unavailable")
	AssertAPIBody(t, APICall(t, handler, token, http.MethodPost, "/api/v1/playlists", map[string]any{"name": "Too many", "ids": make([]string, 201)}), http.StatusBadRequest, "at most 200")
	AssertAPIBody(t, APICall(t, handler, token, http.MethodPost, "/api/v1/playlists", map[string]any{"name": "Long ID", "ids": []string{strings.Repeat("x", 257)}}), http.StatusBadRequest, "1 to 256")
	listed := APICall(t, handler, token, http.MethodGet, "/api/v1/playlists", nil)
	if strings.Contains(listed.Body.String(), "Too many") || strings.Contains(listed.Body.String(), "Long ID") {
		t.Fatalf("rejected playlists changed state: %q", listed.Body.String())
	}
}
