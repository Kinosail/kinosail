package identitycore

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type recordedResponse struct {
	audit, message string
	status         int
	json           any
	notFound       bool
}

func (record *recordedResponse) denialConfig(authenticationError bool) DenialConfig {
	return DenialConfig{
		Denied: func(_ *http.Request, reason string) { record.audit = reason },
		NotFound: func(writer http.ResponseWriter, _ *http.Request) {
			record.notFound = true
			writer.WriteHeader(http.StatusNotFound)
		},
		AuthenticationError: func(*http.Request) bool { return authenticationError },
		Error: func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
			record.message, record.status = message, status
			writer.WriteHeader(status)
		},
		JSON: func(writer http.ResponseWriter, value any, status int) {
			record.json, record.status = value, status
			writer.WriteHeader(status)
		},
	}
}

func TestRespondDenial(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/private", nil)
	tests := []struct {
		name                string
		denial              Denial
		authenticationError bool
		status              int
	}{
		{"not found", RemoteRouteDenied, false, http.StatusNotFound},
		{"MFA redirect", MFAEnrollmentRequired, false, http.StatusSeeOther},
		{"MFA JSON", MFAEnrollmentRequired, true, http.StatusForbidden},
		{"ordinary", APIKeyScopeDenied, false, http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			record := &recordedResponse{}
			response := httptest.NewRecorder()
			RespondDenial(response, request, test.denial, record.denialConfig(test.authenticationError))
			if response.Code != test.status || record.audit != test.denial.Reason() {
				t.Fatalf("denial response=%d audit=%q", response.Code, record.audit)
			}
			assertDenialPresentation(t, test.name, response, record)
		})
	}
	record := &recordedResponse{}
	response := httptest.NewRecorder()
	RespondDenial(response, request, Allowed, record.denialConfig(false))
	if response.Code != http.StatusInternalServerError || record.audit != "" {
		t.Fatalf("invalid denial response=%d audit=%q", response.Code, record.audit)
	}
}

func assertDenialPresentation(t *testing.T, name string, response *httptest.ResponseRecorder, record *recordedResponse) {
	t.Helper()
	switch name {
	case "not found":
		assertNotFoundDenial(t, record)
	case "MFA redirect":
		assertMFARedirectDenial(t, response, record)
	case "MFA JSON":
		assertMFAJSONDenial(t, record)
	case "ordinary":
		assertOrdinaryDenial(t, record)
	}
}

func assertNotFoundDenial(t *testing.T, record *recordedResponse) {
	t.Helper()
	if !record.notFound {
		t.Fatal("remote route denial did not use not-found presentation")
	}
}

func assertMFARedirectDenial(t *testing.T, response *httptest.ResponseRecorder, record *recordedResponse) {
	t.Helper()
	if response.Header().Get("Location") != "/account?mfa=required" || record.json != nil || record.message != "" {
		t.Fatalf("MFA redirect = location %q, json %#v, message %q", response.Header().Get("Location"), record.json, record.message)
	}
}

func assertMFAJSONDenial(t *testing.T, record *recordedResponse) {
	t.Helper()
	if record.json == nil || record.message != "" {
		t.Fatalf("MFA JSON = %#v, message %q", record.json, record.message)
	}
}

func assertOrdinaryDenial(t *testing.T, record *recordedResponse) {
	t.Helper()
	if record.message == "" || record.json != nil {
		t.Fatalf("ordinary denial = message %q, json %#v", record.message, record.json)
	}
}

func ownerConfig(record *recordedResponse, owner, local, managed, recentlyAuthenticated, authenticationError bool) OwnerConfig {
	return OwnerConfig{
		Identity:              func(*http.Request) (bool, bool) { return owner, local },
		Managed:               func(*http.Request) bool { return managed },
		RecentlyAuthenticated: func(*http.Request, time.Duration) bool { return recentlyAuthenticated },
		AuthenticationError:   func(*http.Request) bool { return authenticationError },
		StepUpPath:            func(*http.Request) string { return "/login?step-up=true" },
		Error: func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
			record.message, record.status = message, status
			writer.WriteHeader(status)
		},
		JSON: func(writer http.ResponseWriter, value any, status int) {
			record.json, record.status = value, status
			writer.WriteHeader(status)
		},
	}
}

