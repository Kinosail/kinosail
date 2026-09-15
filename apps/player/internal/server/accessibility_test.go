package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestCorePagesExposeKeyboardAndContrastSupport(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir()})
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	styles := httptest.NewRecorder()
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))
	for _, value := range []string{`href="#main"`, `id="main"`, `aria-label="Server name"`} {
		if !strings.Contains(settings.Body.String(), value) {
			t.Fatalf("settings missing %q", value)
		}
	}
	for _, value := range []string{"@media(max-width:1180px)", "@media(max-width:700px)", "prefers-reduced-motion", "forced-colors:active", ":focus-visible", ".auth>main form", ".mcp-approval-form"} {
		if !strings.Contains(styles.Body.String(), value) {
			t.Fatalf("styles missing %q", value)
		}
	}
}

func TestAgentConnectionsExposeAccessibleConsentAndSettingsLandmarks(t *testing.T) {
	t.Parallel()
	origin := "https://kino.test:38127"
	servertest.AgentConnectionsExposeAccessibleConsentAndSettingsLandmarks(t, server.New(server.Config{DataDir: t.TempDir(), AuthURL: origin}), origin)
}

func TestPlayerStatusUpdatesAreAnnounced(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Movie.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: t.TempDir(), MediaDir: media})
	library := httptest.NewRecorder()
	handler.ServeHTTP(library, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	id := regexp.MustCompile(`"id":"([a-f0-9]+)"`).FindStringSubmatch(library.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	for _, value := range []string{`role="status" aria-live="polite" data-cast-state`, `video aria-label="Movie"`, `data-player-fallback>Try again</button>`} {
		if !strings.Contains(player.Body.String(), value) {
			t.Fatalf("player missing %q", value)
		}
	}
}

func TestAccountJourneysExposeMainLandmarks(t *testing.T) {
	t.Parallel()
	servertest.AccountJourneysExposeMainLandmarks(t, server.New(server.Config{DataDir: t.TempDir()}))
}
