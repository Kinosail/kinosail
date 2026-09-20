package identitycore

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMFAEnrollmentRequiresPriorRecentAuthentication(t *testing.T) {
	for _, endpoint := range []string{"setup", "confirm", "web-confirm"} {
		t.Run(endpoint, func(t *testing.T) {
			state := &mfaHTTPState{}
			config := state.config()
			config.RecentlyAuthenticated = func(*http.Request, time.Duration) bool { return false }
			handlers, err := NewMFAHandlers(config)
			if err != nil {
				t.Fatal(err)
			}
			handler := map[string]http.HandlerFunc{"setup": handlers.Setup, "confirm": handlers.Confirm, "web-confirm": handlers.WebConfirm}[endpoint]
			response := httptest.NewRecorder()
			handler(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/account/mfa", strings.NewReader(`{"code":"123456"}`)))
			if response.Code != http.StatusForbidden || state.setup || state.confirm || state.strong {
				t.Fatalf("stale session enrolled or strengthened: %d %#v", response.Code, state)
			}
		})
	}
}
