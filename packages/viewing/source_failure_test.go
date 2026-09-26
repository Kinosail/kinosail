package viewing

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func viewingTestClient(handler func(*http.Request) (int, string)) *http.Client {
	return viewingTestClientLimit(10, handler)
}

func viewingTestClientLimit(limit int, handler func(*http.Request) (int, string)) *http.Client {
	calls := 0
	var mutex sync.Mutex
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		mutex.Lock()
		defer mutex.Unlock()
		calls++
		if calls > limit {
			return &http.Response{StatusCode: http.StatusLoopDetected, Status: "508 Loop", Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
		}
		status, body := handler(request)
		return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d Test", status), Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
}

func TestJellyfinAdapterFailurePaths(t *testing.T) { //nolint:cyclop,funlen,gocognit // Each remote page stage fails closed without a partial snapshot.
	transportFailure := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, io.ErrUnexpectedEOF })}
	input := Input{Source: "jellyfin", URL: "https://source.example", Token: "token"}
	if _, err := fetchJellyfinViewingActivity(t.Context(), transportFailure, input, false); err == nil {
		t.Fatal("user lookup failure accepted")
	}
	input.SourceUser = "viewer"
	if _, err := fetchJellyfinViewingActivity(t.Context(), transportFailure, input, false); err == nil {
		t.Fatal("item lookup failure accepted")
	}
	if _, err := addJellyfinPlaylists(t.Context(), transportFailure, input, "viewer", nil); err == nil {
		t.Fatal("playlist lookup failure accepted")
	}

	tooMany := viewingTestClient(func(request *http.Request) (int, string) {
		if request.URL.Path == "/Items" {
			return http.StatusOK, `{"TotalRecordCount":1001,"Items":[]}`
		}
		return http.StatusOK, `{}`
	})
	if _, err := addJellyfinPlaylists(t.Context(), tooMany, input, "viewer", nil); err == nil {
		t.Fatal("playlist count accepted")
	}
	invalidChild := viewingTestClient(func(*http.Request) (int, string) {
		return http.StatusOK, `{"TotalRecordCount":1,"Items":[{"Id":"","Name":"Queue","Type":"Playlist"}]}`
	})
	if _, err := addJellyfinPlaylists(t.Context(), invalidChild, input, "viewer", nil); err == nil {
		t.Fatal("invalid playlist accepted")
	}
	pageCalls := 0
	pageLimit := viewingTestClient(func(*http.Request) (int, string) {
		pageCalls++
		return http.StatusOK, jellyfinPlaylistPage(200, "music")
	})
	if _, err := addJellyfinPlaylists(t.Context(), pageLimit, input, "viewer", nil); err == nil {
		t.Fatal("playlist page limit accepted")
	}
	if pageCalls != 5 {
		t.Fatalf("playlist page calls=%d", pageCalls)
	}

	cases := map[string]struct {
		playlist jellyfinViewingPlaylist
		client   *http.Client
	}{
		"name":    {playlist: jellyfinViewingPlaylist{ID: "id", Name: "bad/name", Type: "Playlist"}, client: viewingTestClient(func(*http.Request) (int, string) { return http.StatusOK, `{}` })},
		"request": {playlist: jellyfinViewingPlaylist{ID: "id", Name: "Queue", Type: "Playlist"}, client: transportFailure},
		"pagination": {playlist: jellyfinViewingPlaylist{ID: "id", Name: "Queue", Type: "Playlist"}, client: viewingTestClient(func(*http.Request) (int, string) {
			return http.StatusOK, `{"TotalRecordCount":1,"Items":[{"Id":"one"},{"Id":"two"}]}`
		})},
		"item": {playlist: jellyfinViewingPlaylist{ID: "id", Name: "Queue", Type: "Playlist"}, client: viewingTestClient(func(*http.Request) (int, string) { return http.StatusOK, `{"TotalRecordCount":1,"Items":[{"Id":""}]}` })},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := addJellyfinPlaylist(t.Context(), test.client, input, "viewer", test.playlist, make(map[string]int), make(map[string]int), nil, new(int)); err == nil {
				t.Fatal("invalid playlist state accepted")
			}
		})
	}
	if result, err := addJellyfinPlaylist(t.Context(), http.DefaultClient, input, "viewer", jellyfinViewingPlaylist{Type: "Audio"}, nil, nil, []Activity{{SourceID: "keep"}}, new(int)); err != nil || len(result) != 1 {
		t.Fatalf("ignored playlist=%#v error=%v", result, err)
	}
	itemCalls := 0
	itemLimit := viewingTestClientLimit(510, func(*http.Request) (int, string) { itemCalls++; return http.StatusOK, jellyfinItemPage(200, "known") })
	if _, err := addJellyfinPlaylist(t.Context(), itemLimit, input, "viewer", jellyfinViewingPlaylist{ID: "id", Name: "Queue", Type: "Playlist"}, make(map[string]int), map[string]int{"known": 0}, []Activity{{SourceID: "known"}}, new(int)); err == nil {
		t.Fatal("playlist item page limit accepted")
	}
	if itemCalls != 500 {
		t.Fatalf("playlist item page calls=%d", itemCalls)
	}
}

