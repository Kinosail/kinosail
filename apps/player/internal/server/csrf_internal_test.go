package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestCSRFContract(t *testing.T) {
	servertest.CSRF(t, security, csrfToken, newLocalizedTemplate)
}

func TestLibrarySearchDoesNotSubmitCSRFFieldInQuery(t *testing.T) {
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "MFA.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := New(Config{MediaDir: media})
	cookie := &http.Cookie{Name: "__Host-kinosail_session", Value: "session-secret", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}
	pageRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	pageRequest.AddCookie(cookie)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, pageRequest)
	body := page.Body.String()
	searchStart, searchEnd := strings.Index(body, `<form class="search"`), strings.Index(body, `</form>`)
	if searchStart < 0 || searchEnd < searchStart || strings.Contains(body[searchStart:searchEnd], `name="_csrf"`) {
		t.Fatalf("search form carries CSRF query field: %q", body)
	}
	searchRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?q=mfa&sort=title", nil)
	searchRequest.AddCookie(cookie)
	search := httptest.NewRecorder()
	handler.ServeHTTP(search, searchRequest)
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), "Search results") || strings.Contains(search.Body.String(), "unknown library query") {
		t.Fatalf("library search = %d %q", search.Code, search.Body.String())
	}
}
