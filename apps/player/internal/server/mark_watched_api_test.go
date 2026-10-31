package server_test

import (
	"net/http"
	"testing"
)

type watchedListing struct {
	Items []struct {
		ID, Kind, Title string
		Progress        struct {
			Watched bool `json:"watched"`
		} `json:"progress"`
	}
}

func watchedState(t *testing.T, handler http.Handler, token string) map[string]bool {
	t.Helper()
	var listing watchedListing
	mustJSON(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/library?view=all&q=", nil), &listing)
	state := map[string]bool{}
	for _, item := range listing.Items {
		state[item.Title] = item.Progress.Watched
	}
	var show struct {
		Episodes []struct {
			Title    string
			Progress struct {
				Watched bool `json:"watched"`
			} `json:"progress"`
		}
	}
	mustJSON(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/shows/"+severanceID(t, handler, token), nil), &show)
	for _, episode := range show.Episodes {
		state[episode.Title] = episode.Progress.Watched
	}
	return state
}

func severanceID(t *testing.T, handler http.Handler, token string) string {
	t.Helper()
	var shows struct {
		Shows []struct{ ID, Title string }
	}
	mustJSON(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/shows", nil), &shows)
	for _, show := range shows.Shows {
		if show.Title == "Severance" {
			return show.ID
		}
	}
	t.Fatalf("Severance missing from %+v", shows)
	return ""
}

func episodeIDs(t *testing.T, handler http.Handler, token, show string) []string {
	t.Helper()
	var detail struct {
		Episodes []struct{ ID string }
	}
	mustJSON(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/shows/"+show, nil), &detail)
	ids := make([]string, 0, len(detail.Episodes))
	for _, episode := range detail.Episodes {
		ids = append(ids, episode.ID)
	}
	return ids
}

func TestMarkWatchedCoversMoviesEpisodesSeasonsAndShows(t *testing.T) {
	handler, token := apiServer(t)
	show := severanceID(t, handler, token)
	episodes := episodeIDs(t, handler, token, show)
	if len(episodes) != 2 {
		t.Fatalf("fixture episodes = %v", episodes)
	}
	movie := firstAPIItemID(t, handler, token)

	put := func(path string, body any, status int) map[string]any {
		t.Helper()
		response := apiCall(t, handler, token, http.MethodPut, path, body)
		if response.Code != status {
			t.Fatalf("PUT %s %v = %d %q", path, body, response.Code, response.Body.String())
		}
		result := map[string]any{}
		if status == http.StatusOK {
			mustJSON(t, response, &result)
		}
		return result
	}
	if result := put("/api/v1/shows/"+show+"/watched", map[string]any{"watched": true, "season": 1}, http.StatusOK); result["items"] != float64(2) || result["watched"] != true {
		t.Fatalf("season result = %v", result)
	}
	for _, id := range episodes {
		if !itemWatched(t, handler, token, id) {
			t.Fatalf("episode %s not watched after season mark", id)
		}
	}
	put("/api/v1/items/"+episodes[1]+"/watched", map[string]any{"watched": false}, http.StatusOK)
	if itemWatched(t, handler, token, episodes[1]) {
		t.Fatal("episode still watched after mark unwatched")
	}
	if result := put("/api/v1/shows/"+show+"/watched", map[string]any{"watched": false}, http.StatusOK); result["items"] != float64(2) {
		t.Fatalf("show result = %v", result)
	}
	if result := put("/api/v1/items/"+movie+"/watched", map[string]any{"watched": true}, http.StatusOK); result["items"] != float64(1) {
		t.Fatalf("movie result = %v", result)
	}
	if !itemWatched(t, handler, token, movie) {
		t.Fatal("movie not watched")
	}
}

func TestMarkWatchedRejectsBadInputWithoutChangingState(t *testing.T) {
	handler, token := apiServer(t)
	show := severanceID(t, handler, token)
	movie := firstAPIItemID(t, handler, token)
	if response := apiCall(t, handler, token, http.MethodPut, "/api/v1/items/"+movie+"/watched", map[string]any{"watched": true}); response.Code != http.StatusOK {
		t.Fatalf("seed watched = %d", response.Code)
	}
	before := watchedState(t, handler, token)
	for _, call := range []struct {
		path   string
		body   any
		status int
	}{
		{"/api/v1/shows/" + show + "/watched", map[string]any{"watched": true, "season": 9}, http.StatusNotFound},
		{"/api/v1/shows/" + show + "/watched", map[string]any{"watched": true, "season": -1}, http.StatusBadRequest},
		{"/api/v1/shows/" + show + "/watched", map[string]any{"season": 1}, http.StatusBadRequest},
		{"/api/v1/shows/" + show + "/watched", map[string]any{"watched": "yes"}, http.StatusBadRequest},
		{"/api/v1/shows/" + show + "/watched", map[string]any{"watched": true, "extra": 1}, http.StatusBadRequest},
		{"/api/v1/shows/0000000000000000/watched", map[string]any{"watched": true}, http.StatusNotFound},
		{"/api/v1/items/" + movie + "/watched", map[string]any{}, http.StatusBadRequest},
		{"/api/v1/items/0000000000000000/watched", map[string]any{"watched": true}, http.StatusNotFound},
	} {
		if response := apiCall(t, handler, token, http.MethodPut, call.path, call.body); response.Code != call.status {
			t.Errorf("PUT %s %v = %d %q, want %d", call.path, call.body, response.Code, response.Body.String(), call.status)
		}
	}
	after := watchedState(t, handler, token)
	for title, watched := range before {
		if after[title] != watched {
			t.Errorf("%s watched changed from %v to %v after rejected calls", title, watched, after[title])
		}
	}
	if response := apiCall(t, handler, "", http.MethodPut, "/api/v1/shows/"+show+"/watched", map[string]any{"watched": true}); response.Code != http.StatusUnauthorized {
		t.Errorf("anonymous PUT = %d", response.Code)
	}
}

func itemWatched(t *testing.T, handler http.Handler, token, id string) bool {
	t.Helper()
	var detail struct {
		Item struct {
			Progress struct {
				Watched bool `json:"watched"`
			} `json:"progress"`
		} `json:"item"`
	}
	mustJSON(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/items/"+id, nil), &detail)
	return detail.Item.Progress.Watched
}

func TestShowsCanBeAddedToMyList(t *testing.T) {
	handler, token := apiServer(t)
	show := severanceID(t, handler, token)
	listed := func() bool {
		t.Helper()
		var state struct {
			Listed bool `json:"listed"`
		}
		mustJSON(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/shows/"+show+"/list", nil), &state)
		return state.Listed
	}
	listedShows := func() []string {
		t.Helper()
		var shows struct {
			Shows []struct{ ID string }
		}
		mustJSON(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/shows?listed=true", nil), &shows)
		ids := []string{}
		for _, entry := range shows.Shows {
			ids = append(ids, entry.ID)
		}
		return ids
	}
	if listed() || len(listedShows()) != 0 {
		t.Fatal("show listed before it was added")
	}
	if response := apiCall(t, handler, token, http.MethodPut, "/api/v1/shows/"+show+"/list", map[string]any{"listed": true}); response.Code != http.StatusOK {
		t.Fatalf("add show = %d %q", response.Code, response.Body.String())
	}
	if !listed() || len(listedShows()) != 1 || listedShows()[0] != show {
		t.Fatalf("listed shows = %v", listedShows())
	}
	var all struct {
		Shows []struct{ ID string }
	}
	mustJSON(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/shows", nil), &all)
	if len(all.Shows) != 1 {
		t.Fatalf("unfiltered shows = %v", all.Shows)
	}
	for _, call := range []struct {
		method, path string
		body         any
		status       int
	}{
		{http.MethodPut, "/api/v1/shows/" + show + "/list", map[string]any{}, http.StatusBadRequest},
		{http.MethodPut, "/api/v1/shows/" + show + "/list", map[string]any{"listed": "no"}, http.StatusBadRequest},
		{http.MethodPut, "/api/v1/shows/0000000000000000/list", map[string]any{"listed": false}, http.StatusNotFound},
		{http.MethodGet, "/api/v1/shows?listed=maybe", nil, http.StatusBadRequest},
	} {
		if response := apiCall(t, handler, token, call.method, call.path, call.body); response.Code != call.status {
			t.Errorf("%s %s = %d %q, want %d", call.method, call.path, response.Code, response.Body.String(), call.status)
		}
	}
	if !listed() {
		t.Fatal("rejected calls removed the show from My List")
	}
	if response := apiCall(t, handler, token, http.MethodPut, "/api/v1/shows/"+show+"/list", map[string]any{"listed": false}); response.Code != http.StatusOK || listed() {
		t.Fatalf("remove show = %d, listed %v", response.Code, listed())
	}
}