func TestPlexAdapterFailurePaths(t *testing.T) { //nolint:cyclop,funlen,gocognit // Each Plex page stage fails closed without a partial snapshot.
	transportFailure := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, io.ErrUnexpectedEOF })}
	input := Input{Source: "plex", URL: "https://source.example", Token: "token"}
	if _, err := fetchPlexViewingActivity(t.Context(), transportFailure, input, false); err == nil {
		t.Fatal("section lookup failure accepted")
	}
	if _, err := addPlexPlaylists(t.Context(), transportFailure, input, nil); err == nil {
		t.Fatal("playlist lookup failure accepted")
	}

	for name := range map[string]bool{"page": true, "metadata": true, "parts": true, "identity": true} {
		t.Run(name, func(t *testing.T) {
			client := viewingTestClient(func(request *http.Request) (int, string) {
				if request.URL.Path == "/library/sections" {
					return http.StatusOK, `{"MediaContainer":{"Directory":[{"Key":"1","Type":"movie"}]}}`
				}
				switch name {
				case "page":
					return http.StatusOK, `{"MediaContainer":{"TotalSize":1,"Metadata":[{},{}]}}`
				case "metadata":
					return http.StatusOK, `{"MediaContainer":{"TotalSize":1,"Metadata":[{"RatingKey":"id","Type":"movie","Title":"Title","Guid":` + stringArray(33) + `}]}}`
				case "parts":
					return http.StatusOK, `{"MediaContainer":{"TotalSize":1,"Metadata":[{"RatingKey":"id","Type":"movie","Title":"Title","Media":[{"Part":` + partArray(17) + `}] }]}}`
				default:
					return http.StatusOK, `{"MediaContainer":{"TotalSize":1,"Metadata":[{"RatingKey":"id","Type":"movie","Title":""}]}}`
				}
			})
			if _, err := fetchPlexViewingActivity(t.Context(), client, input, false); err == nil {
				t.Fatal("invalid Plex page accepted")
			}
		})
	}
	pageRequest := viewingTestClient(func(request *http.Request) (int, string) {
		if request.URL.Path == "/library/sections" {
			return http.StatusOK, `{"MediaContainer":{"Directory":[{"Key":"1","Type":"movie"}]}}`
		}
		return http.StatusUnauthorized, `{}`
	})
	if _, err := fetchPlexViewingActivity(t.Context(), pageRequest, input, false); err == nil {
		t.Fatal("Plex item request failure accepted")
	}

	tooMany := viewingTestClient(func(*http.Request) (int, string) {
		return http.StatusOK, `{"MediaContainer":{"TotalSize":1001,"Metadata":[]}}`
	})
	if _, err := addPlexPlaylists(t.Context(), tooMany, input, nil); err == nil {
		t.Fatal("playlist count accepted")
	}
	invalidChild := viewingTestClient(func(*http.Request) (int, string) {
		return http.StatusOK, `{"MediaContainer":{"TotalSize":1,"Metadata":[{"RatingKey":"","Title":"Queue"}]}}`
	})
	if _, err := addPlexPlaylists(t.Context(), invalidChild, input, nil); err == nil {
		t.Fatal("invalid playlist accepted")
	}
	pageCalls := 0
	pageLimit := viewingTestClient(func(*http.Request) (int, string) { pageCalls++; return http.StatusOK, plexPlaylistPage(200, "audio") })
	if _, err := addPlexPlaylists(t.Context(), pageLimit, input, nil); err == nil {
		t.Fatal("playlist page limit accepted")
	}
	if pageCalls != 5 {
		t.Fatalf("playlist page calls=%d", pageCalls)
	}

	cases := map[string]struct {
		playlist plexViewingPlaylist
		client   *http.Client
	}{
		"name":    {playlist: plexViewingPlaylist{RatingKey: "id", Title: "bad/name"}, client: viewingTestClient(func(*http.Request) (int, string) { return http.StatusOK, `{}` })},
		"request": {playlist: plexViewingPlaylist{RatingKey: "id", Title: "Queue"}, client: transportFailure},
		"pagination": {playlist: plexViewingPlaylist{RatingKey: "id", Title: "Queue"}, client: viewingTestClient(func(*http.Request) (int, string) {
			return http.StatusOK, `{"MediaContainer":{"TotalSize":1,"Metadata":[{"RatingKey":"one"},{"RatingKey":"two"}]}}`
		})},
		"item": {playlist: plexViewingPlaylist{RatingKey: "id", Title: "Queue"}, client: viewingTestClient(func(*http.Request) (int, string) {
			return http.StatusOK, `{"MediaContainer":{"TotalSize":1,"Metadata":[{"RatingKey":""}]}}`
		})},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := addPlexPlaylist(t.Context(), test.client, input, test.playlist, make(map[string]int), make(map[string]int), nil, new(int)); err == nil {
				t.Fatal("invalid playlist state accepted")
			}
		})
	}
	if result, err := addPlexPlaylist(t.Context(), http.DefaultClient, input, plexViewingPlaylist{PlaylistType: "audio"}, nil, nil, []Activity{{SourceID: "keep"}}, new(int)); err != nil || len(result) != 1 {
		t.Fatalf("ignored playlist=%#v error=%v", result, err)
	}
	itemCalls := 0
	itemLimit := viewingTestClientLimit(510, func(*http.Request) (int, string) { itemCalls++; return http.StatusOK, plexItemPage(200, "known") })
	if _, err := addPlexPlaylist(t.Context(), itemLimit, input, plexViewingPlaylist{RatingKey: "id", Title: "Queue"}, make(map[string]int), map[string]int{"known": 0}, []Activity{{SourceID: "known"}}, new(int)); err == nil {
		t.Fatal("playlist item page limit accepted")
	}
	if itemCalls != 500 {
		t.Fatalf("playlist item page calls=%d", itemCalls)
	}
}

