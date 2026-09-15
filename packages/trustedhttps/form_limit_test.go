package trustedhttps

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSettingsFormLimitAppliesOnlyToFormRoutes(t *testing.T) {
	for _, test := range []struct {
		method, path string
		size         int
		limited      bool
	}{
		{http.MethodPost, "/settings/trusted-https", settingsFormMaxBody, false},
		{http.MethodPost, "/settings/trusted-https", settingsFormMaxBody + 1, true},
		{http.MethodPost, "/onboarding/trusted-https", settingsFormMaxBody + 1, true},
		{http.MethodPost, "/api/v1/settings/trusted-https", settingsFormMaxBody + 1, false},
		{http.MethodPost, "/settings/other", settingsFormMaxBody + 1, false},
		{http.MethodGet, "/settings/trusted-https", settingsFormMaxBody + 1, false},
	} {
		request := httptest.NewRequestWithContext(t.Context(), test.method, test.path, strings.NewReader(strings.Repeat("x", test.size)))
		request.ContentLength = -1
		called := false
		handler := LimitSettingsForms(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			called = true
			body, err := io.ReadAll(r.Body)
			if (err != nil) != test.limited || !test.limited && len(body) != test.size {
				t.Fatalf("%s %s size=%d: read=%d error=%v", test.method, test.path, test.size, len(body), err)
			}
		}))
		handler.ServeHTTP(httptest.NewRecorder(), request)
		if !called {
			t.Fatal("middleware skipped handler")
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/trusted-https", nil)
	request.Body = nil
	LimitSettingsForms(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			t.Fatal("nil body changed")
		}
	})).ServeHTTP(httptest.NewRecorder(), request)
}
