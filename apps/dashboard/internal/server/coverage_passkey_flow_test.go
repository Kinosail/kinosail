package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoverageRealPasskeyRegistrationAndLogin(t *testing.T) {
	manager, store, session := coverageAuthentication(t, true)
	passkeys := newPasskeyAuth("https://dashboard.example:38400", manager, true)
	device := newCoverageAuthenticator(t)
	challenge, cookie := coveragePasskeyChallenge(t, passkeys, true, session)
	registration := device.registration(t, challenge)
	request := coveragePasskeyRequest(t, registration, session)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	passkeys.finishRegistration(response, request)
	assertCoverageAPIStatus(t, response, http.StatusNoContent)
	owner, err := manager.PasskeyOwner()
	if err != nil || len(owner.Credentials) != 1 {
		t.Fatalf("registered credentials = %+v, %v", owner, err)
	}
	for _, mode := range []string{"success", "storage", "invalid", "expired"} {
		t.Run(mode, func(t *testing.T) {
			challenge, cookie := coveragePasskeyChallenge(t, passkeys, false, session)
			if mode == "invalid" {
				challenge = "different-challenge"
			}
			request := coveragePasskeyRequest(t, device.assertion(t, challenge, owner.ID), session)
			if mode != "expired" {
				request.AddCookie(cookie)
			}
			store.fail = mode == "storage"
			response := httptest.NewRecorder()
			passkeys.finishLogin(response, request)
			store.fail = false
			assertCoverageAPIStatus(t, response, coveragePasskeyLoginStatus(mode))
			if mode == "success" {
				assertCoveragePasskeySession(t, response)
			}
		})
	}
}

func coveragePasskeyLoginStatus(mode string) int {
	switch mode {
	case "success":
		return http.StatusNoContent
	case "expired":
		return http.StatusBadRequest
	default:
		return http.StatusUnauthorized
	}
}

func assertCoveragePasskeySession(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if len(response.Result().Cookies()) != 1 || response.Header().Get("X-Kinosail-Login-Next") != "/" {
		t.Fatal("passkey login omitted session or destination")
	}
}

func TestCoveragePasskeyRegistrationVerificationFailures(t *testing.T) {
	manager, store, session := coverageAuthentication(t, true)
	passkeys := newPasskeyAuth("https://dashboard.example:38400", manager, true)
	device := newCoverageAuthenticator(t)
	for _, mode := range []string{"expired", "invalid", "storage"} {
		t.Run(mode, func(t *testing.T) {
			challenge, cookie := coveragePasskeyChallenge(t, passkeys, true, session)
			if mode == "invalid" {
				challenge = "different-challenge"
			}
			request := coveragePasskeyRequest(t, device.registration(t, challenge), session)
			if mode != "expired" {
				request.AddCookie(cookie)
			}
			store.fail = mode == "storage"
			response := httptest.NewRecorder()
			passkeys.finishRegistration(response, request)
			store.fail = false
			assertCoverageAPIStatus(t, response, http.StatusBadRequest)
			owner, err := manager.PasskeyOwner()
			if err != nil || len(owner.Credentials) != 0 {
				t.Fatal("failed registration changed credentials")
			}
		})
	}
}
