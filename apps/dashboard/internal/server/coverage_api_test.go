package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-dashboard/internal/dashboard"
)

type coverageRoute struct{ method, path string }

var coverageMutationRoutes = []coverageRoute{
	{http.MethodPut, "/api/v1/board"},
	{http.MethodPost, "/api/v1/apps"},
	{http.MethodPatch, "/api/v1/apps/id"},
	{http.MethodDelete, "/api/v1/apps/id"},
	{http.MethodPost, "/api/v1/apps/id/restore"},
	{http.MethodPut, "/api/v1/apps/order"},
	{http.MethodPost, "/api/v1/apps/id/check"},
	{http.MethodPost, "/api/v1/board/check"},
	{http.MethodPut, "/api/v1/import"},
	{http.MethodPost, "/api/v1/import/preview"},
	{http.MethodPost, "/api/v1/import/external"},
	{http.MethodPost, "/api/v1/supporter/activate"},
}

func TestCoverageAPIRouteGuards(t *testing.T) {
	mux := http.NewServeMux()
	registerAPI(mux, nil, nil, nil)
	routes := append([]coverageRoute(nil), coverageMutationRoutes...)
	for _, path := range []string{"/api/v1", "/api/v1/openapi.json", "/api/v1/me", "/api/v1/catalog", "/api/v1/board", "/api/v1/export", "/api/v1/supporter"} {
		routes = append(routes, coverageRoute{http.MethodGet, path})
	}
	for _, route := range routes {
		t.Run(route.method+route.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), route.method, route.path, nil))
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("unauthenticated route = %d %s", response.Code, response.Body)
			}
		})
	}
}

func TestCoverageAPIRejectsMalformedMutationBodies(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	for _, route := range coverageMutationRoutes {
		t.Run(route.method+route.path, func(t *testing.T) {
			response := app.request(t, route.method, route.path, "{", &session)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("malformed body = %d %s", response.Code, response.Body)
			}
			if got := app.board.Snapshot(); got.Version != 1 || len(got.Apps) != 0 {
				t.Fatalf("invalid body mutated board: %+v", got)
			}
		})
	}
}

func TestCoverageAPIApplicationLifecycle(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusOK) }))
	t.Cleanup(provider.Close)
	for _, path := range []string{"/api/v1", "/api/v1/openapi.json", "/api/v1/catalog", "/api/v1/board", "/api/v1/export", "/api/v1/supporter", "/static/passkeys.js"} {
		assertCoverageAPIStatus(t, app.request(t, http.MethodGet, path, "", &session), http.StatusOK)
	}
	created := app.request(t, http.MethodPost, "/api/v1/apps", fmt.Sprintf(`{"name":"Media","url":%q,"checkEnabled":true,"expectedVersion":1}`, provider.URL), &session)
	assertCoverageAPIStatus(t, created, http.StatusCreated)
	var payload struct{ App dashboard.App }
	decodeBody(t, created.Body, &payload)
	path := "/api/v1/apps/" + payload.App.ID
	assertCoverageAPIStatus(t, app.request(t, http.MethodPatch, path, `{"name":"Changed","expectedVersion":2}`, &session), http.StatusOK)
	assertCoverageAPIStatus(t, app.request(t, http.MethodPut, "/api/v1/apps/order", fmt.Sprintf(`{"ids":[%q],"expectedVersion":3}`, payload.App.ID), &session), http.StatusOK)
	assertCoverageAPIStatus(t, app.request(t, http.MethodPost, path+"/check", `{}`, &session), http.StatusOK)
	assertCoverageAPIStatus(t, app.request(t, http.MethodDelete, path, `{"expectedVersion":4}`, &session), http.StatusOK)
	assertCoverageAPIStatus(t, app.request(t, http.MethodPost, path+"/restore", `{"expectedVersion":5}`, &session), http.StatusOK)
	assertCoverageAPIStatus(t, app.request(t, http.MethodPut, "/api/v1/board", `{"title":"Household","expectedVersion":6}`, &session), http.StatusOK)
	if got := app.board.Snapshot(); got.Title != "Household" || len(got.Apps) != 1 || got.Apps[0].Name != "Changed" {
		t.Fatalf("lifecycle board = %+v", got)
	}
	assertCoverageAPIStatus(t, app.request(t, http.MethodPost, "/api/v1/board/check", `{}`, &session), http.StatusOK)
	assertCoverageAPIStatus(t, app.request(t, http.MethodPut, "/api/v1/import", `{"title":"Recovered","apps":[],"expectedVersion":7}`, &session), http.StatusOK)
}

func TestCoverageAPIDomainErrors(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	for _, test := range []struct {
		coverageRoute
		body   string
		status int
	}{
		{coverageRoute{http.MethodPost, "/api/v1/apps"}, `{"expectedVersion":1}`, http.StatusBadRequest},
		{coverageRoute{http.MethodPatch, "/api/v1/apps/id"}, `{"name":"New","expectedVersion":1}`, http.StatusBadRequest},
		{coverageRoute{http.MethodPost, "/api/v1/apps/id/restore"}, `{"expectedVersion":1}`, http.StatusNotFound},
		{coverageRoute{http.MethodPost, "/api/v1/apps/id/check"}, `{}`, http.StatusNotFound},
		{coverageRoute{http.MethodPost, "/api/v1/import/preview"}, `{"source":"unknown","content":"{}"}`, http.StatusBadRequest},
		{coverageRoute{http.MethodPut, "/api/v1/board"}, `{"title":"New","expectedVersion":2}`, http.StatusConflict},
	} {
		assertCoverageAPIStatus(t, app.request(t, test.method, test.path, test.body, &session), test.status)
	}
	response := httptest.NewRecorder()
	writeDomainError(response, fmt.Errorf("private storage detail: %w", dashboard.ErrState))
	assertCoverageAPIStatus(t, response, http.StatusInternalServerError)
	if strings.Contains(response.Body.String(), "private storage detail") {
		t.Fatal("domain error exposed private detail")
	}
	response = httptest.NewRecorder()
	writeDomainError(response, errors.New("invalid input"))
	assertCoverageAPIStatus(t, response, http.StatusBadRequest)
}

func assertCoverageAPIStatus(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("API = %d %s, want %d", response.Code, response.Body, status)
	}
}
