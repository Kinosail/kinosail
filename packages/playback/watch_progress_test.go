package playback

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWatchProgressRejectsInvalidInputBeforeLookup(t *testing.T) {
	for _, tc := range []struct{ id, query string }{{"", ""}, {"../secret", ""}, {"movie?x=1", ""}, {"é", ""}, {strings.Repeat("a", 129), ""}, {"movie", "unknown=1"}, {"movie", "a=1&a=2"}} {
		t.Run(tc.id+tc.query, func(t *testing.T) {
			handler := WatchProgressHandler(func(*http.Request, string) (float64, float64, bool) {
				t.Fatal("rejected request reached media inspection")
				return 0, 0, false
			})
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/progress", nil)
			request.SetPathValue("id", tc.id)
			request.URL.RawQuery = tc.query
			response := httptest.NewRecorder()
			handler(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d", response.Code)
			}
		})
	}
}

func TestWatchProgressDoesNotExposeHiddenItem(t *testing.T) {
	handler := WatchProgressHandler(func(*http.Request, string) (float64, float64, bool) { return 50, 100, false })
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/progress", nil)
	request.SetPathValue("id", "hidden")
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "seconds") {
		t.Fatalf("hidden response=%d %s", response.Code, response.Body.String())
	}
}

func TestWatchProgressNormalizesMediaFacts(t *testing.T) {
	for _, tc := range []struct {
		name                                         string
		seconds, duration, wantSeconds, wantDuration float64
	}{
		{"resume", 1200, 3600, 1200, 3600},
		{"finished", 3700, 3600, 3600, 3600},
		{"unknown runtime", 1200, 0, 1200, 0},
		{"negative", -1, -2, 0, 0},
		{"nonfinite", math.NaN(), math.Inf(1), 0, 0},
		{"oversized", 315360001, 315360001, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := WatchProgressHandler(func(*http.Request, string) (float64, float64, bool) { return tc.seconds, tc.duration, true })
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/progress", nil)
			request.SetPathValue("id", "movie")
			response := httptest.NewRecorder()
			handler(response, request)
			var value map[string]float64
			if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
			if len(value) != 2 || value["seconds"] != tc.wantSeconds || value["duration"] != tc.wantDuration {
				t.Fatalf("progress=%v", value)
			}
			if response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("viewer progress must not enter shared caches")
			}
		})
	}
}
