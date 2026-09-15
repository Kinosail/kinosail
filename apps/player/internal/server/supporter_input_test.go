package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func assertAmbiguousSupporterJSON(t *testing.T, handler http.Handler, token string) {
	t.Helper()
	for name, body := range map[string]string{
		"duplicate key":  `{"key":"VALID_SUPPORTER_KEY","key":"OTHER_SUPPORTER_KEY"}`,
		"duplicate name": `{"key":"VALID_SUPPORTER_KEY","recognitionName":"First","recognitionName":"Second"}`,
		"empty name":     `{"key":"VALID_SUPPORTER_KEY","recognitionName":""}`,
		"null name":      `{"key":"VALID_SUPPORTER_KEY","recognitionName":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/supporter/activate", strings.NewReader(body))
			request.Header.Set("Authorization", "Bearer "+token)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("ambiguous supporter JSON = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func assertInvalidSupporterAPIBoundaries(t *testing.T, handler http.Handler, token string) {
	t.Helper()
	for name, target := range map[string]string{"wrong media type": "/api/v1/supporter/activate", "query": "/api/v1/supporter/activate?extra=true"} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(`{"key":"VALID_SUPPORTER_KEY"}`))
			request.Header.Set("Authorization", "Bearer "+token)
			if name != "wrong media type" {
				request.Header.Set("Content-Type", "application/json")
			} else {
				request.Header.Set("Content-Type", "text/plain")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid supporter API boundary = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func assertInvalidSupporterForms(t *testing.T, handler http.Handler, token string) {
	t.Helper()
	for name, request := range map[string]*http.Request{
		"wrong content type": httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/supporter/activate", strings.NewReader("key=VALID_SUPPORTER_KEY")),
		"query":              httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/supporter/activate?extra=true", strings.NewReader("key=VALID_SUPPORTER_KEY")),
		"unknown field":      httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/supporter/activate", strings.NewReader("key=VALID_SUPPORTER_KEY&extra=true")),
		"duplicate":          httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/supporter/activate", strings.NewReader("key=VALID_SUPPORTER_KEY&key=OTHER_SUPPORTER_KEY")),
	} {
		t.Run(name, func(t *testing.T) {
			request.Header.Set("Authorization", "Bearer "+token)
			if name != "wrong content type" {
				request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid supporter form = %d %q", response.Code, response.Body.String())
			}
		})
	}
}
