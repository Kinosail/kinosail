package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	ownerTestPassword = "long-password-123"
	ownerNewPassword  = "new-long-password-456"
)

func TestOwnerCanChangePasswordAndRevokeOtherSessions(t *testing.T) {
	app := newTestApplication(t, Config{})
	missingAuthentication := app.request(t, http.MethodPost, "/api/v1/owner/password", `{"currentPassword":"long-password-123","newPassword":"new-long-password-456","confirmPassword":"new-long-password-456"}`, nil)
	requireJSONError(t, missingAuthentication, http.StatusUnauthorized, "authentication required")
	primary := app.setup(t)
	other := loginTestSession(t, app, ownerTestPassword, "other browser")

	response := app.request(t, http.MethodPost, "/api/v1/owner/password", `{"currentPassword":"long-password-123","newPassword":"new-long-password-456","confirmPassword":"new-long-password-456"}`, &primary)
	if response.Code != http.StatusNoContent {
		t.Fatalf("password change status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := app.request(t, http.MethodGet, "/api/v1/me", "", &primary).Code; got != http.StatusOK {
		t.Fatalf("presented session status = %d, want 200", got)
	}
	requireJSONError(t, app.request(t, http.MethodGet, "/api/v1/me", "", &other), http.StatusUnauthorized, "authentication required")
	requireJSONError(t, loginResponse(t, app, ownerTestPassword, "old password browser"), http.StatusUnauthorized, "invalid credentials")
	if got := loginResponse(t, app, ownerNewPassword, "new password browser").Code; got != http.StatusCreated {
		t.Fatalf("new password login status = %d, want 201", got)
	}
}

func TestPasswordChangeRejectsInvalidInputWithoutRevokingSessions(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
	}{
		{name: "wrong current password", body: `{"currentPassword":"wrong-password-value","newPassword":"new-long-password-456","confirmPassword":"new-long-password-456"}`, status: http.StatusBadRequest},
		{name: "confirmation mismatch", body: `{"currentPassword":"long-password-123","newPassword":"new-long-password-456","confirmPassword":"different-password-789"}`, status: http.StatusBadRequest},
		{name: "oversized new password", body: `{"currentPassword":"long-password-123","newPassword":"` + strings.Repeat("a", 73) + `","confirmPassword":"` + strings.Repeat("a", 73) + `"}`, status: http.StatusBadRequest},
		{name: "null field", body: `{"currentPassword":"long-password-123","newPassword":null,"confirmPassword":"new-long-password-456"}`, status: http.StatusBadRequest},
		{name: "mis-cased field", body: `{"CurrentPassword":"long-password-123","newPassword":"new-long-password-456","confirmPassword":"new-long-password-456"}`, status: http.StatusBadRequest},
		{name: "malformed JSON", body: `{"currentPassword":`, status: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newTestApplication(t, Config{})
			primary := app.setup(t)
			other := loginTestSession(t, app, ownerTestPassword, "other browser")
			response := app.request(t, http.MethodPost, "/api/v1/owner/password", test.body, &primary)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
			if got := app.request(t, http.MethodGet, "/api/v1/me", "", &primary).Code; got != http.StatusOK {
				t.Fatalf("primary session status = %d, want 200", got)
			}
			if got := app.request(t, http.MethodGet, "/api/v1/me", "", &other).Code; got != http.StatusOK {
				t.Fatalf("other session status = %d, want 200", got)
			}
			if got := loginResponse(t, app, ownerTestPassword, "verification browser").Code; got != http.StatusCreated {
				t.Fatalf("current password login status = %d, want 201", got)
			}
		})
	}
}

func loginTestSession(t *testing.T, app testApplication, password, device string) testSession {
	t.Helper()
	response := loginResponse(t, app, password, device)
	if response.Code != http.StatusCreated {
		t.Fatalf("login status = %d, body = %s", response.Code, response.Body.String())
	}
	var output struct {
		CSRF string `json:"csrf"`
	}
	decodeBody(t, response.Body, &output)
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login cookies = %#v", cookies)
	}
	return testSession{cookie: cookies[0], csrf: output.CSRF}
}

func loginResponse(t *testing.T, app testApplication, password, device string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"name":"Owner","password":"` + password + `","device":"` + device + `"}`
	return app.request(t, http.MethodPost, "/api/v1/session", body, nil)
}
