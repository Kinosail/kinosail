package playback

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestTrickplayRegistrationOwnsRouteAndResponseMapping(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.mkv")
	writeTestFile(t, source, "source")
	target := filepath.Join(root, "trickplay", "video", "20.jpg")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, target, "jpeg")
	if err := os.Chtimes(target, time.Now().Add(time.Second), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	lookups := 0
	lookup := func(_ *http.Request, id string) (library.Item, bool) {
		lookups++
		switch id {
		case "video":
			return library.Item{ID: id, Kind: "video", Path: source}, true
		case "unavailable":
			return library.Item{ID: id, Kind: "video", Path: source}, true
		default:
			return library.Item{}, false
		}
	}
	failures := 0
	failure := func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
		failures++
		http.Error(writer, message, status)
	}
	mux := http.NewServeMux()
	MustTrickplayRegistration(root, "/missing", HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}, lookup, failure)(mux)

	assertTrickplayResponse(t, mux, http.MethodGet, "/trickplay/video/27", http.StatusOK, "image/jpeg", "jpeg")
	assertTrickplayResponse(t, mux, http.MethodGet, "/trickplay/missing/27", http.StatusNotFound, "text/plain; charset=utf-8", "not found\n")
	assertTrickplayResponse(t, mux, http.MethodGet, "/trickplay/unavailable/27", http.StatusServiceUnavailable, "text/plain; charset=utf-8", "preview unavailable\n")
	wrongMethod := assertTrickplayResponse(t, mux, http.MethodPost, "/trickplay/video/27", http.StatusMethodNotAllowed, "text/plain; charset=utf-8", "Method Not Allowed\n")
	if wrongMethod.Code != http.StatusMethodNotAllowed || lookups != 3 || failures != 2 {
		t.Fatalf("method = %d, lookups = %d, failures = %d", wrongMethod.Code, lookups, failures)
	}
}

func assertTrickplayResponse(t *testing.T, handler http.Handler, method, path string, status int, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), method, path, nil))
	if response.Code != status || response.Header().Get("Content-Type") != contentType || response.Body.String() != body {
		t.Fatalf("response = %d %q %q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
	return response
}

func TestTrickplayRegistrationRejectsDependenciesBeforeCallbacks(t *testing.T) {
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	effects := 0
	lookup := func(*http.Request, string) (library.Item, bool) {
		effects++
		return library.Item{ID: "unused"}, false
	}
	failure := func(http.ResponseWriter, *http.Request, string, int) { effects++ }
	for name, register := range map[string]func(){
		"lookup":  func() { MustTrickplayRegistration("cache", "ffmpeg", policy, nil, failure) },
		"failure": func() { MustTrickplayRegistration("cache", "ffmpeg", policy, lookup, nil) },
		"policy":  func() { MustTrickplayRegistration("cache", "ffmpeg", HLSRecipePolicy{}, lookup, failure) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid registration did not panic")
				}
				if effects != 0 {
					t.Fatalf("invalid registration caused %d callback effects", effects)
				}
			}()
			register()
		})
	}
}
