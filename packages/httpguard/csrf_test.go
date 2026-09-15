package httpguard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCSRFSessionTokenContract(t *testing.T) {
	t.Parallel()
	const want = "uqxJcfcTGXzomlUWs30svhokxDSJchrPMolX2MwLc1I"
	if got := CSRFToken("session-A"); got != want || CSRFToken("session-B") == got {
		t.Fatalf("session-bound token=%q", got)
	}
	if CSRFForRequest(nil) != "" {
		t.Fatal("nil request has a session token")
	}
	for _, value := range []string{"", "session-A"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
		request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: value, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		expected := ""
		if value != "" {
			expected = want
		}
		if got := CSRFForRequest(request); got != expected {
			t.Fatalf("cookie token=%q, want %q", got, expected)
		}
	}
}

func TestCSRFValidationPreservesHeaderAndFormRules(t *testing.T) {
	t.Parallel()
	token := CSRFToken("session-A")
	for _, test := range []struct {
		name, session, body string
		headers             []string
		want, stripped      bool
	}{
		{"no session", "", "bad=%", nil, true, false},
		{"missing token", "session-A", "", nil, false, true},
		{"wrong header", "session-A", "", []string{"wrong"}, false, false},
		{"valid header", "session-A", "", []string{token}, true, false},
		{"duplicate headers", "session-A", "", []string{token, token}, false, false},
		{"malformed form", "session-A", "_csrf=%", nil, false, false},
		{"valid form", "session-A", "_csrf=" + token + "&name=Home", nil, true, true},
		{"wrong form", "session-A", "_csrf=wrong", nil, false, true},
		{"duplicate form", "session-A", "_csrf=" + token + "&_csrf=" + token, nil, false, true},
		{"header wins", "session-A", "_csrf=wrong", []string{token}, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := csrfValidationRequest(t, test.session, test.body, test.headers)
			if got := ValidCSRF(request); got != test.want {
				t.Fatalf("valid=%v, want %v", got, test.want)
			}
			if test.stripped {
				assertCSRFFieldsRemoved(t, request)
			}
			if test.name == "valid form" && request.PostForm.Get("name") != "Home" {
				t.Fatal("CSRF validation removed unrelated form input")
			}
		})
	}
}

func csrfValidationRequest(t *testing.T, session, body string, headers []string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if session != "" {
		request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: session, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}
	for _, header := range headers {
		request.Header.Add("X-Kinosail-CSRF", header)
	}
	return request
}

func assertCSRFFieldsRemoved(t *testing.T, request *http.Request) {
	t.Helper()
	if _, found := request.PostForm["_csrf"]; found {
		t.Fatal("POST token was not consumed")
	}
	if _, found := request.Form["_csrf"]; found {
		t.Fatal("combined form token was not consumed")
	}
}
