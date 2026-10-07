package homeassistant

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// These registered public-route tests control time/profile boundaries that the
// populated browser cannot force. They are isolated protocol proof, not app E2E.
func claimedHTTPTest(t *testing.T) (*testState, *Integration[testProfile], func(string, string, string, ...string) *httptest.ResponseRecorder) {
	t.Helper()
	state, integration, _ := registeredHTTPTest(t)
	sequence := 0
	integration.text = func() string { sequence++; return fmt.Sprintf("fixture_claim_%032d", sequence) }
	mux := http.NewServeMux()
	integration.Register(mux, func(next http.Handler) http.Handler { return next }, func(http.ResponseWriter, *http.Request, Approval) error { return nil })
	call := func(method, path, body string, claims ...string) *httptest.ResponseRecorder {
		req := request(method, path, strings.NewReader(body))
		for _, claim := range claims {
			req.Header.Add("X-Kinosail-Player-Claim", claim)
		}
		return response(mux, req)
	}
	return state, integration, call
}

type claimedHTTPReply struct {
	ID        string `json:"id"`
	Claim     string `json:"claim"`
	ExpiresIn int    `json:"expiresIn"`
}

func readClaimReply(t *testing.T, got *httptest.ResponseRecorder) claimedHTTPReply {
	t.Helper()
	var claim claimedHTTPReply
	if got.Code != http.StatusCreated || json.Unmarshal(got.Body.Bytes(), &claim) != nil || !playerID.MatchString(claim.ID) || len(claim.Claim) < 20 || len(claim.Claim) > 64 || claim.ExpiresIn != 30 || got.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("claim response = status %d, valid fields required", got.Code)
	}
	return claim
}

const claimedPlayerJSON = `{"name":"Fictional web tab","state":"paused","position":1,"duration":12,"volume":0.5}`

func TestRegisteredClaimRequiresDocumentOwnershipBeforeStateAndCommandDrain(t *testing.T) {
	state, _, call := claimedHTTPTest(t)
	claim := readClaimReply(t, call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"fixture-tab"}`))
	path := "/api/v1/home-assistant/players/" + claim.ID
	if got := call(http.MethodPut, path, claimedPlayerJSON, claim.Claim); got.Code != http.StatusOK {
		t.Fatalf("owned state = %d", got.Code)
	}
	if got := call(http.MethodPost, path+"/commands", `{"command":"seek","position":4}`); got.Code != http.StatusAccepted {
		t.Fatalf("queue harmless seek = %d", got.Code)
	}
	for _, headers := range [][]string{nil, {"wrong_claim_000000000000000000"}, {claim.Claim, claim.Claim}, {strings.Repeat("a", 65)}} {
		if got := call(http.MethodPut, path, strings.Replace(claimedPlayerJSON, `"position":1`, `"position":9`, 1), headers...); got.Code != http.StatusForbidden {
			t.Fatalf("unowned state = %d", got.Code)
		}
	}
	state.profile.ID = "other-profile"
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"fixture-tab"}`); got.Code != http.StatusForbidden {
		t.Fatalf("other profile claim = %d", got.Code)
	}
	if got := call(http.MethodPut, path, claimedPlayerJSON, claim.Claim); got.Code != http.StatusForbidden {
		t.Fatalf("other profile state = %d", got.Code)
	}
	if got := call(http.MethodPost, path+"/release", `{}`, claim.Claim); got.Code != http.StatusForbidden {
		t.Fatalf("other profile release = %d", got.Code)
	}
	state.profile.ID = "owner"
	if got := call(http.MethodGet, "/api/v1/home-assistant/players", ""); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"position":1`) || strings.Contains(got.Body.String(), `"position":9`) {
		t.Fatal("rejected state altered the published player")
	}
	if got := call(http.MethodPut, path, claimedPlayerJSON, claim.Claim); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"command":"seek"`) || !strings.Contains(got.Body.String(), `"position":4`) {
		t.Fatal("rejected ownership drained the addressed command")
	}
	if got := call(http.MethodPut, path, claimedPlayerJSON, claim.Claim); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"command":null`) {
		t.Fatal("the acknowledged command was dispatched more than once")
	}
}

func TestRegisteredClaimConflictCannotRefreshAndStaleReleaseCannotDeleteNewOwner(t *testing.T) {
	state, _, call := claimedHTTPTest(t)
	first := readClaimReply(t, call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"fixture-tab"}`))
	path := "/api/v1/home-assistant/players/" + first.ID
	state.now = state.now.Add(20 * time.Second)
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"fixture-tab"}`); got.Code != http.StatusConflict || got.Header().Get("Retry-After") != "10" {
		t.Fatalf("occupied candidate must report remaining lease, status=%d retry=%q", got.Code, got.Header().Get("Retry-After"))
	}
	if got := call(http.MethodPut, path, claimedPlayerJSON); got.Code != http.StatusForbidden {
		t.Fatalf("missing token = %d", got.Code)
	}
	state.now = state.now.Add(11 * time.Second)
	second := readClaimReply(t, call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"fixture-tab"}`))
	if second.ID != first.ID || second.Claim == first.Claim {
		t.Fatal("expired candidate must keep identity and get new document ownership")
	}
	for _, operation := range []string{http.MethodPut, http.MethodPost} {
		target, body := path, claimedPlayerJSON
		if operation == http.MethodPost {
			target, body = path+"/release", `{}`
		}
		if got := call(operation, target, body, first.Claim); got.Code != http.StatusForbidden {
			t.Fatalf("stale %s = %d", operation, got.Code)
		}
	}
	if got := call(http.MethodPut, path, claimedPlayerJSON, second.Claim); got.Code != http.StatusOK {
		t.Fatal("stale release deleted the newer owner")
	}
	if got := call(http.MethodPost, path+"/release", `{}`, second.Claim); got.Code != http.StatusNoContent {
		t.Fatalf("owned pagehide release = %d", got.Code)
	}
	third := readClaimReply(t, call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"fixture-tab"}`))
	if third.ID != first.ID || third.Claim == second.Claim {
		t.Fatal("released reload changed target or reused ownership")
	}
}

func TestRegisteredClaimsShareNativeCapacityWithoutPublishingEmptyReservations(t *testing.T) {
	state, _, call := claimedHTTPTest(t)
	if got := call(http.MethodPut, "/api/v1/home-assistant/players/native-fixture", claimedPlayerJSON); got.Code != http.StatusOK {
		t.Fatal("unchanged native registration failed")
	}
	var first claimedHTTPReply
	for index := 0; index < 63; index++ {
		claim := readClaimReply(t, call(http.MethodPost, "/api/v1/home-assistant/players/claims", fmt.Sprintf(`{"id":"fixture-%d"}`, index)))
		if index == 0 {
			first = claim
		}
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"fixture-overflow"}`); got.Code != http.StatusTooManyRequests {
		t.Fatalf("65th reservation = %d", got.Code)
	}
	if got := call(http.MethodGet, "/api/v1/home-assistant/players", ""); got.Code != http.StatusOK || strings.Contains(got.Body.String(), `"id":"fixture-`) || !strings.Contains(got.Body.String(), `"id":"native-fixture"`) {
		t.Fatal("empty reservation became a player target")
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/"+first.ID+"/commands", `{"command":"pause"}`); got.Code != http.StatusNotFound {
		t.Fatalf("unpublished target command = %d", got.Code)
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"fixture-0"}`); got.Code != http.StatusConflict {
		t.Fatal("unpublished command rejection discarded reservation")
	}
	state.now = state.now.Add(31 * time.Second)
	readClaimReply(t, call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"after-expiry"}`))
	if got := call(http.MethodPut, "/api/v1/home-assistant/players/"+first.ID, claimedPlayerJSON, first.Claim); got.Code != http.StatusForbidden {
		t.Fatal("stale ownership recreated a tokenless native target")
	}
}

