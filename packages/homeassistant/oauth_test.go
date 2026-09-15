package homeassistant

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const testRedirect = "http://homeassistant.local:8123/auth/external/callback"

func oauthQuery(verifier string) url.Values {
	digest := sha256.Sum256([]byte(verifier))
	return url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {testRedirect},
		"state":                 {"state"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(digest[:])},
		"code_challenge_method": {"S256"},
	}
}

func TestAuthorizationValidation(t *testing.T) { //nolint:cyclop // One boundary matrix proves the complete authorization request contract.
	state := &testState{enabled: true, profile: Profile[testProfile]{Source: testProfile{"owner"}, ID: "owner", Name: "Owner", Owner: true}}
	integration := newTestIntegration(t, state)
	verifier := strings.Repeat("A", 43)
	valid := oauthQuery(verifier)
	transaction, err := integration.authorizationRequest(request(http.MethodGet, "/?"+valid.Encode(), nil), state.profile)
	if err != nil || transaction.RedirectURI != testRedirect || transaction.ProfileID != "owner" || !transaction.Expires.Equal(state.now.Add(authorizationTTL)) {
		t.Fatalf("authorizationRequest = %#v, %v", transaction, err)
	}
	mutations := []func(url.Values, *Profile[testProfile]){
		func(values url.Values, _ *Profile[testProfile]) { values["extra"] = []string{"x"} },
		func(values url.Values, _ *Profile[testProfile]) { values.Del("response_type") },
		func(values url.Values, _ *Profile[testProfile]) { values["response_type"] = []string{"token"} },
		func(values url.Values, _ *Profile[testProfile]) { values["client_id"] = []string{"other"} },
		func(values url.Values, _ *Profile[testProfile]) {
			values["redirect_uri"] = []string{"ftp://example.com/auth/external/callback"}
		},
		func(values url.Values, _ *Profile[testProfile]) { values["state"] = []string{""} },
		func(values url.Values, _ *Profile[testProfile]) { values["code_challenge"] = []string{"short"} },
		func(values url.Values, _ *Profile[testProfile]) { values["code_challenge_method"] = []string{"plain"} },
		func(_ url.Values, profile *Profile[testProfile]) { profile.Owner = false },
	}
	for index, mutate := range mutations {
		values := oauthQuery(verifier)
		profile := state.profile
		mutate(values, &profile)
		if _, requestErr := integration.authorizationRequest(request(http.MethodGet, "/?"+values.Encode(), nil), profile); requestErr == nil {
			t.Errorf("authorization mutation %d accepted", index)
		}
	}
	for _, redirect := range []struct {
		value string
		valid bool
	}{
		{"https://my.home-assistant.io/redirect/oauth", true},
		{"https://example.com/auth/external/callback", true},
		{"http://127.0.0.1:8123/auth/external/callback", true},
		{"://bad", false},
		{"https:///auth/external/callback", false},
		{"https://user@example.com/auth/external/callback", false},
		{"https://example.com/auth/external/callback#fragment", false},
		{"https://example.com/auth/external/callback?query=x", false},
		{"https://example.com/wrong", false},
		{"ftp://example.com/auth/external/callback", false},
	} {
		if got := validRedirect(redirect.value); got != redirect.valid {
			t.Errorf("validRedirect(%q) = %v", redirect.value, got)
		}
	}
	path := "/auth/external/callback"
	boundary := "https://" + strings.Repeat("a", 2048-len("https://")-len(path)) + path
	if !validRedirect(boundary) || validRedirect("a"+boundary) {
		t.Fatal("redirect length boundary mismatch")
	}
}

