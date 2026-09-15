package metadata

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestTVMazeRecordAndFillUseValidatedCachedProviderData(t *testing.T) { //nolint:cyclop,gocognit // The assertions cover one provider workflow.
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Header.Get("Accept") != "application/json" || !strings.HasPrefix(request.Header.Get("User-Agent"), "Kinosail/") {
			t.Errorf("provider headers = %#v", request.Header)
		}
		switch request.URL.Path {
		case "/lookup/shows":
			if request.URL.Query().Get("thetvdb") != "123" {
				t.Errorf("TVDB query = %q", request.URL.RawQuery)
			}
			writer.Header().Set("Location", "/shows/42")
			writer.WriteHeader(http.StatusFound)
		case "/shows/42":
			_, _ = writer.Write([]byte(`{"id":42,"name":"<b>Example &amp; Co</b>","premiered":"2021-01-02","summary":"<p>Show plot</p>"}`))
		case "/shows/42/episodebynumber":
			if request.URL.Query().Get("season") != "1" || request.URL.Query().Get("number") != "2" {
				t.Errorf("episode query = %q", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"id":77,"name":"<b>The Episode</b>","season":1,"number":2,"airdate":"2022-03-04","summary":"<p>Episode &amp; plot</p>"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	provider := NewTVMazeProvider(server.URL, "1.2.3")
	item := library.Item{Show: "Folder Show", Season: 1, Episode: 2, ProviderIDs: map[string]string{"tvdb": "123"}}
	result, ok := provider.Record(t.Context(), item)
	if !ok {
		t.Fatal("valid TVmaze episode was not resolved")
	}
	want := Record{Title: "S01E02 · The Episode", Plot: "Episode & plot", Year: "2022", ShowTitle: "Example & Co", ShowYear: "2021", ShowPlot: "Show plot", ProviderIDs: map[string]string{"tvdb": "123", "tvmaze": "77"}, ShowProviderIDs: map[string]string{"tvmaze": "42"}}
	if !reflect.DeepEqual(result.Record, want) {
		t.Fatalf("record = %#v, want %#v", result.Record, want)
	}
	preserved := Record{Title: "Local", Plot: "Local plot", Year: "2000", ShowTitle: "Local show", ShowYear: "1999", ShowPlot: "Local show plot", ProviderIDs: map[string]string{"tvdb": "local"}, ShowProviderIDs: map[string]string{"tvmaze": "local"}}
	provider.Fill(t.Context(), item, &preserved)
	if preserved.Title != "Local" || preserved.ProviderIDs["tvdb"] != "local" || preserved.ProviderIDs["tvmaze"] != "77" || preserved.ShowProviderIDs["tvmaze"] != "local" {
		t.Fatalf("fill overwrote local fields: %#v", preserved)
	}
	blank := Record{}
	provider.Fill(t.Context(), item, &blank)
	if !reflect.DeepEqual(blank, want) {
		t.Fatalf("filled record = %#v", blank)
	}
	if requests.Load() != 3 {
		t.Fatalf("cached provider made %d requests, want 3", requests.Load())
	}
}

func TestTVMazeRejectsInvalidInputsAndResponses(t *testing.T) { //nolint:cyclop,funlen // The table covers one provider trust boundary.
	t.Parallel()
	if NewTVMazeProvider("://bad") != nil || NewTVMazeProvider("http://example.com") != nil || NewTVMazeProvider("https://api.tvmaze.com/path") != nil {
		t.Fatal("invalid provider endpoint was accepted")
	}
	for _, raw := range []string{"https://api.tvmaze.com", "http://localhost:8080", "http://127.0.0.1:8080"} {
		endpoint, err := url.Parse(raw)
		if err != nil || !ValidTVMazeEndpoint(endpoint) {
			t.Fatalf("valid endpoint %q rejected", raw)
		}
	}
	invalidItems := []library.Item{
		{},
		{Show: "Show", Season: -1, Episode: 1, ProviderIDs: map[string]string{"tvdb": "1"}},
		{Show: "Show", Season: 1, Episode: 0, ProviderIDs: map[string]string{"tvdb": "1"}},
		{Show: "Show", Season: 1, Episode: 1, ProviderIDs: map[string]string{"tvdb": "no"}},
	}
	missing := httptest.NewServer(http.NotFoundHandler())
	defer missing.Close()
	provider := NewTVMazeProvider(missing.URL)
	for _, item := range invalidItems {
		if TVMazeEligible(item) {
			t.Fatalf("invalid item eligible: %#v", item)
		}
		if _, ok := provider.Record(t.Context(), item); ok {
			t.Fatalf("invalid item resolved: %#v", item)
		}
	}
	for name, response := range map[string]func(http.ResponseWriter, *http.Request){
		"lookup status": func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusOK) },
		"lookup host": func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Location", "https://evil.example/shows/1")
			writer.WriteHeader(http.StatusFound)
		},
		"lookup path": func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Location", "/people/1")
			writer.WriteHeader(http.StatusFound)
		},
		"lookup id": func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Location", "/shows/no")
			writer.WriteHeader(http.StatusFound)
		},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(response))
			defer server.Close()
			item := library.Item{Show: "Show", Season: 1, Episode: 1, ProviderIDs: map[string]string{"tvdb": "1"}}
			if _, ok := NewTVMazeProvider(server.URL).Record(t.Context(), item); ok {
				t.Fatal("invalid lookup response was accepted")
			}
		})
	}
}

func TestTVMazeRejectsMismatchedAndOversizedEpisodeData(t *testing.T) {
	t.Parallel()
	for name, episode := range map[string]string{
		"mismatch":     `{"id":2,"name":"Episode","season":9,"number":1,"airdate":"2020-01-01"}`,
		"missing name": `{"id":2,"name":"","season":1,"number":1,"airdate":"2020-01-01"}`,
		"long date":    `{"id":2,"name":"Episode","season":1,"number":1,"airdate":"2020-01-011"}`,
		"control":      fmt.Sprintf(`{"id":2,"name":%q,"season":1,"number":1,"airdate":"2020-01-01"}`, "bad\u0000name"),
		"plot control": fmt.Sprintf(`{"id":2,"name":"Episode","season":1,"number":1,"airdate":"2020-01-01","summary":%q}`, "bad\u0000plot"),
	} {
		t.Run(name, func(t *testing.T) {
			server := tvmazeFixtureServer(t, episode, `{"id":1,"name":"Show"}`)
			defer server.Close()
			item := library.Item{Show: "Show", Season: 1, Episode: 1, ProviderIDs: map[string]string{"tvdb": "1"}}
			if _, ok := NewTVMazeProvider(server.URL).Record(context.Background(), item); ok { //nolint:usetesting // Explicit context exercises provider ownership.
				t.Fatal("invalid episode data was accepted")
			}
		})
	}
}

func tvmazeFixtureServer(t *testing.T, episode, show string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/lookup/shows":
			writer.Header().Set("Location", "/shows/1")
			writer.WriteHeader(http.StatusFound)
		case "/shows/1":
			_, _ = writer.Write([]byte(show))
		case "/shows/1/episodebynumber":
			_, _ = writer.Write([]byte(episode))
		default:
			http.NotFound(writer, request)
		}
	}))
}