func jellyfinPlaylistPage(count int, kind string) string {
	items := make([]string, count)
	for index := range items {
		items[index] = fmt.Sprintf(`{"Id":"%d","Name":"List %d","Type":"%s"}`, index, index, kind)
	}
	return `{"TotalRecordCount":0,"Items":[` + strings.Join(items, ",") + `]}`
}

func jellyfinItemPage(count int, id string) string {
	return `{"TotalRecordCount":0,"Items":[` + strings.TrimSuffix(strings.Repeat(`{"Id":"`+id+`"},`, count), ",") + `]}`
}

func plexPlaylistPage(count int, kind string) string {
	items := make([]string, count)
	for index := range items {
		items[index] = fmt.Sprintf(`{"RatingKey":"%d","Title":"List %d","PlaylistType":"%s"}`, index, index, kind)
	}
	return `{"MediaContainer":{"TotalSize":0,"Metadata":[` + strings.Join(items, ",") + `]}}`
}

func plexItemPage(count int, id string) string {
	return `{"MediaContainer":{"TotalSize":0,"Metadata":[` + strings.TrimSuffix(strings.Repeat(`{"RatingKey":"`+id+`"},`, count), ",") + `]}}`
}

func stringArray(count int) string {
	return `[` + strings.TrimSuffix(strings.Repeat(`{"ID":"tmdb://1"},`, count), ",") + `]`
}

func partArray(count int) string {
	return `[` + strings.TrimSuffix(strings.Repeat(`{"File":"file"},`, count), ",") + `]`
}

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (failingReadCloser) Close() error             { return nil }

func TestViewingSourceHTTPRejectsReadFailure(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: failingReadCloser{}, Header: make(http.Header)}, nil
	})}
	if err := viewingSourceJSON(t.Context(), client, Input{Source: "plex", URL: "https://source.example"}, "/items", nil, &struct{}{}); err == nil {
		t.Fatal("read failure accepted")
	}
}