func TestAuthorizeHTTP(t *testing.T) { //nolint:cyclop // The score of 11 remains below the repository ceiling of 22 for the authorization matrix.
	state := &testState{enabled: true, profile: Profile[testProfile]{Source: testProfile{"owner"}, ID: "owner", Name: "Owner", Owner: true}}
	integration := newTestIntegration(t, state)
	verifier := strings.Repeat("A", 43)
	query := oauthQuery(verifier)
	var view Approval
	handler := func(w http.ResponseWriter, r *http.Request) {
		integration.authorizeHTTP(w, r, func(_ http.ResponseWriter, _ *http.Request, input Approval) error {
			view = input
			return errors.New("ignored")
		})
	}
	got := response(http.HandlerFunc(handler), request(http.MethodGet, "/?"+query.Encode(), nil))
	if got.Code != http.StatusOK || got.Header().Get("Cache-Control") != "no-store" || view.ProfileName != "Owner" || view.RequestID != "random-text" || view.Destination != "homeassistant.local:8123" {
		t.Fatalf("authorize = %d %#v", got.Code, view)
	}
	if len(integration.requests) != 1 {
		t.Fatalf("requests = %d", len(integration.requests))
	}
	invalid := response(http.HandlerFunc(handler), request(http.MethodGet, "/", nil))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid authorize = %d", invalid.Code)
	}
	integration.requests = make(map[string]authorization)
	for index := range oauthLimit {
		integration.requests[string(rune(index+1))] = authorization{Expires: state.now.Add(time.Minute)}
	}
	limited := response(http.HandlerFunc(handler), request(http.MethodGet, "/?"+query.Encode(), nil))
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("limited authorize = %d", limited.Code)
	}
	post := formRequest(http.MethodPost, "/", url.Values{})
	if result := response(http.HandlerFunc(handler), post); result.Code != http.StatusBadRequest {
		t.Fatalf("authorize POST = %d", result.Code)
	}
}

func TestApproveAuthorizationHTTP(t *testing.T) { //nolint:cyclop // The score of 12 remains below the repository ceiling of 22 for the approval matrix.
	state := &testState{enabled: true, profile: Profile[testProfile]{Source: testProfile{"owner"}, ID: "owner", Name: "Owner", Owner: true}}
	integration := newTestIntegration(t, state)
	integration.requests = map[string]authorization{secretHash("request"): {RedirectURI: testRedirect, State: "state", ProfileID: "owner", Expires: state.now.Add(time.Minute)}}
	approve := func(values url.Values) *httptest.ResponseRecorder {
		return response(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { integration.approveAuthorization(w, r) }), formRequest(http.MethodPost, "/", values))
	}
	for _, values := range []url.Values{
		{"extra": {"x"}},
		{"request": {"request"}, "decision": {"bad"}},
		{"request": {"missing"}, "decision": {"allow"}},
	} {
		if result := approve(values); result.Code != http.StatusBadRequest {
			t.Errorf("approve(%v) = %d", values, result.Code)
		}
	}
	state.profile.Owner = false
	if result := approve(url.Values{"request": {"request"}, "decision": {"allow"}}); result.Code != http.StatusBadRequest {
		t.Fatalf("non-owner approve = %d", result.Code)
	}
	state.profile.Owner = true
	integration.requests[secretHash("deny")] = authorization{RedirectURI: testRedirect, State: "state", ProfileID: "owner", Expires: state.now.Add(time.Minute)}
	denied := approve(url.Values{"request": {"deny"}, "decision": {"deny"}})
	if denied.Code != http.StatusFound || !strings.Contains(denied.Header().Get("Location"), "error=access_denied") || !strings.Contains(denied.Header().Get("Location"), "state=state") {
		t.Fatalf("deny = %d %s", denied.Code, denied.Header().Get("Location"))
	}
	integration.requests[secretHash("request")] = authorization{RedirectURI: testRedirect, State: "state", ProfileID: "owner", Expires: state.now.Add(time.Minute)}
	allowed := approve(url.Values{"request": {"request"}, "decision": {"allow"}})
	if allowed.Code != http.StatusFound || !strings.Contains(allowed.Header().Get("Location"), "code=random-text") || len(integration.codes) != 1 {
		t.Fatalf("allow = %d %s codes=%d", allowed.Code, allowed.Header().Get("Location"), len(integration.codes))
	}

	integration.requests[secretHash("limit")] = authorization{RedirectURI: testRedirect, State: "state", ProfileID: "owner", Expires: state.now.Add(time.Minute)}
	integration.codes = make(map[string]authorization)
	for index := range oauthLimit {
		integration.codes[string(rune(index+1))] = authorization{Expires: state.now.Add(time.Minute)}
	}
	if result := approve(url.Values{"request": {"limit"}, "decision": {"allow"}}); result.Code != http.StatusTooManyRequests {
		t.Fatalf("code limit = %d", result.Code)
	}
}

