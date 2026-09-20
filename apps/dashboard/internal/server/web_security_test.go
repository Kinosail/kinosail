package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStaticAndHealthResponsesHaveSecurityHeaders(t *testing.T) {
	app := newTestApplication(t, Config{})
	tests := []struct {
		path         string
		cacheControl string
	}{
		{path: "/healthz", cacheControl: "no-store"},
		{path: "/static/app.js", cacheControl: "no-cache"},
		{path: "/static/dashboard.css", cacheControl: "no-cache"},
		{path: "/static/manrope.woff2", cacheControl: "no-cache"},
		{path: "/static/last-light.css", cacheControl: "no-cache"},
		{path: "/static/appearance.js", cacheControl: "no-cache"},
		{path: "/service-worker.js", cacheControl: "no-cache"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			response := app.request(t, http.MethodGet, test.path, "", nil)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			for name, want := range map[string]string{
				"Content-Security-Policy": "default-src 'self'",
				"Referrer-Policy":         "no-referrer",
				"X-Content-Type-Options":  "nosniff",
				"X-Frame-Options":         "DENY",
				"Permissions-Policy":      "camera=()",
				"Cache-Control":           test.cacheControl,
			} {
				if got := response.Header().Get(name); !strings.Contains(got, want) {
					t.Errorf("%s = %q, want content %q", name, got, want)
				}
			}
		})
	}
}

func TestStaticAssetsRevalidateWithoutResendingTheBody(t *testing.T) {
	app := newTestApplication(t, Config{})
	first := app.request(t, http.MethodGet, "/static/app.js", "", nil)
	etag := first.Header().Get("ETag")
	if etag == "" || first.Body.Len() == 0 {
		t.Fatalf("initial asset ETag = %q, bytes = %d", etag, first.Body.Len())
	}
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/static/app.js", nil)
	request.Header.Set("If-None-Match", etag)
	response := httptest.NewRecorder()
	app.handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotModified || response.Body.Len() != 0 {
		t.Fatalf("revalidation status = %d, bytes = %d", response.Code, response.Body.Len())
	}
}

func TestResponseJSONIsValid(t *testing.T) {
	app := newTestApplication(t, Config{})
	response := app.request(t, http.MethodGet, "/healthz", "", nil)
	var output map[string]string
	decodeBody(t, response.Body, &output)
	if got := output["status"]; got != "ok" {
		t.Fatalf("status = %q, want ok", got)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestUnknownBrowserRouteUsesBrandedHTML(t *testing.T) {
	app := newTestApplication(t, Config{})
	response := app.request(t, http.MethodGet, "/missing-page", "", nil)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(response.Body.String(), "That page is not here.") {
		t.Fatalf("body does not contain branded not-found message: %s", response.Body.String())
	}

	apiResponse := app.request(t, http.MethodGet, "/api/v1/missing", "", nil)
	if apiResponse.Code != http.StatusNotFound {
		t.Fatalf("API status = %d, want %d", apiResponse.Code, http.StatusNotFound)
	}
	if got := apiResponse.Header().Get("Content-Type"); got == "text/html; charset=utf-8" {
		t.Fatal("API not-found response unexpectedly used browser HTML")
	}
}
