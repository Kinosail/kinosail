package viewing

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestViewingSourceHTTPAcceptsExactBodyLimit(t *testing.T) {
	prefix, suffix := `{"value":"`, `"}`
	body := prefix + strings.Repeat("a", maximumViewingSourceBody-len(prefix)-len(suffix)) + suffix
	client := viewingTestClient(func(*http.Request) (int, string) { return http.StatusOK, body })
	var result map[string]string
	if err := viewingSourceJSON(t.Context(), client, Input{Source: "plex", URL: "https://source.example"}, "/items", nil, &result); err != nil || len(result["value"]) != maximumViewingSourceBody-len(prefix)-len(suffix) {
		t.Fatalf("body size=%d error=%v", len(result["value"]), err)
	}
}

func TestJellyfinViewingSkipsNonPlaybackItems(t *testing.T) {
	t.Parallel()
	result, err := appendJellyfinViewingActivities([]Activity{{SourceID: "existing"}}, []jellyfinViewingItem{{ID: "folder", Type: "Folder"}})
	if err != nil || len(result) != 1 || result[0].SourceID != "existing" {
		t.Fatalf("activities = %#v, %v", result, err)
	}
}

func TestSourceAdaptersEnforceAggregateItemLimits(t *testing.T) { //nolint:gocognit // Each source limit is asserted against its independent request sequence.
	jellyCalls := 0
	jellyfin := viewingTestClientLimit(510, func(*http.Request) (int, string) { jellyCalls++; return http.StatusOK, jellyfinActivityPage(200) })
	if _, err := fetchJellyfinViewingActivity(t.Context(), jellyfin, Input{Source: "jellyfin", URL: "https://source.example", Token: "token", SourceUser: "viewer"}, false); err == nil {
		t.Fatal("Jellyfin item limit accepted")
	}
	if jellyCalls != 500 {
		t.Fatalf("Jellyfin page calls=%d", jellyCalls)
	}
	for _, total := range []int{0, maximumViewingItems} {
		t.Run(fmt.Sprintf("plex-%d", total), func(t *testing.T) {
			calls := 0
			plex := viewingTestClientLimit(510, func(request *http.Request) (int, string) {
				calls++
				if request.URL.Path == "/library/sections" {
					return http.StatusOK, `{"MediaContainer":{"Directory":[{"Key":"one","Type":"movie"},{"Key":"two","Type":"movie"}]}}`
				}
				start, _ := strconv.Atoi(request.Header.Get("X-Plex-Container-Start"))
				if strings.Contains(request.URL.Path, "/two/") {
					if total == maximumViewingItems {
						return http.StatusOK, plexActivityPage(maximumViewingItems, 1, 1)
					}
					start += maximumViewingItems
				}
				return http.StatusOK, plexActivityPage(start, 200, total)
			})
			if _, err := fetchPlexViewingActivity(t.Context(), plex, Input{Source: "plex", URL: "https://source.example", Token: "token"}, false); err == nil {
				t.Fatal("Plex item limit accepted")
			}
			want := 501
			if total == maximumViewingItems {
				want = 502
			}
			if calls != want {
				t.Fatalf("Plex page calls=%d, want %d", calls, want)
			}
		})
	}
}

