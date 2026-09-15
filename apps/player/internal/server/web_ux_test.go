package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestWebLoginFailureRetainsSafeNameWithoutSecrets(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login?next=%2Fsettings", nil)
	request.PostForm = url.Values{"name": {"<Viewer>"}, "password": {"private-password"}, "code": {"654321"}}
	response := httptest.NewRecorder()
	writeWebError(response, request, "invalid credentials", http.StatusUnauthorized)
	body := response.Body.String()
	for _, expected := range []string{`role="alert"`, `value="&lt;Viewer&gt;"`, `type="password"`, `data-login-next="/settings"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("login recovery missing %q", expected)
		}
	}
	for _, forbidden := range []string{"private-password", "654321", `value="<Viewer>"`} {
		if strings.Contains(body, forbidden) {
			t.Errorf("login recovery leaked %q", forbidden)
		}
	}
	if response.Code != http.StatusUnauthorized || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("error response lost status or privacy headers")
	}
}

func TestWebFailureRejectsUnsafeRecoveryDestinations(t *testing.T) {
	for _, referrer := range []string{"https://other.example/settings", "https://user:secret@example.com/settings", "//other.example/settings", strings.Repeat("x", 2049)} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://example.com/settings/playback", nil)
		request.Header.Set("Referer", referrer)
		response := httptest.NewRecorder()
		writeWebError(response, request, "Could not save settings", http.StatusBadRequest)
		if !strings.Contains(response.Body.String(), `href="/">Open previous page`) || strings.Contains(response.Body.String(), "user:secret") {
			t.Fatalf("unsafe recovery for %q", referrer)
		}
	}
}

func TestOfflineSyncProfileHeaderRejectsBeforeReadingOrWritingMedia(t *testing.T) {
	for _, values := range [][]string{{""}, {"viewer-b"}, {"viewer-a", "viewer-a"}, {strings.Repeat("x", 129)}} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/items/movie/progress/sync", strings.NewReader(`{}`))
		request = request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, viewerProfile{ID: "viewer-a"}))
		request.Header["X-Kinosail-Viewer-Profile"] = values
		response := httptest.NewRecorder()
		// Nil dependencies prove rejected identity cannot reach catalog reads or writes.
		syncMediaProgress(nil, nil)(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("header %q returned %d", values, response.Code)
		}
	}
}
