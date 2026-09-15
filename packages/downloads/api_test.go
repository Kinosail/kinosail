package downloads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestDownloadAPIStartsListsGetsServesAndRemoves(t *testing.T) {
	t.Parallel()
	media := filepath.Join(t.TempDir(), "film.mp4")
	if err := os.WriteFile(media, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := New(Config{Context: t.Context(), Cache: t.TempDir(), Persist: persistJSON})
	mux := http.NewServeMux()
	RegisterAPI(mux, manager, testAccess{library.Item{ID: "item", Kind: "video", Title: "Film", Path: media, Added: time.Now()}})
	started := requestAPI(t, mux, http.MethodPost, "/api/v1/items/item/downloads", `{"quality":"original"}`, true)
	if started.Code != http.StatusAccepted {
		t.Fatalf("start = %d %q", started.Code, started.Body.String())
	}
	var job Job
	if err := json.Unmarshal(started.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if _, err := manager.Wait(ctx, "viewer", job.ID); err != nil {
		t.Fatal(err)
	}
	for method, path := range map[string]string{
		http.MethodGet + " list": "/api/v1/downloads",
		http.MethodGet + " job":  "/api/v1/downloads/" + job.ID,
		http.MethodGet + " file": "/api/v1/downloads/" + job.ID + "/file",
	} {
		response := requestAPI(t, mux, method[:3], path, "", true)
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Fatalf("%s = %d %q", method, response.Code, response.Body.String())
		}
	}
	removed := requestAPI(t, mux, http.MethodDelete, "/api/v1/downloads/"+job.ID, "", true)
	if removed.Code != http.StatusNoContent {
		t.Fatalf("remove = %d %q", removed.Code, removed.Body.String())
	}
}

func TestDownloadAPIRejectsUnauthorizedMissingAndInvalidRequests(t *testing.T) {
	t.Parallel()
	manager := New(Config{})
	mux := http.NewServeMux()
	RegisterAPI(mux, manager, testAccess{library.Item{}})
	for _, test := range []struct {
		method, path, body string
		allow              bool
		want               int
	}{
		{http.MethodGet, "/api/v1/downloads", "", false, http.StatusForbidden},
		{http.MethodGet, "/api/v1/downloads/missing", "", false, http.StatusNotFound},
		{http.MethodGet, "/api/v1/downloads/missing/file", "", false, http.StatusNotFound},
		{http.MethodDelete, "/api/v1/downloads/missing", "", false, http.StatusNotFound},
		{http.MethodPost, "/api/v1/items/missing/downloads", `{"quality":"original"}`, true, http.StatusNotFound},
		{http.MethodPost, "/api/v1/items/item/downloads", `{`, true, http.StatusBadRequest},
		{http.MethodPost, "/api/v1/items/item/downloads", `{"quality":"original"}`, true, http.StatusBadRequest},
		{http.MethodGet, "/api/v1/downloads/missing", "", true, http.StatusNotFound},
		{http.MethodDelete, "/api/v1/downloads/missing", "", true, http.StatusNotFound},
	} {
		response := requestAPI(t, mux, test.method, test.path, test.body, test.allow)
		if response.Code != test.want {
			t.Errorf("%s %s = %d %q; want %d", test.method, test.path, response.Code, response.Body.String(), test.want)
		}
	}
}

func TestDownloadAPIRedactsRemovalFailures(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	root := filepath.Join(directory, "not-a-directory")
	if err := os.WriteFile(root, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	job := Job{ID: "aaaaaaaaaaaaaaaa", Profile: "viewer", File: filepath.Join(directory, "film.mp4")}
	if err := os.WriteFile(job.File, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{root: root, jobs: map[string]Job{job.ID: job}}
	mux := http.NewServeMux()
	RegisterAPI(mux, manager, testAccess{library.Item{}})
	response := requestAPI(t, mux, http.MethodDelete, "/api/v1/downloads/"+job.ID, "", true)
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "download could not be removed") {
		t.Fatalf("failure = %d %q", response.Code, response.Body.String())
	}
}

type testAccess struct{ item library.Item }

func (access testAccess) Profile(request *http.Request) (string, bool) {
	return "viewer", request.Header.Get("X-Allow") == "true"
}

func (access testAccess) Item(request *http.Request, id string) (string, library.Item, bool) {
	return "viewer", access.item, request.Header.Get("X-Allow") == "true" && id == "item"
}

func requestAPI(t *testing.T, handler http.Handler, method, path, body string, allowed bool) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	if allowed {
		request.Header.Set("X-Allow", "true")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
