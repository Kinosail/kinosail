package downloads

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type testRenderer struct {
	view View
	err  error
}

func (renderer *testRenderer) Execute(_ http.ResponseWriter, _ *http.Request, value any) error {
	renderer.view = value.(View)
	return renderer.err
}

func TestDownloadWebListsAndStartsVisibleItems(t *testing.T) {
	t.Parallel()
	media := filepath.Join(t.TempDir(), "film.mp4")
	if err := os.WriteFile(media, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	item := library.Item{ID: "item", Kind: "video", Title: "Film", Path: media, Added: time.Now()}
	gate := make(chan struct{}, 1)
	t.Cleanup(func() { gate <- struct{}{} })
	manager := New(Config{Context: t.Context(), Cache: t.TempDir(), Persist: persistJSON, Acquire: func(context.Context) (func(), error) {
		<-gate
		return func() {}, nil
	}})
	view := &testRenderer{}
	mux := http.NewServeMux()
	RegisterWeb(mux, manager, testAccess{item}, view, testWebError, http.NotFound)

	started := requestWeb(t, mux, http.MethodPost, "/offline/item", "quality=original", true)
	if started.Code != http.StatusSeeOther || started.Header().Get("Location") != "/offline-downloads" {
		t.Fatalf("start = %d %q", started.Code, started.Header().Get("Location"))
	}
	listed := requestAPI(t, mux, http.MethodGet, "/offline-downloads", "", true)
	if listed.Code != http.StatusOK || view.view.Profile != "viewer" || len(view.view.Jobs) != 1 || !view.view.Pending {
		t.Fatalf("view = %#v, status = %d", view.view, listed.Code)
	}
	gate <- struct{}{}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if _, err := manager.Wait(ctx, "viewer", view.view.Jobs[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadWebRejectsHiddenAndInvalidRequests(t *testing.T) {
	t.Parallel()
	manager := New(Config{})
	mux := http.NewServeMux()
	RegisterWeb(mux, manager, testAccess{library.Item{}}, &testRenderer{}, testWebError, http.NotFound)
	for _, test := range []struct {
		method, path, body string
		allowed            bool
		want               int
	}{
		{http.MethodGet, "/offline-downloads", "", false, http.StatusForbidden},
		{http.MethodPost, "/offline/missing", "quality=original", true, http.StatusNotFound},
		{http.MethodPost, "/offline/item", "quality=original", true, http.StatusBadRequest},
		{http.MethodPost, "/offline/item", "extra=value&quality=original", true, http.StatusBadRequest},
		{http.MethodPost, "/offline-downloads/missing/remove", "", false, http.StatusNotFound},
		{http.MethodPost, "/offline-downloads/missing/remove", "", true, http.StatusNotFound},
		{http.MethodPost, "/offline-downloads/missing/remove", "extra=value", true, http.StatusBadRequest},
	} {
		response := requestWeb(t, mux, test.method, test.path, test.body, test.allowed)
		if response.Code != test.want {
			t.Errorf("%s %s = %d; want %d", test.method, test.path, response.Code, test.want)
		}
	}
}

func TestDownloadWebRemovesOwnedJobAndRedactsFailures(t *testing.T) {
	t.Parallel()
	job := Job{ID: "aaaaaaaaaaaaaaaa", Profile: "viewer", File: filepath.Join(t.TempDir(), "film.mp4")}
	if err := os.WriteFile(job.File, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{root: t.TempDir(), jobs: map[string]Job{job.ID: job}}
	mux := http.NewServeMux()
	RegisterWeb(mux, manager, testAccess{}, &testRenderer{}, testWebError, http.NotFound)
	removed := requestWeb(t, mux, http.MethodPost, "/offline-downloads/"+job.ID+"/remove", "", true)
	if removed.Code != http.StatusSeeOther || len(manager.jobs) != 0 {
		t.Fatalf("remove = %d, jobs = %d", removed.Code, len(manager.jobs))
	}

	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager = &Manager{root: blocked, jobs: map[string]Job{job.ID: job}}
	mux = http.NewServeMux()
	RegisterWeb(mux, manager, testAccess{}, &testRenderer{}, testWebError, http.NotFound)
	failed := requestWeb(t, mux, http.MethodPost, "/offline-downloads/"+job.ID+"/remove", "", true)
	if failed.Code != http.StatusInternalServerError {
		t.Fatalf("failure = %d %q", failed.Code, failed.Body.String())
	}
}

func testWebError(writer http.ResponseWriter, _ *http.Request, message string, status int) {
	http.Error(writer, message, status)
}

func requestWeb(t *testing.T, handler http.Handler, method, path, body string, allowed bool) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if allowed {
		request.Header.Set("X-Allow", "true")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

var _ Renderer = (*testRenderer)(nil)
