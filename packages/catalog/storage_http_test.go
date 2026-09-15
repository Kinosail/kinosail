package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestListStorageAndRefreshBoundaries(t *testing.T) { //nolint:cyclop,gocognit,funlen // One test covers transactional restore and refresh coordination.
	values := map[string]bool{}
	playlists := map[string]map[string]bool{}
	order := map[string][]string{}
	smart := map[string]PlaylistRule{}
	loadErr := error(nil)
	storage := ListStorage{Values: &values, Playlists: &playlists, PlaylistOrder: &order, Smart: &smart, Paths: ListPaths{"values", "playlists", "order", "smart"}, LoadError: &loadErr}
	loaded := map[string]any{
		"values":    map[string]bool{"v:i": true},
		"playlists": map[string]map[string]bool{"v:Queue": {"i": true}},
		"order":     map[string][]string{"v:Queue": {"i"}},
		"smart":     map[string]PlaylistRule{},
	}
	storage.Load = func(file string, target any) (bool, error) {
		switch typed := target.(type) {
		case *map[string]bool:
			*typed = loaded[file].(map[string]bool)
		case *map[string]map[string]bool:
			*typed = loaded[file].(map[string]map[string]bool)
		case *map[string][]string:
			*typed = loaded[file].(map[string][]string)
		case *map[string]PlaylistRule:
			*typed = loaded[file].(map[string]PlaylistRule)
		}
		return true, nil
	}
	if err := storage.Restore(); err != nil || !values["v:i"] || !storage.State().Playlists["v:Queue"]["i"] {
		t.Fatalf("restored=%#v error=%v", storage.State(), err)
	}
	writes := 0
	storage.Persist = func(string, any) error { writes++; return nil }
	next := storage.State()
	next.Values["v:two"] = true
	if err := storage.Commit(t.Context(), next, ListedDocument); err != nil || writes != 1 || !values["v:two"] {
		t.Fatalf("commit writes=%d values=%#v error=%v", writes, values, err)
	}
	loadErr = errors.New("restore")
	if err := storage.Commit(t.Context(), next, ListedDocument); err == nil || writes != 1 {
		t.Fatal("restore failure allowed persistence")
	}

	directory := t.TempDir()
	inside := filepath.Join(directory, "movie.mkv")
	outside := filepath.Join(t.TempDir(), "other.mkv")
	if err := os.WriteFile(inside, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !SafePath(inside, []string{directory}) || SafePath(outside, []string{directory}) || SafePath(inside, nil) {
		t.Fatal("safe path boundary was not enforced")
	}
	var refreshMutex sync.Mutex
	acquires, refreshes := 0, 0
	if err := RefreshWork(t.Context(), &refreshMutex, func(context.Context) (func(), error) { acquires++; return func() { acquires++ }, nil }, func(context.Context) error { refreshes++; return nil }); err != nil || acquires != 2 || refreshes != 1 {
		t.Fatalf("refresh work = %d/%d %v", acquires, refreshes, err)
	}
	if err := RefreshWork(t.Context(), &refreshMutex, func(context.Context) (func(), error) { return nil, errors.New("busy") }, func(context.Context) error { refreshes++; return nil }); err == nil || refreshes != 1 {
		t.Fatal("failed lease ran refresh")
	}

	reschedule := make(chan struct{}, 1)
	requests := 0
	reschedule <- struct{}{}
	if !WaitRefresh(t.Context(), time.Hour, reschedule, func() { requests++ }) || requests != 0 {
		t.Fatal("reschedule did not request refresh")
	}
	if !WaitRefresh(t.Context(), time.Nanosecond, make(chan struct{}), func() { requests++ }) || requests != 1 {
		t.Fatal("refresh interval did not request refresh")
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if WaitRefresh(cancelled, time.Hour, make(chan struct{}), func() { requests++ }) {
		t.Fatal("cancelled refresh kept running")
	}
}

func TestRescanHandlerPreservesSuccessAndFailureResponses(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "Movie.mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := NewIndex(t.Context(), []ScanRoot{{Path: directory, Namespace: "Movies"}}, "", 0, nil)
	written := httptest.NewRecorder()
	RescanHandler(index, func(http.ResponseWriter, *http.Request, string, int) { t.Fatal("successful scan reported a failure") })(written, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scan", nil))
	if written.Code != http.StatusSeeOther || written.Header().Get("Location") != "/" {
		t.Fatalf("successful rescan = %d location=%q", written.Code, written.Header().Get("Location"))
	}

	missing := NewIndex(t.Context(), []ScanRoot{{Path: filepath.Join(t.TempDir(), "missing"), Namespace: "Movies"}}, "", 0, nil)
	failures := 0
	written = httptest.NewRecorder()
	RescanHandler(missing, func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
		failures++
		http.Error(writer, message, status)
	})(written, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scan", nil))
	if written.Code != http.StatusInternalServerError || failures != 1 || written.Header().Get("Location") != "" {
		t.Fatalf("failed rescan = %d failures=%d location=%q", written.Code, failures, written.Header().Get("Location"))
	}
}
