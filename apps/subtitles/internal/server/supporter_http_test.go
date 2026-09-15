package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupporterRejectsStrictAPIAndWebInputBeforeProviderContact(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	for _, raw := range []string{`{}`, `{"key":null}`, `{"key":"short"}`, `{"key":"VALID_SUPPORTER_KEY","unknown":true}`, `{"key":"VALID_SUPPORTER_KEY","recognitionName":null}`, `{"key":"VALID_SUPPORTER_KEY","recognitionName":""}`, `{"key":"VALID_SUPPORTER_KEY","recognitionName":"bad\u200dname"}`, `{"key":"VALID_SUPPORTER_KEY","key":"OTHER_SUPPORTER_KEY"}`, `{"key":"VALID_SUPPORTER_KEY","recognitionName":"Name","recognitionName":"Other"}`, `{"key":"` + strings.Repeat("A", 1100) + `"}`} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/supporter/activate", strings.NewReader(raw))
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid API input %q = %d %q", raw, response.Code, response.Body.String())
		}
	}
	for _, invalid := range []struct{ path, contentType, body string }{
		{path: "/api/v1/supporter/activate?extra=true", contentType: "application/json", body: `{"key":"VALID_SUPPORTER_KEY"}`},
		{path: "/api/v1/supporter/activate", contentType: "text/plain", body: `{"key":"VALID_SUPPORTER_KEY"}`},
		{path: "/api/v1/supporter/activate", body: `{"key":"VALID_SUPPORTER_KEY"}`},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, invalid.path, strings.NewReader(invalid.body))
		request.Header.Set("Authorization", "Bearer "+token)
		if invalid.contentType != "" {
			request.Header.Set("Content-Type", invalid.contentType)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid API boundary %#v = %d", invalid, response.Code)
		}
	}
	for _, body := range []string{"key=VALID_SUPPORTER_KEY&extra=true", "key=VALID_SUPPORTER_KEY&key=OTHER_SUPPORTER_KEY", "key=VALID_SUPPORTER_KEY&recognitionName=" + strings.Repeat("A", 81)} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/supporter/activate", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid web input %q = %d", body, response.Code)
		}
	}
	queryRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/supporter/activate?extra=true", strings.NewReader("key=VALID_SUPPORTER_KEY"))
	queryRequest.Header.Set("Authorization", "Bearer "+token)
	queryRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	queryResponse := httptest.NewRecorder()
	handler.ServeHTTP(queryResponse, queryRequest)
	if queryResponse.Code != http.StatusBadRequest {
		t.Fatalf("web query input = %d", queryResponse.Code)
	}
	if signer.calls.Load() != 0 {
		t.Fatalf("invalid input reached provider %d times", signer.calls.Load())
	}
}

func TestSupporterStatusAndCertificatesRemainOwnerOnly(t *testing.T) {
	signer := newSupporterSigner(t)
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, _ := supporterServer(t, t.TempDir(), signer, upstream)
	for _, path := range []string{"/api/v1/supporter", "/api/v1/supporter/certificates/patron-order"} {
		if response := apiCall(t, handler, "", http.MethodGet, path, nil); response.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous %s = %d %q", path, response.Code, response.Body.String())
		}
	}
}
