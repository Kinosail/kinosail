package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestChangingTMDBTokenRetriesTitlesThatMissedWithThePreviousToken(t *testing.T) {
	const (
		expired = "expired-tmdb-read-access-token-1234567890" //nolint:gosec // Deterministic fake token for a local HTTP fixture.
		renewed = "renewed-tmdb-read-access-token-1234567890" //nolint:gosec // Deterministic fake token for a local HTTP fixture.
	)
	var expiredSecondSearches atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Header.Get("Authorization") {
		case "Bearer " + renewed:
			request.Header.Set("Authorization", "Bearer token")
			fakeTMDB(writer, request)
		case "Bearer " + expired:
			if request.URL.Path == "/search/movie" && strings.Contains(request.URL.Query().Get("query"), "Second") {
				expiredSecondSearches.Add(1)
			}
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
		default:
			fakeTMDB(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	directory, mediaDir := t.TempDir(), t.TempDir()
	for key, value := range map[string]string{"integrations.tmdb.url": provider.URL, "integrations.tmdb.image_url": provider.URL, "integrations.tmdb.token": expired} {
		if err := configuration.Set(directory, key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, DataDir: directory, CacheDir: t.TempDir(), RequireAuth: true, Configuration: configured, Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: expired}, TMDBCheck: func(context.Context, string, string) error { return nil }})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	if err := os.WriteFile(filepath.Join(mediaDir, "Second.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if scan := apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/tasks/scan", nil); scan.Code != http.StatusNoContent {
		t.Fatalf("library scan = %d %q", scan.Code, scan.Body.String())
	}
	for deadline := time.Now().Add(10 * time.Second); expiredSecondSearches.Load() == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("automatic metadata refresh never tried the expired token")
		}
	}
	saved := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/configuration/integrations.tmdb", map[string]string{"token": renewed})
	if saved.Code != http.StatusAccepted {
		t.Fatalf("TMDB token save = %d %q", saved.Code, saved.Body.String())
	}
	library := requestWithCookie(t, handler, http.MethodGet, "/", "", owner)
	match := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(library.Body.String())
	if len(match) != 2 {
		t.Fatalf("movie missing from library: %s", library.Body.String())
	}
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(25 * time.Millisecond) {
		if player := requestWithCookie(t, handler, http.MethodGet, "/watch/"+match[1], "", owner); strings.Contains(player.Body.String(), "A linguist meets visitors.") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("titles that missed with the expired token were not retried after the token changed")
		}
	}
}
