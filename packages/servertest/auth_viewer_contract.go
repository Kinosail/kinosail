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

// AssertOwnerCanCreateViewerProfileWithoutGrantingAdministration preserves the original real-handler authentication regression.
func AssertOwnerCanCreateViewerProfileWithoutGrantingAdministration(t *testing.T, fixture LibraryAPIFixture) {
	t.Parallel()

	dataDir := t.TempDir()
	handler := fixture.NewHandler("", dataDir, true)
	ownerCookie := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/profiles", strings.NewReader("name=Sam&password=viewer-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(ownerCookie)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, request)
	login := fixture.SignIn(t, handler, "/login", "name=Sam&password=viewer-password")
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil)
	request.AddCookie(login)
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, request)
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(login)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, request)

	if created.Code != http.StatusSeeOther || settings.Code != http.StatusForbidden || strings.Contains(home.Body.String(), `href="/settings"`) || strings.Contains(home.Body.String(), ">Rescan<") || !strings.Contains(home.Body.String(), `class="header-utility-links"`) || strings.Contains(home.Body.String(), `class="header-utility-links owner-utilities"`) || !strings.Contains(home.Body.String(), `aria-label="Profile"`) || strings.Contains(home.Body.String(), `class="quiet danger">Sign out`) {
		t.Fatalf("create = %d, viewer settings = %d, home = %q", created.Code, settings.Code, home.Body.String())
	}
}

// AssertOwnerCanRemoveViewerAndRevokeSessions preserves the original real-handler authentication regression.
func AssertOwnerCanRemoveViewerAndRevokeSessions(t *testing.T, fixture LibraryAPIFixture) {
	t.Parallel()

	dataDir := t.TempDir()
	handler := fixture.NewHandler("", dataDir, true)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/profiles", strings.NewReader("name=Sam&password=viewer-password&rating=all&libraries=all"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	viewer := fixture.SignIn(t, handler, "/login", "name=Sam&password=viewer-password")
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/profiles/remove", strings.NewReader("id="+fixture.StoredProfileID(t, dataDir, "Sam")))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	removed := httptest.NewRecorder()
	handler.ServeHTTP(removed, request)
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(viewer)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, request)

	if removed.Code != http.StatusSeeOther || home.Code != http.StatusSeeOther || home.Header().Get("Location") != "/login" {
		t.Fatalf("remove = %d, former viewer home = %d %q", removed.Code, home.Code, home.Header().Get("Location"))
	}
}

// AssertOwnerCanResetViewerPassword preserves the original real-handler authentication regression.
func AssertOwnerCanResetViewerPassword(t *testing.T, fixture LibraryAPIFixture) {
	t.Parallel()

	dataDir := t.TempDir()
	handler := fixture.NewHandler("", dataDir, true)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/profiles", strings.NewReader("name=Sam&password=viewer-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/profiles/password", strings.NewReader("id="+fixture.StoredProfileID(t, dataDir, "Sam")+"&password=new-viewer-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	reset := httptest.NewRecorder()
	handler.ServeHTTP(reset, request)
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name=Sam&password=viewer-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	oldLogin := httptest.NewRecorder()
	handler.ServeHTTP(oldLogin, request)
	newLogin := fixture.SignIn(t, handler, "/login", "name=Sam&password=new-viewer-password")

	if reset.Code != http.StatusSeeOther || oldLogin.Code != http.StatusUnauthorized || newLogin == nil {
		t.Fatalf("reset = %d, old login = %d", reset.Code, oldLogin.Code)
	}
}

// AssertOwnerControlsViewerDownloads preserves the original real-handler authentication regression.
func AssertOwnerControlsViewerDownloads(t *testing.T, fixture LibraryAPIFixture) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(mediaDir, dataDir, true)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/profiles", strings.NewReader("name=Sam&password=viewer-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	viewer := fixture.SignIn(t, handler, "/login", "name=Sam&password=viewer-password")
	home := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(owner)
	handler.ServeHTTP(home, request)
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	download := func(cookie *http.Cookie) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/download/"+id, nil)
		request.AddCookie(cookie)
		handler.ServeHTTP(response, request)
		return response
	}
	denied := download(viewer)
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/profiles/permissions", strings.NewReader("id="+fixture.StoredProfileID(t, dataDir, "Sam")+"&downloads=true&rating=all&libraries=all"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	allowed := download(viewer)

	if denied.Code != http.StatusForbidden || allowed.Code != http.StatusOK || allowed.Body.String() != "video" || !strings.Contains(allowed.Header().Get("Content-Disposition"), "Heat.mp4") {
		t.Fatalf("denied = %d, allowed = %d %q %q", denied.Code, allowed.Code, allowed.Header().Get("Content-Disposition"), allowed.Body.String())
	}
}
