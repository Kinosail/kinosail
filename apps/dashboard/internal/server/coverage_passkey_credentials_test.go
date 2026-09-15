package server

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-dashboard/internal/auth"
	"github.com/MikeO7/kinosail/packages/servertest"
)

type coverageAuthenticator struct {
	device servertest.WebAuthnDevice
}

func newCoverageAuthenticator(t *testing.T) coverageAuthenticator {
	t.Helper()
	return coverageAuthenticator{device: servertest.NewWebAuthnDevice(t, "dashboard.example", "https://dashboard.example:38400", "dashboard-local-test-credential")}
}

func coveragePasskeyChallenge(t *testing.T, passkeys *passkeyAuth, registration bool, session auth.Session) (string, *http.Cookie) {
	t.Helper()
	request := coveragePasskeyRequest(t, "", session)
	handler := passkeys.beginLogin
	if registration {
		handler = passkeys.beginRegistration
	}
	return servertest.WebAuthnChallenge(t, handler, request)
}

func coveragePasskeyRequest(t *testing.T, body string, session auth.Session) *http.Request {
	t.Helper()
	request := coverageAuthRequest(t, body, session)
	request.URL.Scheme = "https"
	request.URL.Host = "dashboard.example:38400"
	request.Host = request.URL.Host
	return request
}

func (device coverageAuthenticator) registration(t *testing.T, challenge string) string {
	t.Helper()
	return device.device.Registration(t, challenge)
}

func (device coverageAuthenticator) assertion(t *testing.T, challenge, owner string) string {
	t.Helper()
	return device.device.Assertion(t, challenge, owner)
}
