package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// PrimaryPagesExposeTheInstallExperience preserves the shared app asset contract.
func (fixture AssetsFixture) PrimaryPagesExposeTheInstallExperience(t *testing.T) {
	t.Helper()
	t.Parallel()
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Movie.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(media, "", false)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	pages := map[string]*httptest.ResponseRecorder{"home": home}
	for name, path := range map[string]string{"settings": "/settings", "player": "/watch/" + id} {
		pages[name] = httptest.NewRecorder()
		handler.ServeHTTP(pages[name], httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	}
	auth := fixture.NewHandler("", t.TempDir(), true)
	pages["setup"] = httptest.NewRecorder()
	auth.ServeHTTP(pages["setup"], httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/setup", nil))
	_ = fixture.SignIn(t, auth, "/setup", "name=Owner&password=owner-password")
	pages["login"] = httptest.NewRecorder()
	auth.ServeHTTP(pages["login"], httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil))
	assertPrimaryPageInstallAssets(t, pages)
	if !strings.Contains(home.Body.String(), `data-install`) || !strings.Contains(home.Body.String(), `data-install-help`) || !strings.Contains(home.Body.String(), "Add to Home Screen") {
		t.Fatalf("home install controls = %q", home.Body.String())
	}
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/main.kinosail.bundle.js", nil))
	for _, expected := range []string{"serviceWorker.register", "beforeinstallprompt", "navigator.standalone", "IntersectionObserver", "data-library-next", "aria-busy"} {
		if !strings.Contains(script.Body.String(), expected) {
			t.Fatalf("PWA script missing %q: %q", expected, script.Body.String())
		}
	}
}

func assertPrimaryPageInstallAssets(t *testing.T, pages map[string]*httptest.ResponseRecorder) {
	t.Helper()
	for name, page := range pages {
		for _, expected := range []string{`rel="manifest"`, `rel="apple-touch-icon"`, "viewport-fit=cover", `/static/main.kinosail.bundle.js`} {
			if !strings.Contains(page.Body.String(), expected) {
				t.Fatalf("%s missing %q: %q", name, expected, page.Body.String())
			}
		}
	}
}
