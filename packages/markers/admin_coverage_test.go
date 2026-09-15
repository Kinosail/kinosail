package markers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestAdminFormSaveAndRemoveLifecycle(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: "movie", Size: 1, Added: time.Unix(1, 0)}
	analyzer := NewAnalyzer(Config{})
	mux := http.NewServeMux()
	RegisterAdmin(mux, testAdmin(analyzer, item, nil))
	response := markerRequest(t, mux, http.MethodPost, "/markers/movie", "application/x-www-form-urlencoded", "type=intro&start=1&end=10")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/watch/movie" || len(analyzer.Markers(item, nil)) != 1 {
		t.Fatalf("form save = %d %q, markers %#v", response.Code, response.Header().Get("Location"), analyzer.Markers(item, nil))
	}
	response = markerRequest(t, mux, http.MethodPost, "/markers/movie/remove", "application/x-www-form-urlencoded", "type=intro")
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/watch/movie" || len(analyzer.Markers(item, nil)) != 0 {
		t.Fatalf("form remove = %d %q, markers %#v", response.Code, response.Header().Get("Location"), analyzer.Markers(item, nil))
	}
}

func TestAdminRemoveRejectsEveryBoundaryWithoutMutation(t *testing.T) { //nolint:gocognit // One table proves every removal boundary is side-effect free.
	t.Parallel()
	item := library.Item{ID: "movie", Size: 1, Added: time.Unix(1, 0)}
	tests := []struct {
		name, id, method, contentType, query, body, markerType string
		found                                                  bool
		status                                                 int
	}{
		{"invalid identity", "bad/id", http.MethodDelete, "", "", "", "intro", true, http.StatusNotFound},
		{"missing item", "movie", http.MethodDelete, "", "", "", "intro", false, http.StatusNotFound},
		{"wrong content type", "movie", http.MethodPost, "text/plain", "", "type=intro", "", true, http.StatusBadRequest},
		{"query", "movie", http.MethodPost, "application/x-www-form-urlencoded", "extra=1", "type=intro", "", true, http.StatusBadRequest},
		{"unknown field", "movie", http.MethodPost, "application/x-www-form-urlencoded", "", "type=intro&extra=x", "", true, http.StatusBadRequest},
		{"missing type", "movie", http.MethodPost, "application/x-www-form-urlencoded", "", "type=", "", true, http.StatusBadRequest},
		{"invalid delete type", "movie", http.MethodDelete, "", "", "", "unknown", true, http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer := NewAnalyzer(Config{})
			if err := analyzer.SetManual(item, "intro", 1, 10, 100); err != nil {
				t.Fatal(err)
			}
			admin := testAdmin(analyzer, item, nil)
			admin.Find = func(*http.Request, string) (library.Item, bool) { return item, test.found }
			request := httptest.NewRequestWithContext(t.Context(), test.method, "/remove?"+test.query, strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			request.SetPathValue("id", test.id)
			request.SetPathValue("type", test.markerType)
			response := httptest.NewRecorder()
			admin.removeManual(response, request)
			if response.Code != test.status || len(analyzer.Markers(item, nil)) != 1 {
				t.Fatalf("remove response = %d, markers %#v", response.Code, analyzer.Markers(item, nil))
			}
		})
	}
}

func TestAdminMarkerFormHelperBoundaries(t *testing.T) {
	t.Parallel()
	if onlyMarkerFormKeys(url.Values{"type": {"intro"}, "wrong": {"1"}}, "type", "start") {
		t.Fatal("missing required form key was accepted")
	}
	if number, ok := markerNumber(url.Values{}, "start"); ok || number != 0 {
		t.Fatalf("missing marker number = %v, %v", number, ok)
	}
}
