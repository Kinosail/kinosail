package identitycore

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestManagementSessionCannotBeReplayedOnLANPublicOrAnotherDevice(t *testing.T) {
	fixture := newSessionFixture()
	fixture.profiles[0].Owner = true
	sessions := requestSessionFixture(fixture)
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	r := WithManagementDevice(httptest.NewRequest("POST", "https://server.example/login", nil), "viewer", key)
	w := httptest.NewRecorder()
	if err := sessions.SignInStrong(w, r, "viewer"); err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing management cookie")
	}
	r.AddCookie(cookies[0])
	value := fixture.values[SessionKey(cookies[0].Value)]
	if value.ManagementDevice != key || !sessions.RecentlyAuthenticated(r, time.Hour) {
		t.Fatal("session did not bind atomically")
	}
	for _, kind := range []string{"LAN", "public", "other-device", "other-owner"} {
		request := httptest.NewRequest("GET", "https://server.example/settings", nil)
		request.AddCookie(cookies[0])
		switch kind {
		case "public":
			request = markRemote(request)
		case "other-device":
			request = WithManagementDevice(request, "viewer", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("q", 32))))
		case "other-owner":
			request = WithManagementDevice(request, "other", key)
		}
		before := fixture.writes
		if SessionMatchesRequest(value, request) || sessions.RecentlyAuthenticated(request, time.Hour) || sessions.MarkStrong(request) == nil {
			t.Fatalf("cookie replay through %s", kind)
		}
		if err := sessions.SignOut(request); err != nil || fixture.writes != before {
			t.Fatalf("replay mutated state through %s", kind)
		}
	}
	before := fixture.writes
	if _, err := sessions.CreateForRequest(r, "other", "phone", true, true, ""); err == nil || fixture.writes != before {
		t.Fatal("paired device selected another Owner")
	}
}

func TestSensitiveOwnerReadsRequireFreshAuthentication(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, recent := range []bool{false, true} {
			called := false
			record := &recordedResponse{}
			w := httptest.NewRecorder()
			OwnerSensitive(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), ownerConfig(record, true, false, false, recent, true)).ServeHTTP(w, httptest.NewRequest(method, "/api/v1/backup", nil))
			if called != recent || !recent && w.Code != http.StatusForbidden {
				t.Fatalf("%s recent=%v status=%d", method, recent, w.Code)
			}
		}
	}
}

func TestManagementBootstrapExcludesIntegrationAndSetupCredentials(t *testing.T) {
	for _, pattern := range []string{"GET /login", "POST /login", "POST /login/mfa", "GET /static/app.css", "POST /auth/passkeys/login/finish"} {
		if !ManagementBootstrapRoute(pattern) {
			t.Fatalf("missing login resource %s", pattern)
		}
	}
	for _, pattern := range []string{"POST /setup", "POST /api/v1/setup", "GET /scim/v2/Users", "POST /oauth/token", "POST /api/v1/home-assistant/pair", "POST /static/anything", "GET /settings"} {
		if ManagementBootstrapRoute(pattern) {
			t.Fatalf("management bypass %s", pattern)
		}
	}
}
