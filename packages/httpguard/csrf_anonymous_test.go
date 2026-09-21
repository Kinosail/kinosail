package httpguard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnonymousTemplatesDoNotRenderSessionTokens(t *testing.T) {
	view := NewCSRFTemplate("anonymous", `<html><head></head><body><form method="post"><button>Continue</button></form></body></html>`, nil)
	response := httptest.NewRecorder()
	if err := ExecuteCSRFTemplate(view, response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/setup", nil), nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(response.Body.String(), "_csrf") || strings.Contains(response.Body.String(), "kinosail-csrf") || !strings.Contains(response.Body.String(), "Continue") {
		t.Fatal("anonymous page included a session token or lost its content")
	}
}
