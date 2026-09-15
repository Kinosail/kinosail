package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestChaptersDBEndpointIsPinnedToTrustedOrigin(t *testing.T) {
	t.Parallel()
	for value, valid := range map[string]bool{
		"https://chaptersdb.com/api/v1":               true,
		"https://chaptersdb.com/api/v1/":              false,
		"https://evil.example/api/v1":                 false,
		"https://chaptersdb.com/api/v1?redirect=evil": false,
		"https://user@chaptersdb.com/api/v1":          false,
		"http://127.0.0.1:8080/api/v1":                true,
		"http://127.0.0.1/api/v1":                     false,
	} {
		parsed, err := url.Parse(value)
		if err != nil || validChaptersDBEndpoint(parsed) != valid {
			t.Fatalf("endpoint %q validity = %v, error = %v", value, validChaptersDBEndpoint(parsed), err)
		}
	}
}

func TestChaptersDBParsesAndCachesApprovedChapterSet(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if request.URL.Path != "/api/v1/chapters/123" || request.Header.Get("Accept") != "application/json" {
			t.Fatalf("request = %s %q", request.URL.Path, request.Header.Get("Accept"))
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"episodeId":    "episode-123",
			"episodeTitle": "S01E01 - Pilot",
			"chapters": []any{map[string]any{
				"id": "set-1", "entries": []any{
					map[string]string{"time": "00:00:00.000", "name": "Cold open"},
					map[string]string{"time": "00:01:30.500", "name": "The case"},
				}, "note": "approved", "upvotes": 4, "downvotes": 0, "uploaderName": "member", "createdAt": "2026-01-01T00:00:00Z",
			}},
			"count": 1,
		})
	}))
	t.Cleanup(provider.Close)
	client := NewChapterProvider(provider.URL + "/api/v1")
	item := library.Item{Show: "Example", Episode: 1, ProviderIDs: map[string]string{"tvdb": "123"}}
	for range 2 {
		chapters := client.Chapters(t.Context(), item, 120, nil)
		if len(chapters) != 2 || chapters[0].Title != "Cold open" || chapters[0].End != 90.5 || chapters[1].Start != 90.5 || chapters[1].End != 120 {
			t.Fatalf("chapters = %#v", chapters)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("ChaptersDB calls = %d", calls.Load())
	}
}

func TestChaptersDBRejectsUnsafeOrAmbiguousChapterData(t *testing.T) {
	t.Parallel()
	for name, payload := range map[string]string{
		"unknown field":     `{"chapters":[{"entries":[{"time":"00:00:00","name":"Intro","extra":true}]}]}`,
		"duplicate time":    `{"chapters":[{"entries":[{"time":"00:00:00","name":"Intro"},{"time":"00:00:00","name":"Again"}]}]}`,
		"invalid time":      `{"chapters":[{"entries":[{"time":"00:00:61","name":"Intro"}]}]}`,
		"control character": "{\"chapters\":[{\"entries\":[{\"time\":\"00:00:00\",\"name\":\"Intro\\u0000\"}]}]}",
	} {
		t.Run(name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				_, _ = writer.Write([]byte(payload))
			}))
			t.Cleanup(provider.Close)
			client := NewChapterProvider(provider.URL + "/api/v1")
			item := library.Item{Show: "Example", Episode: 1, ProviderIDs: map[string]string{"tvdb": "123"}}
			if chapters := client.Chapters(t.Context(), item, 120, nil); len(chapters) != 0 {
				t.Fatalf("unsafe chapters accepted: %#v", chapters)
			}
		})
	}
}

func TestChaptersDBOnlyReplacesGenericLocalTitles(t *testing.T) {
	local := []Chapter{{Index: 0, Start: 0, End: 60, Title: "Chapter 1"}, {Index: 1, Start: 60, End: 120, Title: "Already named"}}
	external := []Chapter{{Index: 0, Start: 0, End: 60, Title: "Cold open"}, {Index: 1, Start: 60, End: 120, Title: "External name"}}
	merged := mergeExternalChapters(local, external)
	if merged[0].Title != "Cold open" || merged[1].Title != "Already named" || local[0].Title != "Chapter 1" {
		t.Fatalf("merged = %#v, local = %#v", merged, local)
	}
	if got := mergeExternalChapters(local, external[:1]); got[0].Title != "Chapter 1" {
		t.Fatalf("mismatched chapters changed local data: %#v", got)
	}
	if got, ok := parseChapterDBTime("00:01:02.50"); !ok || got != 62.5 {
		t.Fatalf("chapter time = %v", got)
	}
	if _, ok := parseChapterDBTime(strings.TrimSpace("1:2:03")); ok {
		t.Fatal("ambiguous chapter time accepted")
	}
}

func TestChapterProviderReturnsNoDataWithoutTVDBEpisodeIdentity(t *testing.T) {
	provider := NewChapterProvider(defaultChaptersDBURL)
	if got := provider.Chapters(context.Background(), library.Item{Show: "Example", Episode: 1}, 120, nil); got != nil {
		t.Fatalf("chapters without TVDB identity = %#v", got)
	}
}
