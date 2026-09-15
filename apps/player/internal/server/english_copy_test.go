package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnglishBrowserErrorsUseHumanCopy(t *testing.T) {
	for _, language := range []string{"en", "en-US", "en-GB"} {
		t.Run(language, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login?lang="+language, nil)
			response := httptest.NewRecorder()
			localizedError(response, request, "invalid credentials", http.StatusUnauthorized)
			body := response.Body.String()
			if response.Code != http.StatusUnauthorized || !strings.Contains(body, "The name or password is incorrect. Try again.") || strings.Contains(body, "invalid credentials") {
				t.Fatalf("browser error = %d %q", response.Code, body)
			}
			if len(response.Result().Cookies()) != 0 {
				t.Fatal("displaying an error must not change the session")
			}
		})
	}
}

func TestEnglishCopyPreservesAPIErrorContract(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/session?lang=en", nil)
	response := httptest.NewRecorder()
	localizedError(response, request, "invalid credentials", http.StatusUnauthorized)
	body := response.Body.String()
	if response.Code != http.StatusUnauthorized || !strings.Contains(body, "invalid credentials") || strings.Contains(body, "The name or password") {
		t.Fatalf("API error contract changed: %d %q", response.Code, body)
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("displaying an error must not change the session")
	}
}
