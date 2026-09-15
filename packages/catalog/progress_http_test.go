package catalog

import (
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type progressHTTPResult struct {
	message  string
	status   int
	notFound bool
}

type progressVisibleIndex struct{}

func (progressVisibleIndex) VisibleItem(_ *http.Request, id string) (library.Item, bool) {
	return library.Item{ID: id}, id != "missing"
}

func progressForm(target string, values url.Values) *http.Request {
	request := httptest.NewRequestWithContext(progressRequest("viewer", false).Context(), http.MethodPost, target, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-Viewer", "viewer")
	request.SetPathValue("id", "movie")
	return request
}

func testProgressHandlers(t *testing.T, dataDir string, result *progressHTTPResult) ProgressHTTPHandlers {
	t.Helper()
	store := NewRequestProgressStore(dataDir, nil, func(string, any) (bool, error) { return false, nil }, func(string, any) error { return nil }, func(request *http.Request) ProgressViewer {
		return ProgressViewer{ID: request.Header.Get("X-Viewer")}
	}, nil)
	return NewProgressHTTPHandlers(store, progressVisibleIndex{}, func(token string) (playback.Timeline, error) {
		if token == "bad" {
			return playback.Timeline{}, errors.New("token")
		}
		if token == "skip" {
			return playback.Timeline{SourceDuration: 20, Duration: 10, Omitted: []playback.Range{{Start: 0, End: 10}}}, nil
		}
		return playback.Timeline{}, nil
	}, func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
		result.message, result.status = message, status
		writer.WriteHeader(status)
	}, func(writer http.ResponseWriter, _ *http.Request) {
		result.notFound = true
		writer.WriteHeader(http.StatusNotFound)
	}, func(error) int { return http.StatusConflict })
}

func TestProgressHTTPSaveRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		name, id, seconds, token, watched, revision, message string
	}{
		{"missing item", "missing", "1", "", "", "", "invalid progress"},
		{"malformed seconds", "movie", "no", "", "", "", "invalid progress"},
		{"negative seconds", "movie", "-1", "", "", "", "invalid progress"},
		{"infinite seconds", "movie", strconv.FormatFloat(math.Inf(1), 'g', -1, 64), "", "", "", "invalid progress"},
		{"nan seconds", "movie", strconv.FormatFloat(math.NaN(), 'g', -1, 64), "", "", "", "invalid progress"},
		{"invalid token", "movie", "1", "bad", "", "", "invalid progress"},
		{"invalid watched", "movie", "1", "", "sometimes", "", "invalid watched state"},
		{"invalid revision", "movie", "1", "", "", "-1", "invalid progress revision"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := new(progressHTTPResult)
			handlers := testProgressHandlers(t, "", result)
			request := progressForm("/?playbackToken="+test.token, url.Values{"seconds": {test.seconds}, "watched": {test.watched}, "revision": {test.revision}})
			request.SetPathValue("id", test.id)
			response := httptest.NewRecorder()
			handlers.Save()(response, request)
			if response.Code != http.StatusBadRequest || result.message != test.message || handlers.Store.Len() != 0 {
				t.Fatalf("response=%d message=%q records=%d", response.Code, result.message, handlers.Store.Len())
			}
		})
	}
}

func TestProgressHTTPSaveMapsTimelineAndReportsStorageErrors(t *testing.T) {
	result := new(progressHTTPResult)
	handlers := testProgressHandlers(t, "", result)
	request := progressForm("/?playbackToken=skip", url.Values{"seconds": {"5"}, "watched": {"true"}, "revision": {"2"}, "session": {"phone"}})
	response := httptest.NewRecorder()
	handlers.Save()(response, request)
	state := handlers.Store.Get(request, "movie")
	if response.Code != http.StatusNoContent || state.Seconds != 15 || !state.Watched || state.Session != "phone" || state.Revision != 2 {
		t.Fatalf("response=%d state=%#v", response.Code, state)
	}

	result = new(progressHTTPResult)
	handlers = testProgressHandlers(t, t.TempDir(), result)
	handlers.Store.SetPersistence(func(string, any) error { return errors.New("blocked") })
	response = httptest.NewRecorder()
	handlers.Save()(response, progressForm("/", url.Values{"seconds": {"1"}}))
	if response.Code != http.StatusConflict || result.message != "blocked" || handlers.Store.Len() != 0 {
		t.Fatalf("response=%d message=%q records=%d", response.Code, result.message, handlers.Store.Len())
	}
}

func TestProgressHTTPSaveWatched(t *testing.T) {
	for _, test := range []struct {
		name, id, watched string
	}{
		{"missing", "missing", "true"},
		{"malformed", "movie", "sometimes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := new(progressHTTPResult)
			handlers := testProgressHandlers(t, "", result)
			request := progressForm("/", url.Values{"watched": {test.watched}})
			request.SetPathValue("id", test.id)
			response := httptest.NewRecorder()
			handlers.SaveWatched()(response, request)
			if response.Code != http.StatusBadRequest || result.message != "invalid watched state" {
				t.Fatalf("response=%d message=%q", response.Code, result.message)
			}
		})
	}
	result := new(progressHTTPResult)
	handlers := testProgressHandlers(t, "", result)
	request := progressForm("/", url.Values{"watched": {"true"}})
	response := httptest.NewRecorder()
	handlers.SaveWatched()(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/watch/movie" || !handlers.Store.Get(request, "movie").Watched {
		t.Fatalf("response=%d location=%q state=%#v", response.Code, response.Header().Get("Location"), handlers.Store.Get(request, "movie"))
	}
	result = new(progressHTTPResult)
	handlers = testProgressHandlers(t, t.TempDir(), result)
	handlers.Store.SetPersistence(func(string, any) error { return errors.New("blocked") })
	response = httptest.NewRecorder()
	handlers.SaveWatched()(response, progressForm("/", url.Values{"watched": {"true"}}))
	if response.Code != http.StatusInternalServerError || result.message != "blocked" {
		t.Fatalf("response=%d message=%q", response.Code, result.message)
	}
}

func TestProgressHTTPDismiss(t *testing.T) {
	result := new(progressHTTPResult)
	handlers := testProgressHandlers(t, "", result)
	missing := progressForm("/", nil)
	missing.SetPathValue("id", "missing")
	response := httptest.NewRecorder()
	handlers.Dismiss()(response, missing)
	if response.Code != http.StatusNotFound || !result.notFound {
		t.Fatalf("missing response=%d notFound=%t", response.Code, result.notFound)
	}
	result = new(progressHTTPResult)
	handlers = testProgressHandlers(t, t.TempDir(), result)
	handlers.Store.SetPersistence(func(string, any) error { return errors.New("blocked") })
	response = httptest.NewRecorder()
	handlers.Dismiss()(response, progressForm("/", nil))
	if response.Code != http.StatusInternalServerError || result.message != "blocked" {
		t.Fatalf("failed response=%d message=%q", response.Code, result.message)
	}
	result = new(progressHTTPResult)
	handlers = testProgressHandlers(t, "", result)
	request := progressForm("/", nil)
	response = httptest.NewRecorder()
	handlers.Dismiss()(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" || !handlers.Store.Get(request, "movie").Dismissed {
		t.Fatalf("response=%d location=%q state=%#v", response.Code, response.Header().Get("Location"), handlers.Store.Get(request, "movie"))
	}
}
