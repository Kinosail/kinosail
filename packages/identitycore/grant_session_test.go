package identitycore

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRevisionBoundSessionGrants(t *testing.T) {
	t.Parallel()
	for _, compatibility := range []bool{false, true} {
		fixture := newSessionFixture()
		sessions := fixture.sessions()
		if _, err := sessions.CreateLocalGrant("viewer", "Device", 6, compatibility); err == nil || fixture.writes != 0 {
			t.Fatal("stale approval issued a session")
		}
		token, err := sessions.CreateLocalGrant("viewer", "Device", 7, compatibility)
		if err != nil || fixture.writes != 1 {
			t.Fatalf("grant=%q %v", token, err)
		}
		want := ""
		if compatibility {
			want = "compatibility"
		}
		if state := fixture.values[SessionKey(token)]; state.Channel != want || state.Browser || state.ProfileRevision != 7 {
			t.Fatalf("grant state=%#v", state)
		}
	}
}

func TestPublicGrantCookieRequiresCurrentAuthorizedProfile(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"success", "stale", "owner", "local only", "unsecured", "persistence", "management", "nil writer", "nil request"} {
		t.Run(scenario, func(t *testing.T) {
			assertPublicGrantScenario(t, scenario)
		})
	}
}

func assertPublicGrantScenario(t *testing.T, scenario string) {
	t.Helper()
	fixture := newSessionFixture()
	sessions := requestSessionFixture(fixture)
	request := httptest.NewRequestWithContext(t.Context(), "POST", "https://example.test/", nil)
	response := httptest.NewRecorder()
	request, revision := publicGrantScenario(fixture, request, scenario)
	var err error
	if scenario == "nil writer" {
		err = sessions.SignInPublicGrant(nil, request, "viewer", revision)
	} else {
		err = sessions.SignInPublicGrant(response, request, "viewer", revision)
	}
	if scenario == "success" {
		if err != nil || len(response.Result().Cookies()) != 1 || fixture.values[SessionKey("token")].Channel != "public" {
			t.Fatalf("public grant=%v %v", response.Result().Cookies(), err)
		}
		return
	}
	if err == nil || len(response.Result().Cookies()) != 0 || len(fixture.values) != 0 {
		t.Fatalf("rejected grant changed state: %v %#v", err, fixture.values)
	}
}

func TestManagementSessionNilAndMissingBinding(t *testing.T) {
	t.Parallel()
	if ManagementProfileID(nil) != "" || ManagementDeviceKey(nil) != "" || SessionMatchesRequest(Session{}, nil) {
		t.Fatal("nil request has authority")
	}
	fixture := newSessionFixture()
	sessions := requestSessionFixture(fixture)
	if _, err := sessions.CreateForRequest(nil, "viewer", "Device", false, false, ""); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("nil request=%v", err)
	}
	if _, err := sessions.CreateForRequest(httptest.NewRequestWithContext(t.Context(), "POST", "/", nil), "viewer", "Device", false, false, ""); err != nil {
		t.Fatal(err)
	}
	profile := SessionProfile{Owner: false, Secured: true}
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	if err := validateSessionAuthority(profile, "", nil, key); !errors.Is(err, ErrSessionKind) {
		t.Fatal("viewer received owner management authority")
	}
}

func publicGrantScenario(fixture *sessionFixture, request *http.Request, scenario string) (*http.Request, uint64) {
	revision := uint64(7)
	switch scenario {
	case "stale":
		revision = 6
	case "owner":
		fixture.profiles[0].Owner = true
	case "local only":
		fixture.profiles[0].Remote = false
	case "unsecured":
		fixture.profiles[0].Secured = false
	case "persistence":
		fixture.persist = errors.New("blocked")
	case "management":
		request = WithManagementDevice(request, "viewer", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	case "nil request":
		request = nil
	}
	return request, revision
}
