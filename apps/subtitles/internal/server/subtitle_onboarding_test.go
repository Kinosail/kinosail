package server_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleOnboardingUsesSharedValidatedSettingsOperations(t *testing.T) { //nolint:cyclop // The flow proves web and API settings share validation.
	t.Parallel()
	media := t.TempDir()
	if err := os.Mkdir(filepath.Join(media, "Movies"), 0o700); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	for path, form := range map[string]url.Values{
		"/onboarding/subtitles/language":  {"language": {"es"}},
		"/onboarding/subtitles/libraries": {"path": {"Movies"}},
		"/onboarding/subtitles/scans":     {"frequency": {"15m"}},
	} {
		response := webFormCall(t, handler, "", path, form)
		if response.Code != http.StatusSeeOther || !strings.HasPrefix(response.Header().Get("Location"), "/onboarding/connection#") {
			t.Fatalf("%s = %d, location = %q", path, response.Code, response.Header().Get("Location"))
		}
	}
	invalid := webFormCall(t, handler, "", "/onboarding/subtitles/language", url.Values{"language": {"english"}})
	page := requestApp(t, handler, http.MethodGet, "/onboarding/connection", "")
	settings := requestApp(t, handler, http.MethodGet, "/api/v1/settings", "")
	if invalid.Code != http.StatusBadRequest || page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `value="es"`) || !strings.Contains(page.Body.String(), `value="15m" selected`) || !strings.Contains(page.Body.String(), ">Movies<") || !strings.Contains(settings.Body.String(), `"subtitleLanguages":["es"]`) {
		t.Fatalf("invalid = %d; page = %d %q", invalid.Code, page.Code, page.Body.String())
	}
}
