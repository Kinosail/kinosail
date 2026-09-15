package viewing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestFetchJellyfinProjectsProgressAndPlaylists(t *testing.T) { //nolint:cyclop,gocognit // One source fixture proves pagination, authentication, normalization, and lists.
	const token = "jellyfin-secret-token"
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		methods = append(methods, request.Method)
		if len(methods) > 20 {
			http.Error(writer, "pagination runaway", http.StatusLoopDetected)
			return
		}
		if request.Header.Get("X-Emby-Token") != token || request.Header.Get("X-MediaBrowser-Token") != token || request.Header.Get("Accept") != "application/json" {
			http.Error(writer, "bad headers", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/Users/Me":
			_, _ = writer.Write([]byte(`{"Id":" source-viewer "}`))
		case request.URL.Path == "/Items" && request.URL.Query().Get("IncludeItemTypes") == "Movie,Episode":
			start := request.URL.Query().Get("StartIndex")
			if start == "0" {
				_, _ = writer.Write([]byte(`{"TotalRecordCount":2,"Items":[{"Id":"movie","Name":"Arrival","Type":"Movie","ProductionYear":2016,"RunTimeTicks":1000000000,"Path":"/Arrival.mkv","ProviderIds":{"Tmdb":" 329865 ","Other":"ignored"},"UserData":{"PlaybackPositionTicks":2000000000,"LastPlayedDate":"2026-08-20T12:00:00Z","IsFavorite":true}}]}`))
			} else {
				_, _ = writer.Write([]byte(`{"TotalRecordCount":2,"Items":[{"Id":"episode","Name":"Pilot","Type":"Episode","SeriesName":"Show","ParentIndexNumber":1,"IndexNumber":2,"UserData":{"Played":true}}]}`))
			}
		case request.URL.Path == "/Items":
			start := request.URL.Query().Get("StartIndex")
			if start == "0" {
				_, _ = writer.Write([]byte(`{"TotalRecordCount":2,"Items":[{"Id":"p1","Name":"Queue","Type":"Playlist"}]}`))
			} else {
				_, _ = writer.Write([]byte(`{"TotalRecordCount":2,"Items":[{"Id":"p2","Name":"Queue","Type":"Playlist"}]}`))
			}
		case strings.HasPrefix(request.URL.Path, "/Playlists/"):
			id := "movie"
			if strings.Contains(request.URL.Path, "p2") {
				id = "episode"
			}
			_, _ = writer.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"` + id + `"}]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	activities, err := Fetch(t.Context(), server.Client(), Input{Source: "jellyfin", URL: server.URL, Token: token}, true)
	if err != nil || len(activities) != 2 || activities[0].Seconds != 100 || activities[0].ProviderIDs["tmdb"] != "329865" || activities[0].Playlists["Queue"] != 0 || activities[1].Playlists["Queue (2)"] != 0 || !activities[1].Watched {
		t.Fatalf("activities=%#v error=%v", activities, err)
	}
	for _, method := range methods {
		if method != http.MethodGet {
			t.Fatalf("source method = %s", method)
		}
	}
	withoutLists, err := Fetch(t.Context(), server.Client(), Input{Source: "jellyfin", URL: server.URL, Token: token, SourceUser: "source-viewer"}, false)
	if err != nil || len(withoutLists) != 2 || withoutLists[0].Playlists != nil {
		t.Fatalf("without lists=%#v error=%v", withoutLists, err)
	}
}

