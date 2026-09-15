package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestPasskeyPagesRedirectAliasesToConfiguredOrigin(t *testing.T) {
	t.Parallel()
	const origin = "https://media.example:38127"

	t.Run("setup", func(t *testing.T) {
		t.Parallel()
		handler := canonicalAliasHandler(t)
		assertCanonicalPageRedirect(t, handler, "/setup", nil, origin+"/setup")
	})

	t.Run("login", func(t *testing.T) {
		t.Parallel()
		handler := canonicalAliasHandler(t)
		_ = setupOwnerAtOrigin(t, handler)
		assertCanonicalPageRedirect(t, handler, "/login?next=%2F%3Fview%3Dmovies", nil, origin+"/login?next=%2F%3Fview%3Dmovies")
	})

	t.Run("account", func(t *testing.T) {
		t.Parallel()
		handler := canonicalAliasHandler(t)
		owner := setupOwnerAtOrigin(t, handler)
		assertCanonicalPageRedirect(t, handler, "/account?mfa=required", owner, origin+"/account?mfa=required")
	})

	t.Run("authenticated browser navigation", func(t *testing.T) {
		t.Parallel()
		handler := canonicalAliasHandler(t)
		owner := setupOwnerAtOrigin(t, handler)
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://192.0.2.55:38127/?view=movies", nil)
		request.Header.Set("Accept", "text/html,application/xhtml+xml")
		request.AddCookie(owner)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != origin+"/?view=movies" {
			t.Fatalf("alias browser navigation = %d, location = %q", response.Code, response.Header().Get("Location"))
		}
	})

	t.Run("oversized query", func(t *testing.T) {
		t.Parallel()
		handler := canonicalAliasHandler(t)
		_ = setupOwnerAtOrigin(t, handler)
		assertCanonicalPageRedirect(t, handler, "/login?next="+strings.Repeat("x", 4097), nil, origin+"/login")
	})

	t.Run("HEAD navigation", func(t *testing.T) {
		t.Parallel()
		assertCanonicalNavigation(t, canonicalAliasHandler(t), http.MethodHead, "https://192.0.2.55:38127/login", origin+"/login")
	})

	t.Run("insecure canonical host", func(t *testing.T) {
		t.Parallel()
		assertCanonicalNavigation(t, canonicalAliasHandler(t), http.MethodGet, "http://media.example:38127/login", origin+"/login")
	})
}

func assertCanonicalNavigation(t *testing.T, handler http.Handler, method, target, location string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, target, nil)
	request.Header.Set("Sec-Fetch-Dest", "document")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != location {
		t.Fatalf("canonical navigation = %d, location = %q", response.Code, response.Header().Get("Location"))
	}
}

func TestPasskeyPageRedirectPreservesCompatibilityBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("API", func(t *testing.T) {
		t.Parallel()
		assertNoCanonicalRedirect(t, canonicalAliasHandler(t), "https://192.0.2.55:38127/api/v1/library", http.StatusUnauthorized)
	})

	t.Run("Jellyfin", func(t *testing.T) {
		t.Parallel()
		assertNoCanonicalRedirect(t, canonicalAliasHandler(t), "https://192.0.2.55:38127/Users/Me", http.StatusNotFound)
	})

	t.Run("remote browser", func(t *testing.T) {
		t.Parallel()
		handler := canonicalAliasHandler(t)
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://192.0.2.55:38127/setup", nil)
		request.Header.Set("Sec-Fetch-Dest", "document")
		response := httptest.NewRecorder()
		Remote(handler).ServeHTTP(response, request)
		if response.Code != http.StatusNotFound || response.Header().Get("Location") != "" {
			t.Fatalf("remote browser = %d, location = %q", response.Code, response.Header().Get("Location"))
		}
	})

	for name, test := range map[string]struct{ authURL, requestURL string }{
		"localhost": {"http://localhost:38127", "http://127.0.0.1:38127/setup"},
		"IPv4":      {"http://127.0.0.1:38127", "http://localhost:38127/setup"},
		"IPv6":      {"http://[::1]:38127", "http://localhost:38127/setup"},
	} {
		t.Run("loopback "+name, func(t *testing.T) {
			t.Parallel()
			handler := New(Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: test.authURL})
			servertest.AssertLoopbackNavigation(t, handler, test.requestURL)
		})
	}
}

func assertNoCanonicalRedirect(t *testing.T, handler http.Handler, target string, status int) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	request.Header.Set("Accept", "text/html")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != status || response.Header().Get("Location") != "" {
		t.Fatalf("request = %d, location = %q", response.Code, response.Header().Get("Location"))
	}
}

