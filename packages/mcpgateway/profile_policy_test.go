package mcpgateway

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func TestMCPGrantPolicyRechecksRequiredEnrollment(t *testing.T) {
	var mutex sync.RWMutex
	var stateErr error
	profiles := []identitycore.Profile{{ID: "owner", Owner: true}, {ID: "viewer"}}
	required := false
	config := profilePrincipalTestConfig(&mutex, &profiles, &stateErr)
	config.MFARequired = func() bool { return required }
	repository := NewProfilePrincipals(config)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/mcp", nil)
	allowed := func(id string) bool { return repository.Allowed(Principal{ID: id}, request, time.Now()) }
	if allowed("owner") || !allowed("viewer") {
		t.Fatal("initial enrollment policy was not enforced")
	}
	required = true
	if allowed("viewer") {
		t.Fatal("policy change left an unsecured grant usable")
	}
	profiles[1].TOTPSecret = "secret"
	if !allowed("viewer") {
		t.Fatal("secured Viewer was rejected")
	}
	profiles[1].TOTPSecret = ""
	if allowed("viewer") {
		t.Fatal("factor removal left a grant usable")
	}
	config.MFARequired = nil
	if NewProfilePrincipals(config) != nil {
		t.Fatal("missing policy adapter accepted")
	}
}

func TestMCPGrantCannotFollowAChangedProfileRevision(t *testing.T) {
	profile := Principal{ID: "viewer", Revision: 4}
	principals := &testPrincipals{values: map[string]Principal{profile.ID: profile}}
	state := &memoryState{}
	connections := testConnections("https://kino.test:38127", principals, state)
	now := time.Now()
	connections.grants["grant"] = mcpOAuthGrant{ID: "grant", ProfileID: profile.ID, ProfileRevision: 3, AccessHash: secretHash("access"), AccessExpires: now.Add(time.Hour).Unix()}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/mcp", nil)
	if info, err := connections.VerifyToken(t.Context(), "access", request); err == nil || info != nil || state.saves != 0 {
		t.Fatal("old identity grant survived replacement")
	}
}

func TestPublicMetadataClientDoesNotUseEnvironmentProxy(t *testing.T) {
	client := publicMetadataHTTPClient(time.Second)
	if client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("protected metadata requests retained an unchecked proxy")
	}
}
