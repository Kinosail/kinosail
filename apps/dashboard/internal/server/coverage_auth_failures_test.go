package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-dashboard/internal/auth"
	"github.com/MikeO7/kinosail-dashboard/internal/database"
)

type coverageAuthStore struct {
	*database.Store
	fail bool
}

func (store *coverageAuthStore) SaveJSON(ctx context.Context, name string, value any) error {
	if store.fail {
		return errors.New("storage unavailable")
	}
	return store.Store.SaveJSON(ctx, name, value)
}

func (store *coverageAuthStore) SaveJSONBatch(ctx context.Context, values map[string]any) error {
	if store.fail {
		return errors.New("storage unavailable")
	}
	return store.Store.SaveJSONBatch(ctx, values)
}

func coverageAuthentication(t *testing.T, configured bool) (*auth.Manager, *coverageAuthStore, auth.Session) {
	t.Helper()
	store, err := database.Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	storage := &coverageAuthStore{Store: store}
	manager, err := auth.NewManager(t.Context(), storage)
	if err != nil {
		t.Fatal(err)
	}
	var session auth.Session
	if configured {
		session, err = manager.Setup(t.Context(), "Owner", "long-password-123", "Browser")
		if err != nil {
			t.Fatal(err)
		}
	}
	return manager, storage, session
}

func coverageAuthRequest(t *testing.T, body string, session auth.Session) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: session.Token}) // #nosec G124 -- Incoming test cookie; AddCookie sends only its name and value.
	request.Header.Set("X-Kinosail-CSRF", session.CSRF)
	return request.WithContext(context.WithValue(request.Context(), identityKey{}, session.Identity))
}

func TestCoverageAuthenticationPersistenceErrors(t *testing.T) {
	manager, store, session := coverageAuthentication(t, true)
	if _, err := manager.IssueMCPToken(t.Context()); err != nil {
		t.Fatal(err)
	}
	store.fail = true
	for name, handler := range map[string]http.HandlerFunc{
		"login":    loginHandler(Config{}, manager, newLoginLimiter()),
		"logout":   logoutHandler(Config{}, manager),
		"password": passwordHandler(manager),
		"issue":    issueMCPTokenHandler(manager),
		"revoke":   revokeMCPTokenHandler(manager),
	} {
		t.Run(name, func(t *testing.T) {
			body := `{}`
			if name == "login" {
				body = `{"name":"Owner","password":"long-password-123","device":"Browser"}`
			}
			if name == "password" {
				body = `{"currentPassword":"long-password-123","newPassword":"new-password-123","confirmPassword":"new-password-123"}`
			}
			response := httptest.NewRecorder()
			handler(response, coverageAuthRequest(t, body, session))
			assertCoverageAPIStatus(t, response, http.StatusInternalServerError)
		})
	}
}

func TestCoverageSetupErrorsAndRateLimit(t *testing.T) {
	manager, store, _ := coverageAuthentication(t, false)
	handler := setupHandler(Config{}, manager, newLoginLimiter())
	for attempt := 0; attempt <= perSourceLogins; attempt++ {
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{}`)))
		want := http.StatusBadRequest
		if attempt == perSourceLogins {
			want = http.StatusTooManyRequests
		}
		assertCoverageAPIStatus(t, response, want)
	}
	store.fail = true
	response := httptest.NewRecorder()
	setupHandler(Config{}, manager, newLoginLimiter())(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"name":"Owner","password":"long-password-123","device":"Browser"}`)))
	assertCoverageAPIStatus(t, response, http.StatusInternalServerError)
	configured, _, _ := coverageAuthentication(t, true)
	response = httptest.NewRecorder()
	setupHandler(Config{}, configured, newLoginLimiter())(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{}`)))
	assertCoverageAPIStatus(t, response, http.StatusConflict)
}

func TestCoverageAuthenticationMalformedAndTransportInputs(t *testing.T) {
	manager, _, session := coverageAuthentication(t, true)
	for _, handler := range []http.HandlerFunc{loginHandler(Config{}, manager, newLoginLimiter()), issueMCPTokenHandler(manager), revokeMCPTokenHandler(manager)} {
		response := httptest.NewRecorder()
		handler(response, coverageAuthRequest(t, "{", session))
		assertCoverageAPIStatus(t, response, http.StatusBadRequest)
	}
	response := httptest.NewRecorder()
	revokeMCPTokenHandler(manager)(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
	assertCoverageAPIStatus(t, response, http.StatusUnauthorized)
	request := coverageAuthRequest(t, `{}`, session)
	request.Header.Del("Cookie")
	response = httptest.NewRecorder()
	passwordHandler(manager)(response, request)
	assertCoverageAPIStatus(t, response, http.StatusUnauthorized)
	session.ViaBearer = true
	response = httptest.NewRecorder()
	passwordHandler(manager)(response, coverageAuthRequest(t, `{}`, session))
	assertCoverageAPIStatus(t, response, http.StatusForbidden)
}
