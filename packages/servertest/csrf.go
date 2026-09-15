package servertest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/localization"
)

type csrfFixture struct {
	Security func(http.Handler) http.Handler
	Token    func(string) string
	Template func(string, string) *localization.TemplateSet
}

func CSRF(t *testing.T, security func(http.Handler) http.Handler, token func(string) string, template func(string, string) *localization.TemplateSet) {
	t.Helper()
	fixture := csrfFixture{Security: security, Token: token, Template: template}
	t.Run("BrowserSessionMutationRequiresItsOwnCSRFToken", fixture.testBrowserSessionMutationRequiresItsOwnCSRFToken)
	t.Run("BrowserSessionReadDoesNotRequireCSRFToken", fixture.testBrowserSessionReadDoesNotRequireCSRFToken)
	t.Run("ValidatedCSRFTransportFieldDoesNotReachApplicationForm", fixture.testValidatedCSRFTransportFieldDoesNotReachApplicationForm)
	t.Run("RenderedFormsCarrySessionBoundCSRFToken", fixture.testRenderedFormsCarrySessionBoundCSRFToken)
}

func (fixture csrfFixture) testBrowserSessionMutationRequiresItsOwnCSRFToken(t *testing.T) {
	called := 0
	handler := fixture.Security(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		called++
		writer.WriteHeader(http.StatusNoContent)
	}))
	for name, token := range map[string]string{"missing": "", "wrong": fixture.Token("another-session")} {
		t.Run(name, func(t *testing.T) {
			request := csrfRequest(t, token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden || called != 0 {
				t.Fatalf("mutation = %d called=%d", response.Code, called)
			}
		})
	}
	request := csrfRequest(t, fixture.Token("session-secret"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || called != 1 {
		t.Fatalf("valid mutation = %d called=%d", response.Code, called)
	}
}

func (fixture csrfFixture) testBrowserSessionReadDoesNotRequireCSRFToken(t *testing.T) {
	handler := fixture.Security(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://kinosail.test/settings", nil)
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: "session-secret", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("authenticated read = %d", response.Code)
	}
}

func (fixture csrfFixture) testValidatedCSRFTransportFieldDoesNotReachApplicationForm(t *testing.T) {
	handler := fixture.Security(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.PostForm.Has("_csrf") || request.Form.Has("_csrf") {
			t.Fatal("validated CSRF field reached application handler")
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, csrfRequest(t, fixture.Token("session-secret")))
	if response.Code != http.StatusNoContent {
		t.Fatalf("valid mutation = %d", response.Code)
	}
}

func (fixture csrfFixture) testRenderedFormsCarrySessionBoundCSRFToken(t *testing.T) {
	view := fixture.Template("csrf-test", `<html><head></head><body><form method="post"><button>Save</button></form></body></html>`)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://kinosail.test/settings", nil)
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: "session-secret", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	response := httptest.NewRecorder()
	if err := view.Execute(response, request, nil); err != nil {
		t.Fatal(err)
	}
	token := fixture.Token("session-secret")
	if body := response.Body.String(); !strings.Contains(body, `name="_csrf" value="`+token+`"`) || !strings.Contains(body, `name="kinosail-csrf" content="`+token+`"`) {
		t.Fatalf("CSRF token missing from rendered page: %q", body)
	}
}

func csrfRequest(t *testing.T, token string) *http.Request {
	t.Helper()
	form := url.Values{"value": {"change"}}
	if token != "" {
		form.Set("_csrf", token)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://kinosail.test/settings", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://kinosail.test")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: "session-secret", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	return request
}