func TestTokenExchange(t *testing.T) { //nolint:cyclop,gocognit,funlen // One matrix proves every token boundary and side effect.
	state := &testState{enabled: true, profile: Profile[testProfile]{Source: testProfile{"owner"}, ID: "owner", Owner: true}}
	integration := newTestIntegration(t, state)
	verifier := strings.Repeat("A", 43)
	digest := sha256.Sum256([]byte(verifier))
	transaction := authorization{RedirectURI: testRedirect, Challenge: base64.RawURLEncoding.EncodeToString(digest[:]), ProfileID: "owner", Expires: state.now.Add(time.Minute)}
	valid := url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {"code"}, "redirect_uri": {testRedirect}, "code_verifier": {verifier}}
	exchange := func(values url.Values) *httptest.ResponseRecorder {
		return response(http.HandlerFunc(integration.tokenHTTP), formRequest(http.MethodPost, "/", values))
	}

	for _, mutate := range []func(*http.Request){
		func(request *http.Request) { request.Header.Set("Content-Type", "text/plain") },
		func(request *http.Request) { request.Header.Set("Authorization", "Basic x") },
		func(request *http.Request) { request.URL.RawQuery = "x=y" },
		func(request *http.Request) { request.ContentLength = 16<<10 + 1 },
	} {
		req := formRequest(http.MethodPost, "/", valid)
		mutate(req)
		if got := response(http.HandlerFunc(integration.tokenHTTP), req); got.Code != http.StatusBadRequest || !strings.Contains(got.Body.String(), "invalid_request") {
			t.Errorf("invalid token envelope = %d %s", got.Code, got.Body.String())
		}
	}
	boundaryLength := formRequest(http.MethodPost, "/", valid)
	boundaryLength.ContentLength = 16 << 10
	if got := response(http.HandlerFunc(integration.tokenHTTP), boundaryLength); got.Code != http.StatusBadRequest || !strings.Contains(got.Body.String(), "invalid_grant") {
		t.Fatalf("token content-length boundary = %d %s", got.Code, got.Body.String())
	}
	if got := exchange(url.Values{"extra": {"x"}}); got.Code != http.StatusBadRequest || !strings.Contains(got.Body.String(), "invalid_request") {
		t.Fatalf("unknown token field = %d %s", got.Code, got.Body.String())
	}
	for _, mutation := range []struct{ key, value string }{{"grant_type", "bad"}, {"client_id", "bad"}, {"code", ""}, {"redirect_uri", "bad"}, {"code_verifier", "short"}} {
		values := valid.Clone()
		values.Set(mutation.key, mutation.value)
		if got := exchange(values); got.Code != http.StatusBadRequest || !strings.Contains(got.Body.String(), "invalid_grant") {
			t.Errorf("token %s = %d %s", mutation.key, got.Code, got.Body.String())
		}
	}
	if got := exchange(valid); got.Code != http.StatusBadRequest {
		t.Fatalf("missing code = %d", got.Code)
	}

	integration.codes[secretHash("code")] = transaction
	state.profile.ID = "other"
	if got := exchange(valid); got.Code != http.StatusBadRequest {
		t.Fatalf("missing profile = %d", got.Code)
	}
	integration.codes[secretHash("code")] = transaction
	state.profile.ID = "owner"
	state.profile.Owner = false
	if got := exchange(valid); got.Code != http.StatusBadRequest {
		t.Fatalf("non-owner profile = %d", got.Code)
	}
	integration.codes[secretHash("code")] = transaction
	state.profile.Owner = true
	state.createErr = errors.New("create")
	if got := exchange(valid); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("create failure = %d", got.Code)
	}
	integration.codes[secretHash("code")] = transaction
	state.createErr = nil
	got := exchange(valid)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"access_token":"token-owner"`) || !strings.Contains(got.Body.String(), `"serverId":"server-id"`) {
		t.Fatalf("exchange = %d %s", got.Code, got.Body.String())
	}
	if replay := exchange(valid); replay.Code != http.StatusBadRequest {
		t.Fatalf("replay = %d", replay.Code)
	}
}

func TestOAuthPruning(t *testing.T) {
	state := &testState{enabled: true}
	integration := newTestIntegration(t, state)
	integration.requests["expired"] = authorization{Expires: state.now}
	integration.requests["active"] = authorization{Expires: state.now.Add(time.Second)}
	integration.codes["expired"] = authorization{Expires: state.now}
	integration.codes["active"] = authorization{Expires: state.now.Add(time.Second)}
	integration.pruneOAuthLocked(state.now)
	if len(integration.requests) != 1 || len(integration.codes) != 1 {
		t.Fatalf("prune = requests %d codes %d", len(integration.requests), len(integration.codes))
	}
}
