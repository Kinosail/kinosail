package homeassistant

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Registered routes control the disable interleaving and lease clock that a
// populated browser cannot force. These are isolated protocol regressions.
func TestRegisteredClaimCannotMintAfterAdmittedRequestIsDisabled(t *testing.T) {
	_, integration, call := claimedHTTPTest(t)
	profile := integration.config.CurrentProfile
	reached := false
	integration.config.CurrentProfile = func(request *http.Request) Profile[testProfile] {
		reached = true
		if err := integration.SetEnabled(false); err != nil {
			t.Fatal(err)
		}
		integration.config.CurrentProfile = profile
		return profile(request)
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"disable-boundary"}`); got.Code != http.StatusNotFound || !reached {
		t.Fatalf("admitted claim after disable: HTTP %d, reached=%t", got.Code, reached)
	}
	if err := integration.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	if got := call(http.MethodGet, "/api/v1/home-assistant/players", ""); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"players":[]`) {
		t.Fatal("disabled request left a published target")
	}
	// A surviving hidden reservation would make this return409 after re-enable.
	readClaimReply(t, call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"disable-boundary"}`))
}

type claimValidationCall = func(string, string, string, ...string) *httptest.ResponseRecorder

func rejectClaimedValidation(t *testing.T) (*testState, claimValidationCall, claimedHTTPReply) {
	t.Helper()
	state, _, call := claimedHTTPTest(t)
	claim := readClaimReply(t, call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"validation-boundary"}`))
	path := "/api/v1/home-assistant/players/" + claim.ID
	if got := call(http.MethodPut, path, claimedPlayerJSON, claim.Claim); got.Code != http.StatusOK {
		t.Fatal("owned state prerequisite failed")
	}
	if got := call(http.MethodPost, path+"/commands", `{"command":"seek","position":4}`); got.Code != http.StatusAccepted {
		t.Fatal("queued seek prerequisite failed")
	}
	state.now = state.now.Add(20 * time.Second)
	before := validationSnapshot(t, call)
	for _, input := range []struct{ method, suffix, body string }{
		{http.MethodPut, "", strings.Replace(claimedPlayerJSON, `"position":1`, `"position":-1`, 1)},
		{http.MethodPut, "", strings.TrimSuffix(claimedPlayerJSON, "}") + `,"claim":"unexpected"}`},
		{http.MethodPost, "/release", `{"unexpected":true}`},
	} {
		if got := call(input.method, path+input.suffix, input.body, claim.Claim); got.Code != http.StatusBadRequest {
			t.Fatalf("claimed validation rejection: HTTP %d", got.Code)
		}
		if after := validationSnapshot(t, call); after != before {
			t.Fatal("invalid claimed request changed the published snapshot")
		}
	}
	return state, call, claim
}

func validationSnapshot(t *testing.T, call claimValidationCall) string {
	t.Helper()
	got := call(http.MethodGet, "/api/v1/home-assistant/players", "")
	if got.Code != http.StatusOK {
		t.Fatalf("snapshot prerequisite: HTTP %d", got.Code)
	}
	return got.Body.String()
}

func TestRegisteredClaimedValidationPreservesQueuedCommand(t *testing.T) {
	state, call, claim := rejectClaimedValidation(t)
	state.now = state.now.Add(9 * time.Second)
	path := "/api/v1/home-assistant/players/" + claim.ID
	if got := call(http.MethodPut, path, claimedPlayerJSON, claim.Claim); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"command":"seek"`) || !strings.Contains(got.Body.String(), `"position":4`) {
		t.Fatal("invalid claimed request consumed the queued seek")
	}
	if got := call(http.MethodPut, path, claimedPlayerJSON, claim.Claim); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"command":null`) {
		t.Fatal("retained seek drained more than once")
	}
}

func TestRegisteredClaimedValidationDoesNotRefreshLease(t *testing.T) {
	state, call, claim := rejectClaimedValidation(t)
	state.now = state.now.Add(11 * time.Second)
	path := "/api/v1/home-assistant/players/" + claim.ID
	if got := call(http.MethodPut, path, claimedPlayerJSON, claim.Claim); got.Code != http.StatusForbidden {
		t.Fatalf("invalid request refreshed the lease: HTTP %d", got.Code)
	}
	if !strings.Contains(validationSnapshot(t, call), `"players":[]`) {
		t.Fatal("expired claimed target remained published")
	}
}
