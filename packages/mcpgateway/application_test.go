package mcpgateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/federation"
)

type adapterProfile struct {
	id, name, issuer, subject string
	owner, allowed            bool
}

type adapterProfileKey struct{}

func adapterConfig(profiles *[]adapterProfile, stateErr *error, attributed *adapterProfile) PrincipalAdapterConfig[adapterProfile] {
	find := func(id string) (adapterProfile, bool) {
		for _, profile := range *profiles {
			if profile.id == id {
				return profile, true
			}
		}
		return adapterProfile{}, false
	}
	return PrincipalAdapterConfig[adapterProfile]{
		CurrentProfile: func(request *http.Request) adapterProfile {
			profile, _ := request.Context().Value(adapterProfileKey{}).(adapterProfile)
			return profile
		},
		FindProfile: find,
		FederatedProfiles: func() federation.Profiles[adapterProfile] {
			return federation.Profiles[adapterProfile]{
				Lock: func() {}, Unlock: func() {}, Clone: func() []adapterProfile { return slices.Clone(*profiles) },
				Inspect: func(profile adapterProfile) federation.Profile {
					return federation.Profile{ID: profile.id, OIDC: federation.Identity{Issuer: profile.issuer, Subject: profile.subject}}
				},
			}
		},
		AllowProfile:         func(profile adapterProfile, _ *http.Request, _ time.Time) bool { return profile.allowed },
		RecentAuthentication: func(*http.Request, time.Duration) bool { return true },
		Profiles:             func() []adapterProfile { return slices.Clone(*profiles) },
		StateError:           func() error { return *stateErr },
		AttributeProfile:     func(_ *http.Request, profile adapterProfile) { *attributed = profile },
		ConvertProfile: func(profile adapterProfile) Principal {
			return Principal{ID: profile.id, Name: profile.name, Owner: profile.owner}
		},
	}
}

func TestPrincipalAdapterPreservesIdentityDecisions(t *testing.T) { //nolint:cyclop,gocognit // One flow proves the complete principal adapter contract.
	profiles := []adapterProfile{
		{id: "owner", name: "Owner", issuer: "https://identity.test", subject: "owner-subject", owner: true, allowed: true},
		{id: "viewer", name: "Viewer"},
	}
	var stateErr error
	var attributed adapterProfile
	adapter := NewPrincipalAdapter(adapterConfig(&profiles, &stateErr, &attributed))
	request := httptest.NewRequestWithContext(context.WithValue(t.Context(), adapterProfileKey{}, profiles[0]), http.MethodGet, "/", nil)
	if current := adapter.Current(request); current.ID != "owner" || current.Name != "Owner" || !current.Owner {
		t.Fatalf("current principal = %+v", current)
	}
	if profile, found := adapter.ByID("viewer"); !found || profile != (Principal{ID: "viewer", Name: "Viewer"}) {
		t.Fatalf("profile lookup = %+v, %v", profile, found)
	}
	if profile, found := adapter.ByID("missing"); found || profile != (Principal{}) {
		t.Fatalf("missing profile lookup = %+v, %v", profile, found)
	}
	if profile, found := adapter.ByOIDC("https://identity.test", "owner-subject"); !found || profile.ID != "owner" {
		t.Fatalf("OIDC lookup = %+v, %v", profile, found)
	}
	if _, found := adapter.ByOIDC("", ""); found {
		t.Fatal("invalid OIDC identity was accepted")
	}
	if !adapter.Allowed(Principal{ID: "owner"}, request, time.Now()) || adapter.Allowed(Principal{ID: "viewer"}, request, time.Now()) || adapter.Allowed(Principal{ID: "missing"}, request, time.Now()) {
		t.Fatal("profile access decision changed")
	}
	if !adapter.RecentlyAuthenticated(request, time.Minute) {
		t.Fatal("recent authentication result changed")
	}
	owners, err := adapter.Owners()
	if err != nil || !slices.Equal(owners, []Principal{{ID: "owner", Name: "Owner", Owner: true}}) {
		t.Fatalf("owners = %+v, %v", owners, err)
	}
	adapter.Attribute(request, Principal{ID: "owner"})
	if attributed.id != "owner" {
		t.Fatalf("attributed profile = %+v", attributed)
	}
	attributed = adapterProfile{}
	adapter.Attribute(request, Principal{ID: "missing"})
	if attributed != (adapterProfile{}) {
		t.Fatalf("missing profile was attributed: %+v", attributed)
	}
	stateErr = errors.New("profile state unavailable")
	if owners, err = adapter.Owners(); !errors.Is(err, stateErr) || owners != nil {
		t.Fatalf("unavailable owners = %+v, %v", owners, err)
	}
}

