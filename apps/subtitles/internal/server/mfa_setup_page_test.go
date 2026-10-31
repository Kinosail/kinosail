package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
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