func TestRegisteredClaimsKeepStrictNativeAndClaimContracts(t *testing.T) {
	_, _, call := claimedHTTPTest(t)
	for _, body := range []string{`{"id":"bad id"}`, `{"id":"` + strings.Repeat("a", 65) + `"}`, `{"id":"one","id":"two"}`, `{"id":"one","claim":"client-owned"}`, `{} {}`, `{`} {
		if got := call(http.MethodPost, "/api/v1/home-assistant/players/claims", body); got.Code != http.StatusBadRequest {
			t.Fatalf("ambiguous claim accepted: %d", got.Code)
		}
	}
	generated := readClaimReply(t, call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{}`))
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/"+generated.ID+"/release", `{"extra":true}`, generated.Claim); got.Code != http.StatusBadRequest {
		t.Fatal("release accepted unknown JSON")
	}
	if got := call(http.MethodPut, "/api/v1/home-assistant/players/native-fixture", strings.TrimSuffix(claimedPlayerJSON, "}")+`,"claim":"extra"}`); got.Code != http.StatusBadRequest {
		t.Fatal("native state schema became permissive")
	}
	if got := call(http.MethodPut, "/api/v1/home-assistant/players/native-fixture", claimedPlayerJSON); got.Code != http.StatusOK {
		t.Fatal("unclaimed native state requires new credential")
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"native-fixture"}`); got.Code != http.StatusConflict {
		t.Fatal("claim converted active native identity")
	}
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/native-fixture/commands", `{"command":"seek","position":4}`); got.Code != http.StatusAccepted {
		t.Fatal("native command contract changed")
	}
	if got := call(http.MethodPut, "/api/v1/home-assistant/players/native-fixture", claimedPlayerJSON); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"command":"seek"`) {
		t.Fatal("native command drain changed")
	}
}

func TestRegisteredClaimRestartAndFeatureDisableRevokeOldDocumentAuthority(t *testing.T) {
	_, firstIntegration, firstCall := claimedHTTPTest(t)
	first := readClaimReply(t, firstCall(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"stable-fixture"}`))
	_, restarted, restartedCall := claimedHTTPTest(t)
	restarted.text = func() string { return "restarted_document_claim_0000000000" }
	second := readClaimReply(t, restartedCall(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"stable-fixture"}`))
	if second.ID != first.ID || second.Claim == first.Claim {
		t.Fatal("restart changed the persisted candidate or retained old authority")
	}
	path := "/api/v1/home-assistant/players/stable-fixture"
	if got := restartedCall(http.MethodPut, path, claimedPlayerJSON, first.Claim); got.Code != http.StatusForbidden {
		t.Fatal("old process token controls restarted target")
	}
	if got := restartedCall(http.MethodPut, path, claimedPlayerJSON, second.Claim); got.Code != http.StatusOK {
		t.Fatal("restarted target cannot publish state")
	}
	if err := firstIntegration.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	if got := firstCall(http.MethodPost, path+"/release", `{}`, first.Claim); got.Code != http.StatusNotFound {
		t.Fatal("disabled feature accepted a browser mutation")
	}
	if err := firstIntegration.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	if got := firstCall(http.MethodPut, path, claimedPlayerJSON, first.Claim); got.Code != http.StatusForbidden {
		t.Fatal("reenabling restored revoked document authority")
	}
}
