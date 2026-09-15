package metadata

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestTVMazeDefaultsFallbackAndBoundedRecord(t *testing.T) {
	t.Parallel()
	if NewTVMazeProvider("") == nil {
		t.Fatal("default TVMaze provider was not created")
	}
	provider := &TVMazeProvider{
		baseURL: "https://example.com", userAgent: "Kinosail/test",
		client:   &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })},
		shows:    map[string]int{"1": 1},
		details:  make(map[int]tvmazeShow),
		episodes: map[string]tvmazeEpisode{"1/1/1": {ID: 2, Name: "Episode", Season: 1, Number: 1}},
	}
	item := library.Item{Show: "Folder Show", Season: 1, Episode: 1, ProviderIDs: map[string]string{"tvdb": "1"}}
	if result, ok := provider.Record(t.Context(), item); !ok || result.Record.ShowTitle != "Folder Show" {
		t.Fatalf("fallback record = %#v, %v", result, ok)
	}
	item.Show = strings.Repeat("x", 201)
	if _, ok := provider.Record(t.Context(), item); ok {
		t.Fatal("oversized fallback show title was accepted")
	}
	record := Record{Title: "Local"}
	provider.Fill(t.Context(), library.Item{}, &record)
	if record.Title != "Local" {
		t.Fatalf("failed fill changed record: %#v", record)
	}
}

func TestTVMazeShowIDRejectsRequestAndTransportFailures(t *testing.T) {
	t.Parallel()
	invalidRequest := &TVMazeProvider{baseURL: "http://[::1", client: http.DefaultClient, shows: make(map[string]int)}
	if id, ok := invalidRequest.showID(t.Context(), "1"); ok || id != 0 {
		t.Fatalf("invalid request show ID = %d, %v", id, ok)
	}
	transportFailure := &TVMazeProvider{
		baseURL: "https://example.com", shows: make(map[string]int),
		client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })},
	}
	if id, ok := transportFailure.showID(t.Context(), "1"); ok || id != 0 {
		t.Fatalf("transport failure show ID = %d, %v", id, ok)
	}
}

func TestTVMazeShowRejectsRequestTransportStatusAndPayloadFailures(t *testing.T) { //nolint:cyclop // Every provider failure mode returns no cached show.
	t.Parallel()
	invalidRequest := &TVMazeProvider{baseURL: "http://[::1", client: http.DefaultClient, details: make(map[int]tvmazeShow)}
	if show, ok := invalidRequest.show(t.Context(), 1); ok || show.ID != 0 {
		t.Fatalf("invalid request show = %#v, %v", show, ok)
	}
	for name, trip := range map[string]roundTripFunc{
		"transport": func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") },
		"status":    func(*http.Request) (*http.Response, error) { return providerResponse(http.StatusBadGateway, ""), nil },
		"payload":   func(*http.Request) (*http.Response, error) { return providerResponse(http.StatusOK, "{"), nil },
	} {
		t.Run(name, func(t *testing.T) {
			provider := &TVMazeProvider{baseURL: "https://example.com", userAgent: "Kinosail/test", client: &http.Client{Transport: trip}, details: make(map[int]tvmazeShow)}
			if show, ok := provider.show(t.Context(), 1); ok || show.ID != 0 {
				t.Fatalf("invalid show = %#v, %v", show, ok)
			}
		})
	}
}

func TestTVMazeEpisodeRejectsRequestTransportStatusAndPayloadFailures(t *testing.T) { //nolint:cyclop // Every provider failure mode returns no cached episode.
	t.Parallel()
	invalidRequest := &TVMazeProvider{baseURL: "http://[::1", client: http.DefaultClient, episodes: make(map[string]tvmazeEpisode)}
	if episode, ok := invalidRequest.episode(t.Context(), 1, 1, 1); ok || episode.ID != 0 {
		t.Fatalf("invalid request episode = %#v, %v", episode, ok)
	}
	for name, trip := range map[string]roundTripFunc{
		"transport": func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") },
		"status":    func(*http.Request) (*http.Response, error) { return providerResponse(http.StatusBadGateway, ""), nil },
		"payload":   func(*http.Request) (*http.Response, error) { return providerResponse(http.StatusOK, "{"), nil },
	} {
		t.Run(name, func(t *testing.T) {
			provider := &TVMazeProvider{baseURL: "https://example.com", userAgent: "Kinosail/test", client: &http.Client{Transport: trip}, episodes: make(map[string]tvmazeEpisode)}
			if episode, ok := provider.episode(t.Context(), 1, 1, 1); ok || episode.ID != 0 {
				t.Fatalf("invalid episode = %#v, %v", episode, ok)
			}
		})
	}
}

func TestTVMazeLocationRejectsQueryAndFragment(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"/shows/1?next=evil", "/shows/1#evil"} {
		location, err := url.Parse(raw)
		if err != nil || validTVMazeLocation(location) {
			t.Fatalf("unsafe location %q accepted, parse error %v", raw, err)
		}
	}
}
