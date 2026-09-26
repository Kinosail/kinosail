package server_test

import (
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

type failedCastEntropy struct{}

func (failedCastEntropy) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }

func TestCastEntropyFailureDoesNotPanicOrCreateGrant(t *testing.T) {
	handler, id := castFixture(t)
	endpoint := "/api/v1/items/" + id + "/cast"
	previous := rand.Reader
	rand.Reader = failedCastEntropy{}
	t.Cleanup(func() { rand.Reader = previous })
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://192.168.1.10:8080"+endpoint,
		strings.NewReader(`{"protocol":"google-cast","position":0}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "cast-entropy-test")
	failed := httptest.NewRecorder()
	handler.ServeHTTP(failed, request)
	rand.Reader = previous
	if failed.Code != http.StatusServiceUnavailable || strings.Contains(failed.Body.String(), "entropy") {
		t.Fatalf("cast entropy failure status=%d body=%q", failed.Code, failed.Body.String())
	}
	for index := 0; index < 4; index++ {
		if response := castRequest(t, handler, http.MethodPost, endpoint, `{"protocol":"google-cast","position":0}`); response.Code != http.StatusCreated {
			t.Fatalf("grant %d after entropy failure status=%d", index, response.Code)
		}
	}
}

func TestMissingRequestIDSurvivesEntropyFailure(t *testing.T) {
	handler, _ := castFixture(t)
	previous := rand.Reader
	rand.Reader = failedCastEntropy{}
	t.Cleanup(func() { rand.Reader = previous })
	response := castRequest(t, handler, http.MethodGet, "/api/v1/library", "")
	if response.Code != http.StatusOK || response.Header().Get("X-Request-ID") == "" {
		t.Fatalf("library with unavailable request ID entropy status=%d id=%q", response.Code, response.Header().Get("X-Request-ID"))
	}
}

func TestBookmarkEntropyFailureDoesNotSave(t *testing.T) {
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Song.mp3"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, id := formatTestItem(t, server.Config{MediaDir: media, DataDir: t.TempDir()})
	endpoint := "/api/v1/items/" + id + "/bookmarks"
	previous := rand.Reader
	rand.Reader = failedCastEntropy{}
	t.Cleanup(func() { rand.Reader = previous })
	failed := rawAPIRequest(t, handler, "", http.MethodPost, endpoint, `{"title":"Scene","seconds":1}`)
	rand.Reader = previous
	if failed.Code != http.StatusInternalServerError || strings.Contains(failed.Body.String(), "entropy") {
		t.Fatalf("bookmark entropy failure status=%d body=%q", failed.Code, failed.Body.String())
	}
	if response := castRequest(t, handler, http.MethodGet, endpoint, ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"bookmarks":[]`) {
		t.Fatalf("failed bookmark changed state status=%d body=%q", response.Code, response.Body.String())
	}
}
