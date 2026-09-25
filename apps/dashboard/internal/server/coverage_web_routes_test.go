package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail-dashboard/internal/auth"
)

func TestCoverageWebRouteStates(t *testing.T) {
	app := newTestApplication(t, Config{})
	mux := http.NewServeMux()
	registerWeb(mux, app.auth)
	for _, path := range []string{"/", "/login", "/supporter"} {
		assertCoverageWebRedirect(t, mux, path, "/setup", nil)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/setup", nil))
	assertCoverageAPIStatus(t, response, http.StatusOK)
	app.setup(t)
	assertCoverageWebRedirect(t, mux, "/setup", "/", nil)
	for _, path := range []string{"/", "/supporter"} {
		assertCoverageWebRedirect(t, mux, path, "/login", nil)
	}
	identity := auth.Identity{ID: "Owner", Name: "Owner"}
	assertCoverageWebRedirect(t, mux, "/login", "/", &identity)
	for _, path := range []string{"/", "/supporter", "/login"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		if path != "/login" {
			request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity))
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		assertCoverageAPIStatus(t, response, http.StatusOK)
	}
	for _, path := range []string{"/missing", "/api/missing", "/mcp"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		assertCoverageAPIStatus(t, response, http.StatusNotFound)
	}
}

func assertCoverageWebRedirect(t *testing.T, handler http.Handler, path, location string, identity *auth.Identity) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	if identity != nil {
		request = request.WithContext(context.WithValue(request.Context(), identityKey{}, *identity))
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != location {
		t.Fatalf("redirect = %d %q", response.Code, response.Header().Get("Location"))
	}
}

func TestCoverageMissingAndInvalidStaticAssets(t *testing.T) {
	for _, name := range []string{".hidden", "../file", "missing"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/file", nil)
		request.SetPathValue("name", name)
		response := httptest.NewRecorder()
		serveAsset("web/static/")(response, request)
		assertCoverageAPIStatus(t, response, http.StatusNotFound)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/missing", nil)
	for _, handler := range []http.HandlerFunc{serveFile("missing", "text/plain"), func(writer http.ResponseWriter, request *http.Request) { serveHTML(writer, request, "missing") }} {
		response := httptest.NewRecorder()
		handler(response, request)
		assertCoverageAPIStatus(t, response, http.StatusNotFound)
	}
}
