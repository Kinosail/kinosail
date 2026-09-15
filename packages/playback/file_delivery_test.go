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

func TestLibraryFileHandler(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "media.bin")
	if err := os.WriteFile(path, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/item", nil)
	request.SetPathValue("id", "item")
	selected := 0
	dependencies := LibraryFileDependencies{
		Lookup: func(_ *http.Request, id string) (library.Item, bool) {
			return library.Item{ID: id, Path: path}, true
		},
		Select: func(item library.Item) string { selected++; return item.Path },
		Safe:   func(candidate string) bool { return candidate == path },
		NotFound: func(http.ResponseWriter, *http.Request) {
			t.Fatal("unexpected not-found callback")
		},
		ContentType: "application/octet-stream",
	}
	response := httptest.NewRecorder()
	LibraryFileHandler(dependencies)(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "media" || response.Header().Get("Content-Type") != dependencies.ContentType || selected != 1 {
		t.Fatalf("response = %d %q %#v, selected = %d", response.Code, response.Body.String(), response.Header(), selected)
	}

	dependencies.ContentType = ""
	response = httptest.NewRecorder()
	LibraryFileHandler(dependencies)(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("inferred content type = %d %q", response.Code, response.Header().Get("Content-Type"))
	}
}

func TestLibraryFileHandlerRejectsBeforeSelection(t *testing.T) { //nolint:cyclop // Each rejection proves later callbacks do not run.
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/missing", nil)
	request.SetPathValue("id", "missing")
	selected, safe, notFound := 0, 0, 0
	dependencies := LibraryFileDependencies{
		Lookup: func(*http.Request, string) (library.Item, bool) {
			return library.Item{}, false
		},
		Select: func(library.Item) string { selected++; return "path" },
		Safe:   func(string) bool { safe++; return true },
		NotFound: func(http.ResponseWriter, *http.Request) {
			notFound++
		},
	}
	response := httptest.NewRecorder()
	LibraryFileHandler(dependencies)(response, request)
	if selected != 0 || safe != 0 || notFound != 1 || response.Body.Len() != 0 {
		t.Fatalf("missing item side effects = select %d, safe %d, not found %d, body %q", selected, safe, notFound, response.Body.String())
	}

	dependencies.Lookup = func(*http.Request, string) (library.Item, bool) { return library.Item{}, true }
	dependencies.Select = func(library.Item) string { selected++; return "" }
	response = httptest.NewRecorder()
	LibraryFileHandler(dependencies)(response, request)
	if selected != 1 || safe != 0 || notFound != 2 || response.Body.Len() != 0 {
		t.Fatalf("empty path side effects = select %d, safe %d, not found %d", selected, safe, notFound)
	}

	dependencies.Select = func(library.Item) string { selected++; return "unsafe" }
	dependencies.Safe = func(string) bool { safe++; return false }
	response = httptest.NewRecorder()
	LibraryFileHandler(dependencies)(response, request)
	if selected != 2 || safe != 1 || notFound != 3 || response.Body.Len() != 0 {
		t.Fatalf("unsafe path side effects = select %d, safe %d, not found %d", selected, safe, notFound)
	}

	for name, invalid := range map[string]LibraryFileDependencies{
		"missing dependencies": {},
		"invalid content type": {ContentType: "text/plain\r\nX-Test: bad"},
		"long content type":    {ContentType: strings.Repeat("x", 257)},
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			LibraryFileHandler(invalid)(response, nil)
			if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "file delivery is unavailable") {
				t.Fatalf("invalid config = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestLibraryPersonHandler(t *testing.T) { //nolint:cyclop // Success and all route boundaries share one fixture.
	t.Parallel()
	image := filepath.Join(t.TempDir(), "person.jpg")
	if err := os.WriteFile(image, []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	item := library.Item{Cast: []library.Person{{Image: image}, {}}}
	lookup := func(*http.Request, string) (library.Item, bool) { return item, true }
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/person/item/0", nil)
	request.SetPathValue("id", "item")
	request.SetPathValue("person", "0")
	response := httptest.NewRecorder()
	LibraryPersonHandler(lookup)(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "image" {
		t.Fatalf("person image = %d %q", response.Code, response.Body.String())
	}

	for _, value := range []string{"bad", "-1", "1", "2"} {
		request.SetPathValue("person", value)
		response = httptest.NewRecorder()
		LibraryPersonHandler(lookup)(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("person %q = %d", value, response.Code)
		}
	}
	response = httptest.NewRecorder()
	LibraryPersonHandler(func(*http.Request, string) (library.Item, bool) { return library.Item{}, false })(response, request)
	if response.Code != http.StatusNotFound {
		t.Errorf("missing item = %d", response.Code)
	}
	response = httptest.NewRecorder()
	LibraryPersonHandler(nil)(response, nil)
	if response.Code != http.StatusInternalServerError {
		t.Errorf("invalid config = %d", response.Code)
	}
}

func TestLibraryDownloadHandler(t *testing.T) { //nolint:cyclop // Authorization, visibility, and delivery order are one contract.
	t.Parallel()
	path := filepath.Join(t.TempDir(), "Film.mp4")
	if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/download/item", nil)
	request.SetPathValue("id", "item")
	allowed, lookup, notFound, failed := true, 0, 0, ""
	dependencies := LibraryDownloadDependencies{
		Allowed: func(*http.Request) bool { return allowed },
		Lookup: func(*http.Request, string) (library.Item, bool) {
			lookup++
			return library.Item{Path: path}, true
		},
		NotFound: func(http.ResponseWriter, *http.Request) { notFound++ },
		Error: func(_ http.ResponseWriter, _ *http.Request, message string, status int) {
			failed = message + ":" + http.StatusText(status)
		},
	}
	response := httptest.NewRecorder()
	LibraryDownloadHandler(dependencies)(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "video" || response.Header().Get("Content-Disposition") != `attachment; filename="Film.mp4"` || lookup != 1 {
		t.Fatalf("download = %d %q %#v, lookup = %d", response.Code, response.Body.String(), response.Header(), lookup)
	}

	allowed = false
	response = httptest.NewRecorder()
	LibraryDownloadHandler(dependencies)(response, request)
	if lookup != 1 || failed != "downloads are not enabled for this Viewer Profile:Forbidden" || response.Body.Len() != 0 {
		t.Fatalf("forbidden side effects = lookup %d, error %q, body %q", lookup, failed, response.Body.String())
	}

	allowed = true
	dependencies.Lookup = func(*http.Request, string) (library.Item, bool) { lookup++; return library.Item{}, false }
	response = httptest.NewRecorder()
	LibraryDownloadHandler(dependencies)(response, request)
	if lookup != 2 || notFound != 1 || response.Body.Len() != 0 {
		t.Fatalf("missing side effects = lookup %d, not found %d, body %q", lookup, notFound, response.Body.String())
	}

	response = httptest.NewRecorder()
	LibraryDownloadHandler(LibraryDownloadDependencies{})(response, nil)
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "file delivery is unavailable") {
		t.Fatalf("invalid config = %d %q", response.Code, response.Body.String())
	}
}

func TestLibraryItemPaths(t *testing.T) {
	t.Parallel()
	item := library.Item{Path: "media", Artwork: "art", Backdrop: "back", ShowArtwork: "show-art", ShowBackdrop: "show-back"}
	if MediaPath(item) != "media" || ArtworkPath(item) != "show-art" || BackdropPath(item) != "show-back" {
		t.Fatalf("show paths = %q %q %q", MediaPath(item), ArtworkPath(item), BackdropPath(item))
	}
	item.ShowArtwork, item.ShowBackdrop = "", ""
	if ArtworkPath(item) != "art" || BackdropPath(item) != "back" {
		t.Fatalf("item paths = %q %q", ArtworkPath(item), BackdropPath(item))
	}
	item.Backdrop = ""
	if BackdropPath(item) != "art" {
		t.Fatalf("artwork fallback = %q", BackdropPath(item))
	}
}

func TestShowPersonImageScopeAndVisibility(t *testing.T) {
	image := filepath.Join(t.TempDir(), "actor.jpg")
	if err := os.WriteFile(image, []byte("show actor"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"scope=show", "scope=unknown", "scope=show&scope=show", "scope=show&extra=1", "scope=", "scope=" + strings.Repeat("x", 4096)} {
		for _, visible := range []bool{true, false} {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/person/episode/0?"+query, nil)
			request.SetPathValue("person", "0")
			response := httptest.NewRecorder()
			LibraryPersonHandler(func(*http.Request, string) (library.Item, bool) {
				return library.Item{ShowCast: []library.Person{{Image: image}}}, visible
			})(response, request)
			if query == "scope=show" && visible {
				if response.Code != http.StatusOK || response.Body.String() != "show actor" {
					t.Fatalf("show image = %d %q", response.Code, response.Body.String())
				}
			} else if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "show actor") {
				t.Fatalf("invalid scope or visibility served image: %d", response.Code)
			}
		}
	}
}
