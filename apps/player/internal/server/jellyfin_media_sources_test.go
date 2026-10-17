package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type jellyfinSourceRecord struct {
	ID               string             `json:"Id"`
	Type             string             `json:"Type"`
	MediaSources     *[]json.RawMessage `json:"MediaSources"`
	MediaSourceCount *int               `json:"MediaSourceCount"`
}

func TestJellyfinListingsAttachMediaSourcesOnlyWhenRequested(t *testing.T) {
	t.Parallel()
	handler, sessions, ids := jellyfinShapeServer(t)
	owner := sessions[0].token
	for _, path := range []string{"/Items", "/Items?includeItemTypes=Episode", "/Items/Latest", "/Shows/NextUp", "/UserItems/Resume", "/Shows/" + ids["Severance"] + "/Episodes", "/Shows/" + ids["Severance"] + "/Seasons"} {
		for _, record := range jellyfinSourceRecords(t, handler, owner, path) {
			if record.MediaSources != nil || record.MediaSourceCount != nil {
				t.Fatalf("%s %s %s carries media sources without Fields=MediaSources", path, record.Type, record.ID)
			}
		}
	}
	requested := map[string]int{
		"/Items?fields=MediaSources":                                           4,
		"/Items?Fields=Overview,mediasources&includeItemTypes=Episode":         5,
		"/Items?fields=Overview&fields=MediaSources&parentId=" + ids["movies"]: 3,
		"/Items/Latest?fields=MediaSources&limit=3":                            2,
		"/Shows/" + ids["Severance"] + "/Episodes?fields=MediaSources":         4,
		"/UserItems/Resume?fields=MediaSources":                                1,
		"/Shows/NextUp?fields=MediaSources":                                    2,
	}
	for path, want := range requested {
		attached := 0
		for _, record := range jellyfinSourceRecords(t, handler, owner, path) {
			switch record.Type {
			case "Movie", "Episode", "Audio":
				assertJellyfinMediaSource(t, path, record)
				attached++
			default:
				if record.MediaSources != nil {
					t.Fatalf("%s %s %s carries media sources", path, record.Type, record.ID)
				}
			}
		}
		if attached != want {
			t.Fatalf("%s attached %d media sources, want %d", path, attached, want)
		}
	}
	for _, id := range []string{ids["Arrival"], ids["S01E01 · Good News"]} {
		var record jellyfinSourceRecord
		decodeJellyfin(t, jellyfinCall(t, handler, http.MethodGet, "/Items/"+id, "", owner), &record)
		assertJellyfinMediaSource(t, "/Items/"+id, record)
	}
	var subtitles struct {
		MediaSources []struct {
			MediaStreams []struct {
				Type        string `json:"Type"`
				DeliveryURL string `json:"DeliveryUrl"`
			}
		}
	}
	decodeJellyfin(t, jellyfinCall(t, handler, http.MethodGet, "/Items/"+ids["Arrival"], "", owner), &subtitles)
	if streams := subtitles.MediaSources[0].MediaStreams; len(streams) != 2 || streams[0].Type != "Subtitle" || !strings.HasSuffix(streams[1].DeliveryURL, "/Subtitles/1/Stream.vtt") {
		t.Fatalf("Arrival media streams = %#v", streams)
	}
}

func jellyfinSourceRecords(t *testing.T, handler http.Handler, token, path string) []jellyfinSourceRecord {
	t.Helper()
	response := jellyfinCall(t, handler, http.MethodGet, path, "", token)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), token) || strings.Contains(response.Body.String(), "api_key") || strings.Contains(response.Body.String(), "playSessionId") {
		t.Fatalf("%s = %d %q", path, response.Code, response.Body.String())
	}
	var page struct{ Items []jellyfinSourceRecord }
	if strings.HasPrefix(path, "/Items/Latest") {
		decodeJellyfin(t, response, &page.Items)
	} else {
		decodeJellyfin(t, response, &page)
	}
	return page.Items
}

func assertJellyfinMediaSource(t *testing.T, path string, record jellyfinSourceRecord) {
	t.Helper()
	if record.MediaSources == nil || len(*record.MediaSources) != 1 || record.MediaSourceCount != nil {
		t.Fatalf("%s %s media sources = %v count = %v", path, record.ID, record.MediaSources, record.MediaSourceCount)
	}
	var source struct {
		ID, Protocol, Path string
		SupportsDirectPlay bool
		MediaStreams       []json.RawMessage
	}
	raw := (*record.MediaSources)[0]
	if err := json.Unmarshal(raw, &source); err != nil || !strings.HasPrefix(string(raw), "{") || source.ID != record.ID || source.Protocol != "Http" || source.Path != "/Videos/"+record.ID+"/stream" || !source.SupportsDirectPlay || source.MediaStreams == nil {
		t.Fatalf("%s %s media source = %s: %v", path, record.ID, raw, err)
	}
}