func TestFetchPlexProjectsProviderIDsAndPlaylists(t *testing.T) { //nolint:cyclop // One source fixture proves both section kinds, de-duplication, and playlist snapshots.
	token := strings.Join([]string{"plex", "fixture"}, "-")
	var starts []string
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		if calls > 20 {
			http.Error(writer, "pagination runaway", http.StatusLoopDetected)
			return
		}
		if request.Header.Get("X-Plex-Token") != token || request.Header.Get("X-Plex-Client-Identifier") == "" || request.Header.Get("X-Plex-Product") != "Kinosail" {
			http.Error(writer, "bad headers", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/library/sections":
			_, _ = writer.Write([]byte(`{"MediaContainer":{"Directory":[{"Key":"1","Type":"movie"},{"Key":"2","Type":"show"},{"Key":"3","Type":"music"}]}}`))
		case "/library/sections/1/all":
			starts = append(starts, request.Header.Get("X-Plex-Container-Start"))
			if request.Header.Get("X-Plex-Container-Start") == "0" {
				_, _ = writer.Write([]byte(`{"MediaContainer":{"TotalSize":2,"Metadata":[{"RatingKey":"10","Type":"movie","Title":"Arrival","Year":2016,"ViewOffset":42000,"Duration":100000,"LastViewedAt":1787486400,"guid":"tmdb://329865","Guid":[{"ID":"imdb://tt2543164"}],"Media":[{"Part":[{"File":"/Arrival.mkv"}]}]}]}}`))
			} else {
				_, _ = writer.Write([]byte(`{"MediaContainer":{"TotalSize":2,"Metadata":[{"RatingKey":"10","Type":"movie","Title":"duplicate"},{"RatingKey":"11","Type":"movie","Title":"Watched","ViewCount":1}]}}`))
			}
		case "/library/sections/2/all":
			_, _ = writer.Write([]byte(`{"MediaContainer":{"TotalSize":0,"Metadata":[]}}`))
		case "/playlists":
			_, _ = writer.Write([]byte(`{"MediaContainer":{"TotalSize":2,"Metadata":[{"RatingKey":"p1","Title":"Queue","PlaylistType":"video"},{"RatingKey":"p2","Title":"Smart","PlaylistType":"video","Smart":true}]}}`))
		case "/playlists/p1/items":
			_, _ = writer.Write([]byte(`{"MediaContainer":{"TotalSize":1,"Metadata":[{"RatingKey":"10"}]}}`))
		case "/playlists/p2/items":
			_, _ = writer.Write([]byte(`{"MediaContainer":{"TotalSize":1,"Metadata":[{"RatingKey":"missing"}]}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	activities, err := Fetch(t.Context(), server.Client(), Input{Source: "plex", URL: server.URL, Token: token}, true)
	_, hasSmartSnapshot := activities[2].Playlists["Smart (snapshot)"]
	if err != nil || len(activities) != 3 || activities[0].Seconds != 42 || activities[0].Watched || activities[0].ProviderIDs["tmdb"] != "329865" || activities[0].Playlists["Queue"] != 0 || activities[2].Kind != "unsupported" || !hasSmartSnapshot || strings.Join(starts, ",") != "0,1" {
		t.Fatalf("starts=%#v activities=%#v error=%v", starts, activities, err)
	}
	withoutLists, err := Fetch(t.Context(), server.Client(), Input{Source: "plex", URL: server.URL, Token: token}, false)
	if err != nil || len(withoutLists) != 2 {
		t.Fatalf("without lists=%#v error=%v", withoutLists, err)
	}
}

func TestSourceAdaptersRejectInvalidRemoteState(t *testing.T) { //nolint:cyclop,funlen,gocognit // Remote schemas are untrusted even after HTTP succeeds.
	jellyPayloads := []string{
		`{"Id":""}`,
		`{"TotalRecordCount":1,"Items":[{"Id":"movie","Name":"Arrival","Type":"Movie","ProviderIds":` + objectWithEntries(17) + `}]}`,
		`{"TotalRecordCount":1,"Items":[{"Id":"movie","Name":"Arrival","Type":"Movie","UserData":{"LastPlayedDate":"bad"}}]}`,
		`{"TotalRecordCount":1,"Items":[{"Id":"movie","Name":"","Type":"Movie"}]}`,
		`{"TotalRecordCount":1,"Items":[{"Id":"one","Name":"One","Type":"Movie"},{"Id":"two","Name":"Two","Type":"Movie"}]}`,
	}
	for index, payload := range jellyPayloads {
		t.Run("jelly-"+strconv.Itoa(index), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls++
				if calls > 20 {
					http.Error(writer, "pagination runaway", http.StatusLoopDetected)
					return
				}
				if request.URL.Path == "/Users/Me" && index != 0 {
					_, _ = writer.Write([]byte(`{"Id":"viewer"}`))
					return
				}
				_, _ = writer.Write([]byte(payload))
			}))
			defer server.Close()
			if _, err := fetchJellyfinViewingActivity(t.Context(), server.Client(), Input{URL: server.URL, Source: "jellyfin", Token: "token"}, false); err == nil {
				t.Fatal("invalid Jellyfin state accepted")
			}
		})
	}
	plexPayloads := []string{
		`{"MediaContainer":{"Directory":` + plexDirectories(1001) + `}}`,
		`{"MediaContainer":{"Directory":[{"Key":"","Type":"movie"}]}}`,
		`{"MediaContainer":{"Directory":[{"Key":"1","Type":"movie"}]}}`,
	}
	for index, payload := range plexPayloads {
		t.Run("plex-"+strconv.Itoa(index), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls++
				if calls > 20 {
					http.Error(writer, "pagination runaway", http.StatusLoopDetected)
					return
				}
				if request.URL.Path == "/library/sections" {
					_, _ = writer.Write([]byte(payload))
					return
				}
				_, _ = writer.Write([]byte(`{"MediaContainer":{"TotalSize":1,"Metadata":[{"RatingKey":"id","Type":"movie","Title":"Title","LastViewedAt":-1}]}}`))
			}))
			defer server.Close()
			if _, err := fetchPlexViewingActivity(t.Context(), server.Client(), Input{URL: server.URL, Source: "plex", Token: "token"}, false); err == nil {
				t.Fatal("invalid Plex state accepted")
			}
		})
	}
}

func objectWithEntries(count int) string {
	values := make(map[string]string, count)
	for index := range count {
		values[strconv.Itoa(index)] = "id"
	}
	data, _ := json.Marshal(values)
	return string(data)
}

func plexDirectories(count int) string {
	values := make([]map[string]string, count)
	for index := range count {
		values[index] = map[string]string{"Key": strconv.Itoa(index), "Type": "music"}
	}
	data, _ := json.Marshal(values)
	return string(data)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestViewingSourceHTTPRejectsTransportAndPayloadFailures(t *testing.T) {
	input := Input{Source: "plex", URL: "https://source.example", Token: "token"}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, context.Canceled })}
	if err := viewingSourceJSON(t.Context(), client, input, "/items", nil, &struct{}{}); err == nil {
		t.Fatal("transport failure accepted")
	}
	for name, response := range map[string]struct {
		status int
		body   string
	}{
		"status": {http.StatusUnauthorized, `{}`}, "invalid": {http.StatusOK, `{`}, "trailing": {http.StatusOK, `{} {}`}, "large": {http.StatusOK, strings.Repeat("x", maximumViewingSourceBody+1)},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(response.status)
				_, _ = writer.Write([]byte(response.body))
			}))
			defer server.Close()
			input.URL = server.URL
			if err := viewingSourceJSONHeaders(t.Context(), server.Client(), input, "/items", nil, map[string]string{"X-Test": "yes"}, &struct{}{}); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
	input.URL = "://bad"
	if err := viewingSourceJSON(t.Context(), http.DefaultClient, input, "/items", nil, &struct{}{}); err == nil {
		t.Fatal("invalid URL accepted")
	}
	if viewingSourceName("plex") != "Plex" || viewingSourceName("jellyfin") != "Jellyfin" {
		t.Fatal("source name changed")
	}
}