func TestPasskeyPageRedirectPrecedesProfileStorageFailure(t *testing.T) {
	t.Parallel()
	profiles := newProfileStore("")
	profiles.err = errors.New("storage failed")
	auth := &authentication{profiles: profiles, passkeys: newPasskeyAuth("https://media.example:38127", profiles)}
	handler := auth.protect(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler ran")
	}), func(*http.Request) string { return "GET /setup" })
	servertest.AssertOriginRedirectPrecedesStorageFailure(t, handler, "https://192.0.2.55:38127/setup", "https://media.example:38127/setup", "storage failure redirect")
}

func TestPasskeyPageRedirectSanitizesUnsafePaths(t *testing.T) {
	t.Parallel()
	auth := newPasskeyAuth("https://media.example:38127", newProfileStore(""))
	servertest.AssertOriginRedirectSanitizesUnsafePaths(t, "https://192.0.2.55:38127", "https://media.example:38127", "unsafe path redirect", func(response http.ResponseWriter, request *http.Request) bool {
		return auth.engine.RequirePageOrigin(response, request, auth.redirectPages && auth.err == nil, secureRequest(request))
	})
}

func TestPasskeyPagePostsRedirectBeforeAccountSideEffects(t *testing.T) {
	t.Parallel()
	const origin = "https://media.example:38127"

	t.Run("setup", func(t *testing.T) {
		t.Parallel()
		handler := canonicalAliasHandler(t)
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://192.0.2.55:38127/setup", strings.NewReader("name=Owner&password=owner-password"))
		request.Header.Set("Accept", "text/html,application/xhtml+xml")
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assertAliasPostRedirect(t, response, "setup", origin+"/setup")
		page := httptest.NewRecorder()
		handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/setup", nil))
		if page.Code != http.StatusOK {
			t.Fatalf("alias setup changed profile state: canonical setup = %d", page.Code)
		}
	})

	t.Run("login", func(t *testing.T) {
		t.Parallel()
		handler := canonicalAliasHandler(t)
		_ = setupOwnerAtOrigin(t, handler)
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://192.0.2.55:38127/login?next=%2Fsettings", strings.NewReader("name=Owner&password=owner-password"))
		request.Header.Set("Accept", "text/html,application/xhtml+xml")
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assertAliasPostRedirect(t, response, "login", origin+"/login?next=%2Fsettings")
	})

	t.Run("logout", func(t *testing.T) {
		t.Parallel()
		handler := canonicalAliasHandler(t)
		owner := setupOwnerAtOrigin(t, handler)
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://192.0.2.55:38127/logout", nil)
		request.Header.Set("Sec-Fetch-Mode", "navigate")
		request.AddCookie(owner)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assertAliasPostRedirect(t, response, "logout", origin+"/")
		account := httptest.NewRecorder()
		accountRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/account", nil)
		accountRequest.AddCookie(owner)
		handler.ServeHTTP(account, accountRequest)
		if account.Code != http.StatusOK {
			t.Fatalf("alias logout changed session state: canonical account = %d", account.Code)
		}
	})
}

func canonicalAliasHandler(t *testing.T) http.Handler {
	t.Helper()
	const origin = "https://media.example:38127"
	dataDir := t.TempDir()
	configured, err := configuration.Load(dataDir, "", func(name string) (string, bool) {
		return `["192.0.2.55"]`, name == "KINOSAIL_TLS_HOSTS"
	})
	if err != nil {
		t.Fatal(err)
	}
	return New(Config{DataDir: dataDir, RequireAuth: true, AuthURL: origin, Configuration: configured})
}

func setupOwnerAtOrigin(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	const origin = "https://media.example:38127"
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+"/setup", strings.NewReader("name=Owner&password=owner-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "__Host-kinosail_session" {
			return cookie
		}
	}
	t.Fatalf("setup = %d %q", response.Code, response.Body.String())
	return nil
}

func assertCanonicalPageRedirect(t *testing.T, handler http.Handler, path string, cookie *http.Cookie, want string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://192.0.2.55:38127"+path, nil)
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != want {
		t.Fatalf("GET %s = %d, location = %q, want %q", path, response.Code, response.Header().Get("Location"), want)
	}
}

func assertAliasPostRedirect(t *testing.T, response *httptest.ResponseRecorder, action, destination string) {
	t.Helper()
	cookieCount := len(response.Result().Cookies())
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != destination || cookieCount != 0 {
		t.Fatalf("alias %s = %d, location = %q, cookie count = %d", action, response.Code, response.Header().Get("Location"), cookieCount)
	}
}
