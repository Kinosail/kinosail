package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestBrowserPageRedirectsAliasToConfiguredOrigin(t *testing.T) {
	t.Parallel()
	const origin = "https://subtitles.example:38128"
	dataDir := t.TempDir()
	configured, err := configuration.Load(dataDir, "", func(name string) (string, bool) {
		return `["192.0.2.56"]`, name == "KINOSAIL_TLS_HOSTS"
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(Config{DataDir: dataDir, RequireAuth: true, AuthURL: origin, Configuration: configured})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://192.0.2.56:38128/setup", nil)
	request.Header.Set("Sec-Fetch-Dest", "document")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != origin+"/setup" {
		t.Fatalf("alias browser page = %d, location = %q", response.Code, response.Header().Get("Location"))
	}
}

func TestBrowserPageRedirectPreservesNavigationAndCompatibility(t *testing.T) {
	t.Parallel()
	const origin = "https://subtitles.example:38128"
	handler := canonicalAliasHandler(t, origin)

	for _, test := range []struct {
		name       string
		method     string
		url        string
		headers    map[string]string
		wantStatus int
		wantTarget string
	}{
		{name: "query", method: http.MethodGet, url: "https://192.0.2.56:38128/login?next=%2Fsettings", headers: map[string]string{"Accept": "text/html"}, wantStatus: http.StatusTemporaryRedirect, wantTarget: origin + "/login?next=%2Fsettings"},
		{name: "HEAD", method: http.MethodHead, url: "https://192.0.2.56:38128/login", headers: map[string]string{"Sec-Fetch-Dest": "document"}, wantStatus: http.StatusTemporaryRedirect, wantTarget: origin + "/login"},
		{name: "insecure canonical host", method: http.MethodGet, url: "http://subtitles.example:38128/login", headers: map[string]string{"Sec-Fetch-Mode": "navigate"}, wantStatus: http.StatusTemporaryRedirect, wantTarget: origin + "/login"},
		{name: "API", method: http.MethodGet, url: "https://192.0.2.56:38128/api/v1/library", headers: map[string]string{"Accept": "text/html"}, wantStatus: http.StatusUnauthorized},
		{name: "Jellyfin", method: http.MethodGet, url: "https://192.0.2.56:38128/Users/Me", headers: map[string]string{"Accept": "text/html"}, wantStatus: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), test.method, test.url, nil)
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus || response.Header().Get("Location") != test.wantTarget {
				t.Fatalf("request = %d, location = %q", response.Code, response.Header().Get("Location"))
			}
		})
	}

	t.Run("oversized query", func(t *testing.T) {
		t.Parallel()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://192.0.2.56:38128/login?next="+strings.Repeat("x", 4097), nil)
		request.Header.Set("Accept", "text/html")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != origin+"/login" {
			t.Fatalf("oversized query = %d, location = %q", response.Code, response.Header().Get("Location"))
		}
	})

	t.Run("remote browser", func(t *testing.T) {
		t.Parallel()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://192.0.2.56:38128/setup", nil)
		request.Header.Set("Sec-Fetch-Dest", "document")
		response := httptest.NewRecorder()
		Remote(handler).ServeHTTP(response, request)
		if response.Code != http.StatusNotFound || response.Header().Get("Location") != "" {
			t.Fatalf("remote browser = %d, location = %q", response.Code, response.Header().Get("Location"))
		}
	})
}

func TestBrowserPageRedirectDisablesLoopbackOrigins(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ authURL, requestURL string }{
		"localhost": {"http://localhost:38128", "http://127.0.0.1:38128/setup"},
		"IPv4":      {"http://127.0.0.1:38128", "http://localhost:38128/setup"},
		"IPv6":      {"http://[::1]:38128", "http://localhost:38128/setup"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			handler := New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: test.authURL})
			servertest.AssertLoopbackNavigation(t, handler, test.requestURL)
		})
	}
}

func TestBrowserPageRedirectPrecedesStorageFailure(t *testing.T) {
	t.Parallel()
	profiles := newProfileStore("")
	profiles.err = errors.New("storage failed")
	auth := &authentication{profiles: profiles, passkeys: newPasskeyAuth("https://subtitles.example:38128", profiles)}
	handler := auth.protect(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler ran")
	}), func(*http.Request) string { return "GET /setup" })
	servertest.AssertOriginRedirectPrecedesStorageFailure(t, handler, "https://192.0.2.56:38128/setup", "https://subtitles.example:38128/setup", "storage redirect")
}

func TestBrowserPageRedirectSanitizesUnsafePaths(t *testing.T) {
	t.Parallel()
	auth := newPasskeyAuth("https://subtitles.example:38128", newProfileStore(""))
	servertest.AssertOriginRedirectSanitizesUnsafePaths(t, "https://192.0.2.56:38128", "https://subtitles.example:38128", "unsafe path", func(response http.ResponseWriter, request *http.Request) bool {
		return auth.engine.RequirePageOrigin(response, request, auth.redirectPages && auth.err == nil, secureRequest(request))
	})
}

func TestBrowserPagePostsRedirectBeforeSideEffects(t *testing.T) {
	t.Parallel()
	const origin = "https://subtitles.example:38128"
	handler := canonicalAliasHandler(t, origin)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://192.0.2.56:38128/setup", strings.NewReader("name=Owner&password=owner-password"))
	request.Header.Set("Accept", "text/html")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != origin+"/setup" || len(response.Result().Cookies()) != 0 {
		t.Fatalf("alias setup = %d, location = %q, cookie count = %d", response.Code, response.Header().Get("Location"), len(response.Result().Cookies()))
	}
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/setup", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("alias setup changed state: setup = %d", page.Code)
	}
}

func canonicalAliasHandler(t *testing.T, origin string) http.Handler {
	t.Helper()
	dataDir := t.TempDir()
	configured, err := configuration.Load(dataDir, "", func(name string) (string, bool) {
		return `["192.0.2.56"]`, name == "KINOSAIL_TLS_HOSTS"
	})
	if err != nil {
		t.Fatal(err)
	}
	return New(Config{DataDir: dataDir, RequireAuth: true, AuthURL: origin, Configuration: configured})
}
