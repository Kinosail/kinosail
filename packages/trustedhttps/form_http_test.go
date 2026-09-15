package trustedhttps

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSettingsFormPreservesSaveAndErrorOrdering(t *testing.T) {
	t.Parallel()
	managed, conflict, storage := errors.New("managed"), errors.New("conflict"), errors.New("storage")
	for name, test := range map[string]struct {
		body          string
		failure       error
		status, saves int
	}{
		"saved":            {"domain=house&token=&address=192.168.1.2&termsAccepted=true", nil, http.StatusSeeOther, 1},
		"invalid form":     {"domain=house", nil, http.StatusBadRequest, 0},
		"managed":          {"domain=house&token=&address=x&termsAccepted=true", managed, http.StatusConflict, 1},
		"conflict":         {"domain=house&token=&address=x&termsAccepted=true", fmt.Errorf("wrapped: %w", conflict), http.StatusConflict, 1},
		"storage":          {"domain=house&token=&address=x&termsAccepted=true", storage, http.StatusInternalServerError, 1},
		"invalid settings": {"domain=house&token=&address=x&termsAccepted=true", errors.New("invalid address"), http.StatusBadRequest, 1},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			form := SettingsForm{
				Save: func(input SettingsInput) error {
					calls++
					if input.Domain != "house" || !input.TermsAccepted {
						t.Fatalf("unexpected save input: %#v", input)
					}
					return test.failure
				},
				Managed: managed, Conflict: conflict, Storage: storage, Redirect: "/settings#trusted-https",
				Error: func(w http.ResponseWriter, _ *http.Request, message string, status int) {
					http.Error(w, message, status)
				},
			}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			form.ServeHTTP(response, request)
			if response.Code != test.status || calls != test.saves {
				t.Fatalf("status=%d saves=%d; want %d/%d", response.Code, calls, test.status, test.saves)
			}
			if test.status == http.StatusSeeOther && response.Header().Get("Location") != form.Redirect {
				t.Fatalf("redirect=%q", response.Header().Get("Location"))
			}
		})
	}
}
