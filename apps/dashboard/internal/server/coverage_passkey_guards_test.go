package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-dashboard/internal/auth"
)

func TestCoveragePasskeyFinishTransportGuards(t *testing.T) {
	manager, _, session := coverageAuthentication(t, true)
	passkeys := newPasskeyAuth("https://dashboard.example:38400", manager, true)
	for _, test := range []struct {
		body   string
		status int
	}{{"{", http.StatusBadRequest}, {`{}`, http.StatusUnauthorized}} {
		response := httptest.NewRecorder()
		passkeys.finishLogin(response, coveragePasskeyRequest(t, test.body, session))
		assertCoverageAPIStatus(t, response, test.status)
	}
	response := httptest.NewRecorder()
	passkeys.finishRegistration(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
	assertCoverageAPIStatus(t, response, http.StatusUnauthorized)
	for _, handler := range []http.HandlerFunc{passkeys.finishRegistration, passkeys.finishLogin} {
		response := httptest.NewRecorder()
		handler(response, coverageAuthRequest(t, `{}`, session))
		assertCoverageAPIStatus(t, response, http.StatusMisdirectedRequest)
	}
	session.ViaBearer = true
	response = httptest.NewRecorder()
	passkeys.beginRegistration(response, coveragePasskeyRequest(t, "", session))
	assertCoverageAPIStatus(t, response, http.StatusForbidden)
}

func TestCoveragePasskeyUnavailableAndOwnerState(t *testing.T) {
	manager, _, _ := coverageAuthentication(t, false)
	session := auth.Session{Identity: auth.Identity{ID: "Owner", CSRF: "verification"}}
	broken := newPasskeyAuth("ftp://invalid.test", manager, true)
	for _, handler := range []http.HandlerFunc{broken.beginRegistration, broken.finishRegistration, broken.beginLogin, broken.finishLogin} {
		response := httptest.NewRecorder()
		handler(response, coveragePasskeyRequest(t, "", session))
		assertCoverageAPIStatus(t, response, http.StatusServiceUnavailable)
	}
	passkeys := newPasskeyAuth("https://dashboard.example:38400", manager, true)
	response := httptest.NewRecorder()
	passkeys.beginRegistration(response, coveragePasskeyRequest(t, "", session))
	assertCoverageAPIStatus(t, response, http.StatusServiceUnavailable)
	device := newCoverageAuthenticator(t)
	response = httptest.NewRecorder()
	passkeys.finishRegistration(response, coveragePasskeyRequest(t, device.registration(t, "challenge"), session))
	assertCoverageAPIStatus(t, response, http.StatusServiceUnavailable)
}

func TestCoveragePasskeyCapacityAndLoginLimit(t *testing.T) {
	manager, _, session := coverageAuthentication(t, true)
	passkeys := newPasskeyAuth("https://dashboard.example:38400", manager, true)
	passkeys.limiter.global = loginBucket{count: globalLogins, until: time.Now().Add(time.Minute)}
	response := httptest.NewRecorder()
	passkeys.beginLogin(response, coveragePasskeyRequest(t, "", session))
	assertCoverageAPIStatus(t, response, http.StatusTooManyRequests)
	if response.Header().Get("Retry-After") == "" {
		t.Fatal("limited login lacks retry time")
	}
	passkeys.limiter = newLoginLimiter()
	for range 1024 {
		if _, _, err := passkeys.engine.BeginLogin(passkeys.ceremony("")); err != nil {
			t.Fatal(err)
		}
	}
	for _, handler := range []http.HandlerFunc{passkeys.beginRegistration, passkeys.beginLogin} {
		response := httptest.NewRecorder()
		handler(response, coveragePasskeyRequest(t, "", session))
		assertCoverageAPIStatus(t, response, http.StatusServiceUnavailable)
	}
}

func TestCoveragePasskeyDeviceNames(t *testing.T) {
	for _, test := range []struct{ input, want string }{{"", "Browser"}, {" Device ", "Device"}, {strings.Repeat("x", 81), strings.Repeat("x", 80)}} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
		request.Header.Set("User-Agent", test.input)
		if got := passkeyDevice(request); got != test.want {
			t.Fatalf("device = %q", got)
		}
	}
}
