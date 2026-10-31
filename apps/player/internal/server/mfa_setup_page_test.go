package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestMFASetupPageExplainsTheStepsAndAcceptsOnlyASixDigitCode(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	cookie := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	page := requestWithCookie(t, handler, http.MethodPost, "/account/mfa/setup", "", cookie)
	if page.Code != http.StatusOK {
		t.Fatalf("MFA setup = %d", page.Code)
	}
	body := page.Body.String()
	for _, fragment := range []string{
		"<ol class=\"wizard-help-list\">", "Save every recovery code before you continue.", "Enter the current 6-digit code to confirm the app.",
		"name=\"code\"", "inputmode=\"numeric\"", "autocomplete=\"one-time-code\"", "pattern=\"[0-9]{6}\"", "maxlength=\"6\"",
	} {
		if !strings.Contains(body, fragment) {
			t.Errorf("MFA setup page lacks %q", fragment)
		}
	}
}

func TestMFASetupStepsAreTranslated(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	cookie := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/account/mfa/setup", nil)
	request.AddCookie(cookie)
	request.Header.Set("Accept-Language", "de-DE,de;q=0.9")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.com")
	request.Host = "example.com"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if !strings.Contains(body, "Speichern Sie alle Wiederherstellungscodes, bevor Sie fortfahren.") || strings.Contains(body, "Save every recovery code before you continue.") {
		t.Fatalf("German MFA setup page = %d %q", response.Code, body)
	}
}
