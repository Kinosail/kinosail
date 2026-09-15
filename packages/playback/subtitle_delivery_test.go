package playback

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitlePath(t *testing.T) {
	t.Parallel()
	paths := []string{"first.srt", "second.vtt"}
	for value, want := range map[string]string{"": paths[0], "0": paths[0], "1": paths[1], "+1": paths[1]} {
		if path, ok := SubtitlePath(paths, value); !ok || path != want {
			t.Errorf("track %q = %q, %v", value, path, ok)
		}
	}
	for _, value := range []string{"bad", "-1", "2"} {
		if path, ok := SubtitlePath(paths, value); ok || path != "" {
			t.Errorf("accepted track %q as %q", value, path)
		}
	}
	if path, ok := SubtitlePath(nil, ""); ok || path != "" {
		t.Errorf("accepted an absent track as %q", path)
	}
}

func TestServeSubtitleDelivery(t *testing.T) { //nolint:cyclop // Direct, converted, and mapped delivery share one contract.
	t.Parallel()
	root := t.TempDir()
	srt := filepath.Join(root, "film.srt")
	vtt := filepath.Join(root, "film.VTT")
	if err := os.WriteFile(srt, []byte("1\r\n00:00:16,000 --> 00:00:18,000\r\nAfter intro\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vtt, []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\ndirect\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	parsed := 0
	dependencies := SubtitleDeliveryDependencies{
		ParseTimeline: func(value string) (Timeline, error) {
			parsed++
			if value != "valid" {
				t.Fatal("unexpected token", value)
			}
			return Timeline{SourceDuration: 20, Duration: 10, Omitted: []Range{{Start: 5, End: 15}}}, nil
		},
		NotFound: func(http.ResponseWriter, *http.Request) { t.Fatal("unexpected not-found callback") },
		Error: func(http.ResponseWriter, *http.Request, string, int) {
			t.Fatal("unexpected internal-error callback")
		},
	}

	direct := httptest.NewRecorder()
	ServeSubtitle(direct, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/id", nil), vtt, nil, dependencies)
	assertSubtitleResponse(t, direct, "00:00:01.000 --> 00:00:02.000", "direct")

	converted := httptest.NewRecorder()
	ServeSubtitle(converted, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/id", nil), srt, nil, dependencies)
	assertSubtitleResponse(t, converted, "WEBVTT\n\n1\n00:00:16.000 --> 00:00:18.000", "After intro")

	mapped := httptest.NewRecorder()
	ServeSubtitle(mapped, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/id?playbackToken=valid", nil), srt, nil, dependencies)
	assertSubtitleResponse(t, mapped, "00:00:06.000 --> 00:00:08.000", "After intro")
	if parsed != 1 {
		t.Fatalf("parsed timeline %d times", parsed)
	}

	explicit := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/id?playbackToken=ignored", nil)
	ServeSubtitle(explicit, request, vtt, []Timeline{{SourceDuration: 20, Duration: 20}}, dependencies)
	assertSubtitleResponse(t, explicit, "00:00:01.000 --> 00:00:02.000", "direct")
}

func TestServeSubtitleRejectsBeforeDelivery(t *testing.T) { //nolint:cyclop // Negative paths must prove that no subtitle body is emitted.
	t.Parallel()
	root := t.TempDir()
	tooLarge := filepath.Join(root, "large.srt")
	if err := os.WriteFile(tooLarge, []byte(strings.Repeat("x", maxSubtitleBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	callback := ""
	dependencies := SubtitleDeliveryDependencies{
		ParseTimeline: func(string) (Timeline, error) { return Timeline{}, os.ErrInvalid },
		NotFound:      func(http.ResponseWriter, *http.Request) { callback = "not-found" },
		Error:         func(http.ResponseWriter, *http.Request, string, int) { callback = "internal-error" },
	}
	for name, test := range map[string]struct {
		path, query, want string
	}{
		"empty path":     {path: "", want: "not-found"},
		"missing path":   {path: filepath.Join(root, "missing.srt"), want: "not-found"},
		"invalid token":  {path: tooLarge, query: "?playbackToken=bad", want: "not-found"},
		"oversized file": {path: tooLarge, want: "internal-error"},
		"read failure":   {path: root, want: "internal-error"},
	} {
		t.Run(name, func(t *testing.T) {
			callback = ""
			response := httptest.NewRecorder()
			ServeSubtitle(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/id"+test.query, nil), test.path, nil, dependencies)
			if callback != test.want || response.Body.Len() != 0 || response.Header().Get("Content-Type") != "" {
				t.Fatalf("callback = %q, body = %q, content type = %q", callback, response.Body.String(), response.Header().Get("Content-Type"))
			}
		})
	}
	response := httptest.NewRecorder()
	ServeSubtitle(response, nil, tooLarge, nil, SubtitleDeliveryDependencies{})
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "subtitle delivery is unavailable") {
		t.Fatalf("invalid dependencies = %d %q", response.Code, response.Body.String())
	}
}

func TestLibrarySubtitleHandlerRejectsBeforeDelivery(t *testing.T) { //nolint:cyclop // Route failures must not start file delivery.
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/item/bad", nil)
	request.SetPathValue("id", "item")
	request.SetPathValue("track", "bad")
	lookup, safe, notFound := 0, 0, 0
	delivery := SubtitleDeliveryDependencies{NotFound: func(http.ResponseWriter, *http.Request) { notFound++ }}
	dependencies := LibrarySubtitleDependencies{
		Lookup: func(*http.Request, string) (library.Item, bool) {
			lookup++
			return library.Item{Subtitles: []string{"safe.srt"}}, true
		},
		Safe:     func(string) bool { safe++; return false },
		Delivery: delivery,
	}
	response := httptest.NewRecorder()
	LibrarySubtitleHandler(dependencies)(response, request)
	if lookup != 1 || safe != 0 || notFound != 1 || response.Body.Len() != 0 {
		t.Fatalf("invalid track side effects = lookup %d, safe %d, not found %d", lookup, safe, notFound)
	}

	request.SetPathValue("track", "0")
	response = httptest.NewRecorder()
	LibrarySubtitleHandler(dependencies)(response, request)
	if lookup != 2 || safe != 1 || notFound != 2 || response.Body.Len() != 0 {
		t.Fatalf("unsafe path side effects = lookup %d, safe %d, not found %d", lookup, safe, notFound)
	}

	dependencies.Lookup = func(*http.Request, string) (library.Item, bool) { return library.Item{}, false }
	response = httptest.NewRecorder()
	LibrarySubtitleHandler(dependencies)(response, request)
	if notFound != 3 || response.Body.Len() != 0 {
		t.Fatalf("missing item = not found %d, body %q", notFound, response.Body.String())
	}

	for name, invalid := range map[string]LibrarySubtitleDependencies{
		"missing config": {},
		"missing callback": {
			Lookup: func(*http.Request, string) (library.Item, bool) { return library.Item{}, false },
			Safe:   func(string) bool { return true },
		},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			LibrarySubtitleHandler(invalid)(response, request)
			if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "subtitle delivery is unavailable") {
				t.Fatalf("invalid config = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestLibrarySubtitleHandlerDeliversSelectedTrack(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "film.srt")
	if err := os.WriteFile(path, []byte("1\n00:00:01,000 --> 00:00:02,000\nline\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/item/0", nil)
	request.SetPathValue("id", "item")
	request.SetPathValue("track", "0")
	response := httptest.NewRecorder()
	LibrarySubtitleHandler(LibrarySubtitleDependencies{
		Lookup: func(*http.Request, string) (library.Item, bool) {
			return library.Item{Subtitles: []string{path}}, true
		},
		Safe: func(candidate string) bool { return candidate == path },
		Delivery: SubtitleDeliveryDependencies{
			ParseTimeline: func(string) (Timeline, error) { return Timeline{}, nil },
			NotFound:      func(http.ResponseWriter, *http.Request) { t.Fatal("unexpected not found") },
			Error:         func(http.ResponseWriter, *http.Request, string, int) { t.Fatal("unexpected error") },
		},
	})(response, request)
	assertSubtitleResponse(t, response, "WEBVTT", "00:00:01.000 --> 00:00:02.000", "line")
}

func assertSubtitleResponse(t *testing.T, response *httptest.ResponseRecorder, values ...string) {
	t.Helper()
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/vtt; charset=utf-8" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("response = %d, headers %#v", response.Code, response.Header())
	}
	for _, value := range values {
		if !strings.Contains(response.Body.String(), value) {
			t.Fatalf("subtitle %q does not contain %q", response.Body.String(), value)
		}
	}
}
