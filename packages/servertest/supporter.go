package servertest

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// SupporterHandlerFactory binds product configuration to the shared supporter setup.
type SupporterHandlerFactory func(data, activation, support string, client *http.Client, now func() time.Time) http.Handler

// SupporterServer creates one authenticated Owner against a controlled supporter service.
func SupporterServer(t *testing.T, dataDir string, upstream *httptest.Server, now func() time.Time, newHandler SupporterHandlerFactory, totp func(*testing.T, string, time.Time) string) (http.Handler, string) {
	t.Helper()
	handler := newHandler(dataDir, upstream.URL, "https://support.example", upstream.Client(), now)
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "device": "Supporter test", "totp": true})
	var session struct {
		Token string `json:"token"`
		TOTP  struct {
			Secret string `json:"secret"`
		} `json:"totp"`
	}
	MustJSON(t, setup, &session)
	AssertAPIBody(t, APICall(t, handler, session.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": totp(t, session.TOTP.Secret, time.Now())}), http.StatusOK)
	return handler, session.Token
}
