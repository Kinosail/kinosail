package catalogapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func registeredMediaHandler(handlers MediaHandlers) http.Handler {
	mux := http.NewServeMux()
	noContent := func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }
	handlers.Register(mux, MediaExtras{PlaybackEvents: noContent, Reader: noContent, ReaderProgress: noContent, Owner: func(handler http.Handler) http.Handler { return handler }})
	return mux
}

func callMedia(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestMediaHandlersServePlayerCatalogRoutes(t *testing.T) { //nolint:cyclop,funlen,gocognit // One matrix proves the complete shared route interface.
	items := mediaFixture()
	handlers, _, progress, lists := mediaHandlersForTest(items)
	server := registeredMediaHandler(handlers)
	_, shows := library.Organize(items)
	_, albums := library.OrganizeMusic(items)
	tests := []struct {
		method, target, body string
		status               int
	}{
		{http.MethodGet, "/api/v1/items/movie", "", http.StatusOK},
		{http.MethodPut, "/api/v1/items/movie/progress", `{"seconds":5}`, http.StatusOK},
		{http.MethodPost, "/api/v1/items/movie/playback-events", "", http.StatusNoContent},
		{http.MethodDelete, "/api/v1/items/movie/continue-watching", "", http.StatusNoContent},
		{http.MethodGet, "/api/v1/history", "", http.StatusOK},
		{http.MethodGet, "/api/v1/audio/track-one/queue", "", http.StatusOK},
		{http.MethodGet, "/api/v1/books/book/reader", "", http.StatusNoContent},
		{http.MethodGet, "/api/v1/books/book/reader/progress", "", http.StatusNoContent},
		{http.MethodPut, "/api/v1/books/book/reader/progress", "", http.StatusNoContent},
		{http.MethodPut, "/api/v1/items/movie/list", `{"listed":true}`, http.StatusOK},
		{http.MethodGet, "/api/v1/playlists", "", http.StatusOK},
		{http.MethodPost, "/api/v1/playlists", `{"name":" Queue ","ids":["movie"]}`, http.StatusCreated},
		{http.MethodPost, "/api/v1/smart-playlists", `{"name":" Recent ","kind":"video","query":"movie","sort":"title"}`, http.StatusCreated},
		{http.MethodGet, "/api/v1/playlists/Queue", "", http.StatusOK},
		{http.MethodGet, "/api/v1/playlists/Queue?format=kinosail", "", http.StatusOK},
		{http.MethodDelete, "/api/v1/playlists/Queue", "", http.StatusNoContent},
		{http.MethodPut, "/api/v1/playlists/Queue/items/movie", `{"included":true}`, http.StatusOK},
		{http.MethodPut, "/api/v1/playlists/Queue/order", `{"ids":["movie"]}`, http.StatusOK},
		{http.MethodGet, "/api/v1/shows", "", http.StatusOK},
		{http.MethodGet, "/api/v1/shows/" + shows[0].ID, "", http.StatusOK},
		{http.MethodGet, "/api/v1/albums", "", http.StatusOK},
		{http.MethodGet, "/api/v1/albums/" + albums[0].ID, "", http.StatusOK},
		{http.MethodGet, "/api/v1/collections", "", http.StatusOK},
		{http.MethodPost, "/api/v1/collections", `{"name":"Queue"}`, http.StatusCreated},
		{http.MethodGet, "/api/v1/collections/Queue", "", http.StatusOK},
		{http.MethodDelete, "/api/v1/collections/Queue", "", http.StatusNoContent},
		{http.MethodPut, "/api/v1/collections/Queue/items/movie", `{"included":true}`, http.StatusOK},
	}
	for _, test := range tests {
		response := callMedia(t, server, test.method, test.target, test.body)
		if response.Code != test.status {
			t.Fatalf("%s %s = %d, want %d, body=%q", test.method, test.target, response.Code, test.status, response.Body.String())
		}
	}
	if progress.itemCalls == 0 || progress.itemsCalls == 0 || progress.dismisses != 1 || progress.progressSet != 1 || lists.creates != 1 || lists.smart != 1 || lists.orders != 1 || lists.deletes != 1 || lists.listed != 1 || lists.membership != 1 {
		t.Fatalf("progress=%#v lists=%#v", progress, lists)
	}
	if response := callMedia(t, server, http.MethodGet, "/api/v1/playlists/Queue?format=kinosail", ""); response.Header().Get("Content-Disposition") == "" {
		t.Fatal("export response omitted the attachment header")
	}
}

func TestMediaHandlersRejectInvalidInputsBeforeSideEffects(t *testing.T) { //nolint:cyclop,funlen,gocognit // One matrix proves every shared request rejection path.
	items := mediaFixture()
	handlers, index, progress, lists := mediaHandlersForTest(items)
	server := registeredMediaHandler(handlers)
	invalidJSON := []struct {
		method, target string
	}{
		{http.MethodPut, "/api/v1/items/movie/progress"},
		{http.MethodPut, "/api/v1/items/movie/list"},
		{http.MethodPost, "/api/v1/playlists"},
		{http.MethodPost, "/api/v1/smart-playlists"},
		{http.MethodPut, "/api/v1/playlists/Queue/items/movie"},
		{http.MethodPut, "/api/v1/playlists/Queue/order"},
	}
	for _, test := range invalidJSON {
		if response := callMedia(t, server, test.method, test.target, "{"); response.Code != http.StatusBadRequest {
			t.Fatalf("%s %s = %d, body=%q", test.method, test.target, response.Code, response.Body.String())
		}
	}
	oversized := strings.Repeat(" ", 1<<20) + `{}`
	if response := callMedia(t, server, http.MethodPost, "/api/v1/playlists", oversized); response.Code != http.StatusBadRequest {
		t.Fatalf("oversized playlist = %d, body=%q", response.Code, response.Body.String())
	}
	if progress.progressSet != 0 || lists.creates != 0 || lists.smart != 0 || lists.orders != 0 || lists.listed != 0 || lists.membership != 0 {
		t.Fatalf("invalid input caused side effects: progress=%#v lists=%#v", progress, lists)
	}

	for _, test := range []struct {
		method, target, body string
	}{
		{http.MethodGet, "/api/v1/audio/missing/queue", ""},
		{http.MethodGet, "/api/v1/shows/missing", ""},
		{http.MethodGet, "/api/v1/albums/missing", ""},
		{http.MethodGet, "/api/v1/playlists/Missing", ""},
		{http.MethodGet, "/api/v1/playlists/Queue?format=other", ""},
	} {
		response := callMedia(t, server, test.method, test.target, test.body)
		if response.Code != http.StatusNotFound && test.target != "/api/v1/playlists/Queue?format=other" || response.Code != http.StatusBadRequest && test.target == "/api/v1/playlists/Queue?format=other" {
			t.Fatalf("%s = %d, body=%q", test.target, response.Code, response.Body.String())
		}
	}
	lists.exportErr = errors.New("missing")
	if response := callMedia(t, server, http.MethodGet, "/api/v1/playlists/Queue?format=kinosail", ""); response.Code != http.StatusNotFound {
		t.Fatalf("missing export = %d", response.Code)
	}
	lists.exportErr = nil
	lists.names = nil
	if response := callMedia(t, server, http.MethodGet, "/api/v1/playlists/Queue", ""); response.Code != http.StatusNotFound {
		t.Fatalf("missing playlist = %d", response.Code)
	}

	index.found = false
	missing := []struct {
		method, target, body string
	}{
		{http.MethodGet, "/api/v1/items/missing", ""},
		{http.MethodPut, "/api/v1/items/missing/progress", `{"seconds":1}`},
		{http.MethodDelete, "/api/v1/items/missing/continue-watching", ""},
		{http.MethodPut, "/api/v1/items/missing/list", `{"listed":true}`},
		{http.MethodPut, "/api/v1/playlists/Queue/items/missing", `{"included":true}`},
	}
	for _, test := range missing {
		if response := callMedia(t, server, test.method, test.target, test.body); response.Code != http.StatusNotFound {
			t.Fatalf("missing %s = %d", test.target, response.Code)
		}
	}
}

func TestMediaHandlersTranslateOperationFailures(t *testing.T) { //nolint:cyclop,funlen,gocognit // One matrix covers app-store and timeline error translation.
	items := mediaFixture()
	handlers, _, _, _ := mediaHandlersForTest(items)
	server := registeredMediaHandler(handlers)
	if response := callMedia(t, server, http.MethodPut, "/api/v1/items/movie/progress", `{"seconds":-1}`); response.Code != http.StatusBadRequest {
		t.Fatalf("negative progress = %d", response.Code)
	}
	if response := callMedia(t, server, http.MethodPut, "/api/v1/items/movie/progress", `{"seconds":1,"playbackToken":"bad"}`); response.Code != http.StatusBadRequest {
		t.Fatalf("bad token = %d", response.Code)
	}
	handlers.Timeline = nil
	server = registeredMediaHandler(handlers)
	if response := callMedia(t, server, http.MethodPut, "/api/v1/items/movie/progress", `{"seconds":1,"playbackToken":"token"}`); response.Code != http.StatusBadRequest {
		t.Fatalf("missing timeline parser = %d", response.Code)
	}
	handlers, _, progress, lists := mediaHandlersForTest(items)
	progress.err = errors.New("disk")
	server = registeredMediaHandler(handlers)
	if response := callMedia(t, server, http.MethodPut, "/api/v1/items/movie/progress", `{"seconds":1}`); response.Code != http.StatusTeapot {
		t.Fatalf("progress store error = %d", response.Code)
	}
	progress.err, progress.accepted = nil, false
	if response := callMedia(t, server, http.MethodPut, "/api/v1/items/movie/progress", `{"seconds":1}`); response.Code != http.StatusConflict {
		t.Fatalf("stale progress = %d", response.Code)
	}
	progress.accepted = true
	if response := callMedia(t, server, http.MethodPut, "/api/v1/items/movie/progress", `{"seconds":5,"playbackToken":"token"}`); response.Code != http.StatusOK || progress.lastSeconds != 25 || !strings.Contains(response.Body.String(), `"seconds":5`) {
		t.Fatalf("mapped progress = %d seconds=%v body=%q", response.Code, progress.lastSeconds, response.Body.String())
	}

	progress.dismissErr = errors.New("disk")
	if response := callMedia(t, server, http.MethodDelete, "/api/v1/items/movie/continue-watching", ""); response.Code != http.StatusInternalServerError {
		t.Fatalf("dismiss error = %d", response.Code)
	}
	progress.dismissErr = nil
	lists.err = errors.New("disk")
	failures := []struct {
		method, target, body string
		status               int
	}{
		{http.MethodPost, "/api/v1/playlists", `{"name":"Queue"}`, http.StatusBadRequest},
		{http.MethodPost, "/api/v1/smart-playlists", `{"name":"Recent","sort":"title"}`, http.StatusBadRequest},
		{http.MethodPut, "/api/v1/playlists/Queue/order", `{"ids":[]}`, http.StatusBadRequest},
		{http.MethodPut, "/api/v1/items/movie/list", `{"listed":true}`, http.StatusInternalServerError},
		{http.MethodPut, "/api/v1/playlists/Queue/items/movie", `{"included":true}`, http.StatusTeapot},
		{http.MethodDelete, "/api/v1/playlists/Queue", "", http.StatusInternalServerError},
	}
	for _, test := range failures {
		if response := callMedia(t, server, test.method, test.target, test.body); response.Code != test.status {
			t.Fatalf("%s = %d, want %d, body=%q", test.target, response.Code, test.status, response.Body.String())
		}
	}
}

func TestItemResponseBindsProgressToTheAuthenticatedViewer(t *testing.T) {
	handlers, _, _, _ := mediaHandlersForTest(mediaFixture())
	response := callMedia(t, registeredMediaHandler(handlers), http.MethodGet, "/api/v1/items/movie", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"profileId":"viewer"`) {
		t.Fatalf("item progress has no authoritative Viewer Profile: %d %s", response.Code, response.Body.String())
	}
}