func TestOwnerMiddleware(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                               string
		owner, local, managed, recent, api bool
		method                             string
		status                             int
	}{
		{"Owner", true, false, false, true, false, http.MethodPost, http.StatusNoContent},
		{"local Owner", true, true, false, false, false, http.MethodPost, http.StatusNoContent},
		{"managed Owner", true, false, true, false, false, http.MethodPost, http.StatusNoContent},
		{"safe Owner", true, false, false, false, false, http.MethodGet, http.StatusNoContent},
		{"step-up redirect", true, false, false, false, false, http.MethodPost, http.StatusSeeOther},
		{"step-up JSON", true, false, false, false, true, http.MethodPost, http.StatusForbidden},
		{"Viewer", false, true, true, true, false, http.MethodGet, http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			record := &recordedResponse{}
			called := false
			next := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				called = true
				writer.WriteHeader(http.StatusNoContent)
			})
			request := httptest.NewRequestWithContext(t.Context(), test.method, "/settings", nil)
			response := httptest.NewRecorder()
			Owner(next, ownerConfig(record, test.owner, test.local, test.managed, test.recent, test.api)).ServeHTTP(response, request)
			if response.Code != test.status || called != (test.status == http.StatusNoContent) {
				t.Fatalf("Owner response=%d called=%v", response.Code, called)
			}
		})
	}
	record := &recordedResponse{}
	response := httptest.NewRecorder()
	Owner(nil, ownerConfig(record, true, true, true, true, true)).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("invalid Owner config response = %d", response.Code)
	}
}

func passwordConfig(record *recordedResponse, valid bool, signInError error, signedIn, audited *bool) PasswordLoginConfig[string] {
	return PasswordLoginConfig[string]{
		Authenticate: func(name, password string) (string, bool) { return name + "-id", valid && password == "password" },
		ProfileID:    func(profile string) string { return profile },
		SignIn:       func(http.ResponseWriter, *http.Request, string) error { *signedIn = true; return signInError },
		SetAudit:     func(*http.Request, string) { *audited = true },
		ReturnPath:   func(string) string { return "/library" },
		Error: func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
			record.message, record.status = message, status
			writer.WriteHeader(status)
		},
	}
}

func TestLoginRequestLifecycle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		valid       bool
		signInError error
		status      int
		signedIn    bool
		audited     bool
	}{
		{"invalid credentials", false, nil, http.StatusUnauthorized, false, false},
		{"session failure", true, errors.New("failed"), http.StatusInternalServerError, true, false},
		{"success", true, nil, http.StatusSeeOther, true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			record := &recordedResponse{}
			signedIn, audited := false, false
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name=owner&password=password"))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			PasswordLoginRequest(response, request, "/requested", passwordConfig(record, test.valid, test.signInError, &signedIn, &audited))
			if response.Code != test.status || signedIn != test.signedIn || audited != test.audited {
				t.Fatalf("login response=%d signedIn=%v audited=%v", response.Code, signedIn, audited)
			}
		})
	}
}

func TestLoginRenderingAndInvalidConfiguration(t *testing.T) {
	t.Parallel()
	record := &recordedResponse{}
	signedIn, audited, rendered := false, false, false
	config := passwordConfig(record, true, nil, &signedIn, &audited)
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil)
	Login(response, request, "/requested", func(_ http.ResponseWriter, _ *http.Request, value any) error {
		rendered = value == "/requested"
		return nil
	}, config)
	if response.Header().Get("Content-Type") != "text/html; charset=utf-8" || !rendered || signedIn {
		t.Fatal("GET login did not render without session side effects")
	}
	response = httptest.NewRecorder()
	Login[string](response, request, "", nil, config)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("nil renderer response = %d", response.Code)
	}
	response = httptest.NewRecorder()
	PasswordLoginRequest(response, request, "", PasswordLoginConfig[string]{})
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("invalid login config response = %d", response.Code)
	}
	response = httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", nil)
	Login(response, request, "", func(http.ResponseWriter, *http.Request, any) error { return nil }, PasswordLoginConfig[string]{})
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("POST login response = %d", response.Code)
	}
}

func TestMFAHandlersStopAfterUnreadableInput(t *testing.T) {
	t.Parallel()
	config := MFAHTTPConfig{
		ReadJSON:          func(http.ResponseWriter, *http.Request, any) bool { return false },
		Setup:             func(*http.Request) (Enrollment, error) { t.Fatal("setup called"); return Enrollment{}, nil },
		Confirm:           func(*http.Request, string) error { t.Fatal("confirm called"); return nil },
		MarkStrong:        func(*http.Request) error { t.Fatal("mark strong called"); return nil },
		Verify:            func(*http.Request, string) bool { t.Fatal("verify called"); return false },
		Disable:           func(*http.Request) error { t.Fatal("disable called"); return nil },
		Error:             func(http.ResponseWriter, error, int) { t.Fatal("error called") },
		JSON:              func(http.ResponseWriter, any, int) { t.Fatal("JSON called") },
		WebError:          func(http.ResponseWriter, *http.Request, error, int) { t.Fatal("web error called") },
		WebConfirmSuccess: func(http.ResponseWriter, *http.Request) { t.Fatal("web confirm success called") },
		WebDisableSuccess: func(http.ResponseWriter, *http.Request) { t.Fatal("web disable success called") },
	}
	handlers, err := NewMFAHandlers(config)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mfa", nil)
	handlers.Confirm(httptest.NewRecorder(), request)
	handlers.Disable(httptest.NewRecorder(), request)
}