func TestPrincipalAdapterRejectsMissingCallbacks(t *testing.T) {
	profiles := []adapterProfile{{id: "owner", owner: true}}
	var stateErr error
	var attributed adapterProfile
	mutations := []func(*PrincipalAdapterConfig[adapterProfile]){
		func(config *PrincipalAdapterConfig[adapterProfile]) { config.CurrentProfile = nil },
		func(config *PrincipalAdapterConfig[adapterProfile]) { config.FindProfile = nil },
		func(config *PrincipalAdapterConfig[adapterProfile]) { config.FederatedProfiles = nil },
		func(config *PrincipalAdapterConfig[adapterProfile]) { config.AllowProfile = nil },
		func(config *PrincipalAdapterConfig[adapterProfile]) { config.RecentAuthentication = nil },
		func(config *PrincipalAdapterConfig[adapterProfile]) { config.Profiles = nil },
		func(config *PrincipalAdapterConfig[adapterProfile]) { config.StateError = nil },
		func(config *PrincipalAdapterConfig[adapterProfile]) { config.AttributeProfile = nil },
		func(config *PrincipalAdapterConfig[adapterProfile]) { config.ConvertProfile = nil },
	}
	for index, mutate := range mutations {
		config := adapterConfig(&profiles, &stateErr, &attributed)
		mutate(&config)
		if adapter := NewPrincipalAdapter(config); adapter != nil {
			t.Fatalf("missing callback %d produced an adapter", index)
		}
	}
}

type (
	adapterViewerKey struct{}
	adapterOwnerKey  struct{}
)

func TestAPIAdapterPreservesRoutingContextAndBytes(t *testing.T) { //nolint:cyclop // One flow verifies contexts, audit order, and the exact response.
	profile := adapterProfile{id: "owner", name: "Owner", owner: true}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/me", func(http.ResponseWriter, *http.Request) {})
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Context().Value(adapterViewerKey{}) != profile || request.Context().Value(adapterOwnerKey{}) != true || auditjournal.ActorName(request.Context()) != "MCP · Owner" {
			t.Fatal("API context changed")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"name":"Owner"}`))
	})
	served := 0
	adapter := NewAPIAdapter(mux, handler, func(id string) (adapterProfile, bool) { return profile, id == profile.id }, adapterViewerKey{}, adapterOwnerKey{},
		func(next http.Handler, writer http.ResponseWriter, request *http.Request) {
			served++
			next.ServeHTTP(writer, request)
		})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me", nil)
	if pattern := adapter.Pattern(request); pattern != "GET /api/v1/me" {
		t.Fatalf("route pattern = %q", pattern)
	}
	response := httptest.NewRecorder()
	if err := adapter.Invoke(response, request, Principal{ID: "owner"}, "MCP · Owner", true); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || response.Body.String() != `{"name":"Owner"}` || response.Header().Get("Content-Type") != "application/json" || served != 1 {
		t.Fatalf("API response = %d %q %q, served=%d", response.Code, response.Body.String(), response.Header().Get("Content-Type"), served)
	}
	if err := adapter.Invoke(httptest.NewRecorder(), request, Principal{ID: "missing"}, "MCP", false); err == nil {
		t.Fatal("missing principal reached the API")
	}
}

func TestAPIAdapterRejectsMissingDependencies(t *testing.T) {
	mux := http.NewServeMux()
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("invalid adapter served a request") })
	find := func(string) (adapterProfile, bool) { return adapterProfile{}, false }
	serve := func(http.Handler, http.ResponseWriter, *http.Request) { t.Fatal("invalid adapter served a request") }
	adapters := []APIInvoker{
		NewAPIAdapter(nil, handler, find, adapterViewerKey{}, adapterOwnerKey{}, serve),
		NewAPIAdapter[adapterProfile](mux, nil, find, adapterViewerKey{}, adapterOwnerKey{}, serve),
		NewAPIAdapter[adapterProfile](mux, handler, nil, adapterViewerKey{}, adapterOwnerKey{}, serve),
		NewAPIAdapter(mux, handler, find, adapterViewerKey{}, adapterOwnerKey{}, nil),
	}
	for index, adapter := range adapters {
		if adapter != nil {
			t.Fatalf("invalid API dependency %d produced an adapter", index)
		}
	}
}

func TestRegisterApplicationReturnsGatewayOrNil(t *testing.T) {
	if gateway := RegisterApplication(nil, GatewayConfig{}, nil); gateway != nil {
		t.Fatal("invalid application registration returned a gateway")
	}
	principals := &testPrincipals{values: map[string]Principal{}}
	api := &testAPI{respond: func(http.ResponseWriter, *http.Request) {}}
	config := testGatewayConfig(OAuthConfig{
		ResourceURL: "https://resource.test/mcp", AuthorizationServer: "https://identity.test",
		IntrospectionURL: "https://identity.test/introspect", ClientID: "resource", ClientSecret: "secret",
	}, principals, api, testRoutes{})
	if gateway := RegisterApplication(http.NewServeMux(), config, nil); gateway == nil {
		t.Fatal("valid application registration returned nil")
	}
}