func TestSourceAdapterBoundaryValuesAreAccepted(t *testing.T) { //nolint:cyclop,funlen,gocognit // Exact maxima separate valid remote state from rejected oversize state.
	if maximumViewingSourceBody != 16_777_216 || maximumViewingItems != 100000 || maximumViewingPlaylists != 1000 || maximumViewingSeconds != 31_622_400 {
		t.Fatal("viewing source limits changed")
	}
	user := strings.Repeat("u", 512)
	discovery := viewingTestClient(func(request *http.Request) (int, string) {
		if request.URL.Path == "/Users/Me" {
			return http.StatusOK, `{"Id":"` + user + `"}`
		}
		return http.StatusOK, `{"TotalRecordCount":0,"Items":[]}`
	})
	if _, err := fetchJellyfinViewingActivity(t.Context(), discovery, Input{Source: "jellyfin", URL: "https://source.example", Token: "token"}, false); err != nil {
		t.Fatalf("boundary discovered user rejected: %v", err)
	}
	client := viewingTestClient(func(*http.Request) (int, string) { return http.StatusOK, `{"TotalRecordCount":0,"Items":[]}` })
	if _, err := fetchJellyfinViewingActivity(t.Context(), client, Input{Source: "jellyfin", URL: "https://source.example", Token: "token", SourceUser: user}, false); err != nil {
		t.Fatalf("boundary user rejected: %v", err)
	}
	providers := make(map[string]string, 16)
	for index := range 16 {
		providers[fmt.Sprintf("provider%d", index)] = "id"
	}
	payload := `{"TotalRecordCount":1,"Items":[{"Id":"id","Name":"Title","Type":"Movie","ProviderIds":` + marshalStringMap(providers) + `}]}`
	client = viewingTestClient(func(*http.Request) (int, string) { return http.StatusOK, payload })
	if _, err := fetchJellyfinViewingActivity(t.Context(), client, Input{Source: "jellyfin", URL: "https://source.example", Token: "token", SourceUser: user}, false); err != nil {
		t.Fatalf("provider boundary rejected: %v", err)
	}
	for _, source := range []string{"jellyfin", "plex"} {
		client = viewingTestClient(func(*http.Request) (int, string) {
			if source == "plex" {
				return http.StatusOK, `{"MediaContainer":{"TotalSize":1000,"Metadata":[]}}`
			}
			return http.StatusOK, `{"TotalRecordCount":1000,"Items":[]}`
		})
		input := Input{Source: source, URL: "https://source.example", Token: "token"}
		var err error
		if source == "plex" {
			_, err = addPlexPlaylists(t.Context(), client, input, nil)
		} else {
			_, err = addJellyfinPlaylists(t.Context(), client, input, "viewer", nil)
		}
		if err != nil {
			t.Fatalf("%s playlist boundary rejected: %v", source, err)
		}
	}
	longID := strings.Repeat("i", 512)
	emptyJelly := viewingTestClient(func(*http.Request) (int, string) { return http.StatusOK, `{"TotalRecordCount":0,"Items":[]}` })
	if _, err := addJellyfinPlaylist(t.Context(), emptyJelly, Input{Source: "jellyfin", URL: "https://source.example"}, "viewer", jellyfinViewingPlaylist{ID: longID, Name: "Queue", Type: "Playlist"}, make(map[string]int), make(map[string]int), nil, new(int)); err != nil {
		t.Fatalf("Jellyfin playlist ID boundary rejected: %v", err)
	}
	emptyPlex := viewingTestClient(func(*http.Request) (int, string) {
		return http.StatusOK, `{"MediaContainer":{"TotalSize":0,"Metadata":[]}}`
	})
	if _, err := addPlexPlaylist(t.Context(), emptyPlex, Input{Source: "plex", URL: "https://source.example"}, plexViewingPlaylist{RatingKey: longID, Title: "Queue"}, make(map[string]int), make(map[string]int), nil, new(int)); err != nil {
		t.Fatalf("Plex playlist ID boundary rejected: %v", err)
	}
	sections := viewingTestClient(func(request *http.Request) (int, string) {
		if request.URL.Path == "/library/sections" {
			return http.StatusOK, `{"MediaContainer":{"Directory":` + plexDirectories(1000) + `}}`
		}
		return http.StatusOK, `{}`
	})
	if result, err := fetchPlexViewingActivity(t.Context(), sections, Input{Source: "plex", URL: "https://source.example"}, false); err != nil || len(result) != 0 {
		t.Fatalf("section boundary=%#v error=%v", result, err)
	}
	key := strings.Repeat("k", 512)
	if _, err := fetchPlexSection(t.Context(), emptyPlex, Input{Source: "plex", URL: "https://source.example"}, key, "movie", nil); err != nil {
		t.Fatalf("section key boundary rejected: %v", err)
	}
	requests := 0
	guarded := viewingTestClient(func(*http.Request) (int, string) { requests++; return http.StatusOK, `{}` })
	if _, err := fetchPlexSection(t.Context(), guarded, Input{Source: "plex", URL: "https://source.example"}, "", "movie", nil); err == nil || requests != 0 {
		t.Fatalf("empty section key made %d requests: %v", requests, err)
	}
	if _, err := fetchPlexSection(t.Context(), guarded, Input{Source: "plex", URL: "https://source.example"}, strings.Repeat("k", 513), "movie", nil); err == nil || requests != 0 {
		t.Fatalf("long section key made %d requests: %v", requests, err)
	}
	item := plexViewingItem{RatingKey: "id", Type: "movie", Title: "Title"}
	item.GuidList = make([]struct{ ID string }, 32)
	item.Media = make([]struct{ Part []struct{ File string } }, 8)
	item.Media[0].Part = make([]struct{ File string }, 16)
	item.Media[0].Part[0].File = "/movie.mkv"
	if activity, err := plexViewingActivity(item); err != nil || activity.Path != "/movie.mkv" || activity.Watched {
		t.Fatalf("metadata boundary=%#v error=%v", activity, err)
	}
	item.Media = []struct{ Part []struct{ File string } }{{}}
	if activity, err := plexViewingActivity(item); err != nil || activity.Path != "" {
		t.Fatalf("empty media part=%#v error=%v", activity, err)
	}
}

