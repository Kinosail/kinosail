package markers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestAdminRoutesManageMarkersAndQueueAnalysis(t *testing.T) { //nolint:cyclop // The assertions exercise one HTTP workflow.
	t.Parallel()
	item := library.Item{ID: "movie", Size: 1, Added: time.Unix(1, 0)}
	analyzer := NewAnalyzer(Config{})
	mux := http.NewServeMux()
	RegisterAdmin(mux, testAdmin(analyzer, item, nil))
	response := markerRequest(t, mux, http.MethodPut, "/api/v1/items/movie/markers", "application/json", `{"type":"intro","start":1,"end":10}`)
	if response.Code != http.StatusOK {
		t.Fatalf("save status = %d: %s", response.Code, response.Body.String())
	}
	if markers := analyzer.Markers(item, nil); len(markers) != 1 || markers[0].Source != "manual" {
		t.Fatalf("saved markers = %#v", markers)
	}
	response = markerRequest(t, mux, http.MethodDelete, "/api/v1/items/movie/markers/intro", "", "")
	if response.Code != http.StatusNoContent || len(analyzer.Markers(item, nil)) != 0 {
		t.Fatalf("delete status = %d, markers = %#v", response.Code, analyzer.Markers(item, nil))
	}
	response = markerRequest(t, mux, http.MethodGet, "/api/v1/marker-analysis", "", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"detectorVersion":`+strconv.Itoa(DetectorVersion)) {
		t.Fatalf("status response = %d: %s", response.Code, response.Body.String())
	}
	response = markerRequest(t, mux, http.MethodPost, "/api/v1/marker-analysis", "", "")
	if response.Code != http.StatusAccepted {
		t.Fatalf("queue status = %d: %s", response.Code, response.Body.String())
	}
	response = markerRequest(t, mux, http.MethodPost, "/settings/marker-analysis", "", "")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/settings" {
		t.Fatalf("redirect = %d %q", response.Code, response.Header().Get("Location"))
	}
}

func TestAdminRoutesRejectInvalidInputWithoutSideEffects(t *testing.T) { //nolint:funlen // All cases prove one strict HTTP mutation boundary.
	t.Parallel()
	item := library.Item{ID: "movie", Size: 1, Added: time.Unix(1, 0)}
	for name, request := range map[string]*http.Request{
		"wrong content": httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/items/movie/markers", strings.NewReader(`{"type":"intro","start":1,"end":10}`)),
		"unknown JSON":  jsonMarkerRequest(t, `/api/v1/items/movie/markers`, `{"type":"intro","start":1,"end":10,"extra":true}`),
		"oversized":     jsonMarkerRequest(t, `/api/v1/items/movie/markers`, `{"type":"intro","start":1,"end":10,"padding":"`+strings.Repeat("x", 5000)+`"}`),
		"unknown type":  jsonMarkerRequest(t, `/api/v1/items/movie/markers`, `{"type":"unknown","start":1,"end":10}`),
		"bad range":     jsonMarkerRequest(t, `/api/v1/items/movie/markers`, `{"type":"intro","start":10,"end":1}`),
		"query":         jsonMarkerRequest(t, `/api/v1/items/movie/markers?x=1`, `{"type":"intro","start":1,"end":10}`),
	} {
		t.Run(name, func(t *testing.T) {
			analyzer := NewAnalyzer(Config{})
			mux := http.NewServeMux()
			RegisterAdmin(mux, testAdmin(analyzer, item, nil))
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || len(analyzer.Markers(item, nil)) != 0 {
				t.Fatalf("status = %d, markers = %#v", response.Code, analyzer.Markers(item, nil))
			}
		})
	}
	for name, body := range map[string]string{
		"duplicate": "type=intro&type=credits&start=1&end=10",
		"extra":     "type=intro&start=1&end=10&extra=x",
		"missing":   "type=intro&start=1",
		"long":      "type=" + strings.Repeat("x", 33) + "&start=1&end=10",
		"number":    "type=intro&start=no&end=10",
	} {
		t.Run("form "+name, func(t *testing.T) {
			analyzer := NewAnalyzer(Config{})
			mux := http.NewServeMux()
			RegisterAdmin(mux, testAdmin(analyzer, item, nil))
			response := markerRequest(t, mux, http.MethodPost, "/markers/movie", "application/x-www-form-urlencoded", body)
			if response.Code != http.StatusBadRequest || len(analyzer.Markers(item, nil)) != 0 {
				t.Fatalf("status = %d, markers = %#v", response.Code, analyzer.Markers(item, nil))
			}
		})
	}
}

func TestAdminRoutesPreserveNotFoundAndUnavailableResponses(t *testing.T) {
	t.Parallel()
	analyzer := NewAnalyzer(Config{})
	mux := http.NewServeMux()
	RegisterAdmin(mux, testAdmin(analyzer, library.Item{}, errors.New("offline")))
	response := markerRequest(t, mux, http.MethodPut, "/api/v1/items/missing/markers", "application/json", `{"type":"intro","start":1,"end":10}`)
	if response.Code != http.StatusNotFound {
		t.Fatalf("not found status = %d", response.Code)
	}
	response = markerRequest(t, mux, http.MethodPut, "/api/v1/items/"+strings.Repeat("x", 257)+"/markers", "application/json", `{"type":"intro","start":1,"end":10}`)
	if response.Code != http.StatusNotFound {
		t.Fatalf("oversized ID status = %d", response.Code)
	}
	response = markerRequest(t, mux, http.MethodPost, "/api/v1/marker-analysis", "", "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d", response.Code)
	}
}

func testAdmin(analyzer *Analyzer, item library.Item, snapshotErr error) Admin {
	return Admin{
		Analyzer: analyzer,
		Owner:    func(next http.Handler) http.Handler { return next },
		Find:     func(_ *http.Request, id string) (library.Item, bool) { return item, item.ID != "" && id == item.ID },
		Snapshot: func() ([]library.Item, error) { return []library.Item{item}, snapshotErr },
		Duration: func(*http.Request, library.Item) float64 { return 100 },
		JSON: func(writer http.ResponseWriter, value any, status int) {
			writer.WriteHeader(status)
			_ = json.NewEncoder(writer).Encode(value)
		},
		Error: func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
			http.Error(writer, message, status)
		},
		NotFound: func(writer http.ResponseWriter, _ *http.Request) { http.NotFound(writer, nil) },
	}
}

func markerRequest(t *testing.T, handler http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func jsonMarkerRequest(t *testing.T, path, body string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestMarkerFormHelpers(t *testing.T) {
	t.Parallel()
	form := url.Values{"type": {"intro"}, "start": {"1"}, "end": {"2"}}
	if !onlyMarkerFormKeys(form, "type", "start", "end") || onlyMarkerFormKeys(form, "type") {
		t.Fatal("form key validation mismatch")
	}
}
