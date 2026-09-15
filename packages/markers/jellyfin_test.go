package markers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJellyfinSegmentsExposeOnlyAuthoritativeMarkerTypes(t *testing.T) {
	t.Parallel()
	segments := JellyfinSegments("episode", []Marker{
		{Type: "intro", Source: "chapter", Start: 1, End: 2},
		{Type: "credits", Source: "manual", Start: 90, End: 100},
		{Type: "recap", Source: "visual", Start: 0, End: 1},
		{Type: "unknown", Source: "chapter", Start: 2, End: 3},
	})
	if len(segments) != 2 || segments[0].Type != "Intro" || segments[1].Type != "Outro" || segments[0].ItemID != "episode" || len(segments[0].ID) != 32 || segments[0].StartTicks != 1e7 || segments[0].EndTicks != 2e7 {
		t.Fatalf("segments = %#v", segments)
	}
	if repeat := JellyfinSegments("episode", []Marker{{Type: "intro", Source: "chapter", Start: 1, End: 2}}); repeat[0].ID != segments[0].ID {
		t.Fatalf("segment ID is unstable: %q != %q", repeat[0].ID, segments[0].ID)
	}
}

func TestJellyfinHandlerBoundsPathAndWritesResponse(t *testing.T) {
	t.Parallel()
	calls := 0
	handler := JellyfinHandler(func(*http.Request) (string, []Marker, bool) {
		calls++
		return "episode", []Marker{{Type: "intro", Source: "chapter", Start: 1, End: 2}}, true
	}, func(writer http.ResponseWriter, value any) { _ = json.NewEncoder(writer).Encode(value) })
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/MediaSegments/episode", nil)
	request.SetPathValue("id", "episode")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || calls != 1 || !strings.Contains(response.Body.String(), `"TotalRecordCount":1`) {
		t.Fatalf("response = %d %q, calls=%d", response.Code, response.Body.String(), calls)
	}
	for _, id := range []string{"", strings.Repeat("x", 257)} {
		request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/MediaSegments/x", nil)
		request.SetPathValue("id", id)
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound || calls != 1 {
			t.Fatalf("invalid ID response = %d, calls=%d", response.Code, calls)
		}
	}
}

func TestJellyfinHandlerPreservesResolverNotFound(t *testing.T) {
	t.Parallel()
	handler := JellyfinHandler(func(*http.Request) (string, []Marker, bool) { return "", nil, false }, func(http.ResponseWriter, any) { t.Fatal("writer called") })
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/MediaSegments/missing", nil)
	request.SetPathValue("id", "missing")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("response = %d", response.Code)
	}
}