func TestPlaylistPositionsContinueAcrossPages(t *testing.T) { //nolint:cyclop // Both source adapters share the same cross-page ordering invariant.
	jellyfin := viewingTestClient(func(request *http.Request) (int, string) {
		if request.URL.Query().Get("StartIndex") == "1" {
			return http.StatusOK, `{"TotalRecordCount":3,"Items":[{"Id":"two"},{"Id":"three"}]}`
		}
		return http.StatusOK, `{"TotalRecordCount":3,"Items":[{"Id":"one"}]}`
	})
	activities := []Activity{{SourceID: "one"}, {SourceID: "two"}, {SourceID: "three"}}
	result, err := addJellyfinPlaylist(t.Context(), jellyfin, Input{Source: "jellyfin", URL: "https://source.example"}, "viewer", jellyfinViewingPlaylist{ID: "id", Name: "Queue", Type: "Playlist"}, make(map[string]int), map[string]int{"one": 0, "two": 1, "three": 2}, activities, new(int))
	if err != nil || result[0].Playlists["Queue"] != 0 || result[1].Playlists["Queue"] != 1 || result[2].Playlists["Queue"] != 2 {
		t.Fatalf("Jellyfin positions=%#v error=%v", result, err)
	}
	plex := viewingTestClient(func(request *http.Request) (int, string) {
		if request.Header.Get("X-Plex-Container-Start") == "1" {
			return http.StatusOK, `{"MediaContainer":{"TotalSize":3,"Metadata":[{"RatingKey":"two"},{"RatingKey":"three"}]}}`
		}
		return http.StatusOK, `{"MediaContainer":{"TotalSize":3,"Metadata":[{"RatingKey":"one"}]}}`
	})
	result, err = addPlexPlaylist(t.Context(), plex, Input{Source: "plex", URL: "https://source.example"}, plexViewingPlaylist{RatingKey: "id", Title: "Queue"}, make(map[string]int), map[string]int{"one": 0, "two": 1, "three": 2}, activities, new(int))
	if err != nil || result[0].Playlists["Queue"] != 0 || result[1].Playlists["Queue"] != 1 || result[2].Playlists["Queue"] != 2 {
		t.Fatalf("Plex positions=%#v error=%v", result, err)
	}
}

func marshalStringMap(values map[string]string) string {
	parts := make([]string, 0, len(values))
	for key, value := range values {
		parts = append(parts, fmt.Sprintf(`%q:%q`, key, value))
	}
	return `{` + strings.Join(parts, ",") + `}`
}

func jellyfinActivityPage(count int) string {
	items := make([]string, count)
	for index := range items {
		items[index] = fmt.Sprintf(`{"Id":"%d","Name":"Movie %d","Type":"Movie"}`, index, index)
	}
	return `{"TotalRecordCount":0,"Items":[` + strings.Join(items, ",") + `]}`
}

func plexActivityPage(start, count, total int) string {
	items := make([]string, count)
	for index := range items {
		items[index] = fmt.Sprintf(`{"RatingKey":"%d","Type":"movie","Title":"Movie %d"}`, start+index, start+index)
	}
	return fmt.Sprintf(`{"MediaContainer":{"TotalSize":%d,"Metadata":[%s]}}`, total, strings.Join(items, ","))
}
