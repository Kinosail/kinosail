package federation

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"golang.org/x/oauth2"
)

func TestProfilesReturnEverySafeEdgeOutcome(t *testing.T) {
	t.Parallel()
	identity := Identity{Issuer: "issuer", Subject: "directory"}
	values := []linkedProfile{{ID: "viewer", Managed: true, External: "directory"}}
	profiles, _ := linkedProfiles(&values, errors.New("persist failed"))
	if _, found := profiles.FindByID("missing"); found {
		t.Fatal("missing profile was found")
	}
	if _, linked, err := profiles.AutoLinkSCIM(OIDCProtocol, Identity{}); err != nil || linked {
		t.Fatalf("empty identity auto-link = %v %v", linked, err)
	}
	if _, linked, err := profiles.AutoLinkSCIM("LDAP", identity); err == nil || linked {
		t.Fatalf("invalid protocol auto-link = %v %v", linked, err)
	}
	if _, linked, err := profiles.AutoLinkSCIM(OIDCProtocol, identity); err == nil || linked {
		t.Fatalf("failed persistence auto-link = %v %v", linked, err)
	}
	if err := profiles.Unlink(OIDCProtocol, "missing"); err == nil {
		t.Fatal("missing profile unlink was accepted")
	}
}

func TestSettingsExposeEveryFailureWithoutPersistence(t *testing.T) { //nolint:funlen // Each callback failure proves a distinct atomicity boundary.
	t.Parallel()
	sentinel := errors.New("sentinel")
	base := func() Settings {
		return NewSettings("data", &sync.Mutex{}, func(string, string) error { return nil },
			func(string) (map[string]string, map[string]string, error) {
				return map[string]string{}, map[string]string{}, nil
			},
			func(map[string]string, map[string]string) error { return nil },
			func(string, map[string]string, map[string]string, bool, bool) error { return nil })
	}
	settings := base()
	settings.read = func(string) (map[string]string, map[string]string, error) { return nil, nil, sentinel }
	if err := settings.DeleteOIDC(); !errors.Is(err, sentinel) {
		t.Fatalf("read error = %v", err)
	}
	settings = base()
	if err := settings.change(false, "", func(map[string]string, map[string]string) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("update error = %v", err)
	}
	settings = base()
	settings.validateStored = func(map[string]string, map[string]string) error { return sentinel }
	if err := settings.DeleteOIDC(); !errors.Is(err, sentinel) {
		t.Fatalf("stored validation error = %v", err)
	}
	settings = base()
	settings.persist = func(string, map[string]string, map[string]string, bool, bool) error { return sentinel }
	if err := settings.DeleteOIDC(); !errors.Is(err, sentinel) {
		t.Fatalf("persistence error = %v", err)
	}
	settings = base()
	settings.validateValue = func(key, _ string) error {
		if key == SAMLMetadataXMLKey {
			return sentinel
		}
		return nil
	}
	if err := settings.SetSAML(SAMLConfig{MetadataXML: "metadata"}); !errors.Is(err, sentinel) {
		t.Fatalf("SAML value error = %v", err)
	}
}

func TestSettingsPersistInlineSAMLAndCompleteOIDC(t *testing.T) {
	t.Parallel()
	effects := &settingsEffects{}
	settings := testSettings(effects)
	metadata := samlMetadataDocument("https://identity.example", "/sso", samlMetadataCertificate(t))
	if err := settings.SetSAML(SAMLConfig{MetadataXML: metadata}); err != nil {
		t.Fatal(err)
	}
	if effects.regular[SAMLMetadataXMLKey] != metadata || effects.regular[SAMLMetadataURLKey] != "" {
		t.Fatalf("inline SAML = %#v", effects.regular)
	}
	values := map[string]string{ //nolint:gosec // Test-only identity settings intentionally include a placeholder secret.
		OIDCIssuerKey:   "https://identity.example",
		OIDCClientIDKey: "client",
		OIDCSecretKey:   "secret",
		OIDCRedirectKey: "https://media.example/callback",
	}
	if err := ValidateOIDC(func(key string) string { return values[key] }); err != nil {
		t.Fatalf("complete OIDC settings = %v", err)
	}
}

func TestProtocolHelpersRejectRemainingMalformedValues(t *testing.T) {
	t.Parallel()
	for _, query := range []string{
		"code=approved&state=state&iss=one&iss=two",
		"code=approved&state=state&iss=" + strings.Repeat("x", 2049),
		"error=denied&error_uri=&state=state",
	} {
		if _, _, _, valid := parseOIDCCallbackQuery(query); valid {
			t.Fatalf("malformed callback %q was accepted", query)
		}
	}
	if validOAuthParameterName("") {
		t.Fatal("empty OAuth parameter name was accepted")
	}
	if raw, ok := tokenExtra(nil, nil); ok || raw != "" {
		t.Fatalf("nil token extra = %q %v", raw, ok)
	}
	if raw, ok := tokenExtra(&oauth2.Token{}, nil); ok || raw != "" {
		t.Fatalf("missing token extra = %q %v", raw, ok)
	}
	if ValidIdentityURL("") {
		t.Fatal("empty identity URL was accepted")
	}
	if err := validateOIDCProvider(rejectedOIDCMetadata{}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("malformed provider metadata = %v", err)
	}
}

type rejectedOIDCMetadata struct{}

func (rejectedOIDCMetadata) Claims(any) error { return errors.New("malformed metadata") }

func (rejectedOIDCMetadata) Endpoint() oauth2.Endpoint { return oauth2.Endpoint{} }

func TestSAMLMetadataValidationEdges(t *testing.T) {
	t.Parallel()
	if !ValidSAMLMetadata("") {
		t.Fatal("empty optional metadata was rejected")
	}
	for _, raw := range []string{strings.Repeat("x", maxSAMLMetadata+1), string([]byte{0xff}), "<not-metadata/>"} {
		if ValidSAMLMetadata(raw) {
			t.Fatal("invalid SAML metadata was accepted")
		}
	}
	invalidCertificates := []saml.KeyDescriptor{{KeyInfo: saml.KeyInfo{X509Data: saml.X509Data{X509Certificates: []saml.X509Certificate{{Data: "%%%"}, {Data: "bm90LWEtY2VydGlmaWNhdGU="}}}}}}
	if validSAMLCertificates(invalidCertificates) {
		t.Fatal("invalid SAML certificates were accepted")
	}
	if validSAMLEndpoints([]saml.Endpoint{{Binding: saml.HTTPRedirectBinding, Location: "http://identity.example/sso"}}) {
		t.Fatal("untrusted SAML endpoint was accepted")
	}
}

func TestOIDCProviderScopeAndStateCapacity(t *testing.T) {
	flow, _, _ := newOIDCTestFlow(t)
	if _, _, err := flow.Begin(t.Context(), "viewer", "session"); err != nil {
		t.Fatal(err)
	}
	flow.config.IdentityClaim = "email"
	if scopes := flow.oauthConfig(flow.provider).Scopes; len(scopes) != 2 || scopes[1] != "profile" {
		t.Fatalf("OIDC scopes = %#v", scopes)
	}
	flow.pending = make(map[string]oidcTransaction, maxPendingStates)
	for index := range maxPendingStates {
		flow.pending[string(rune(index+1))] = oidcTransaction{expires: farFuture()}
	}
	if _, _, err := flow.Begin(t.Context(), "viewer", "session"); !errors.Is(err, ErrTooManyPending) {
		t.Fatalf("OIDC pending limit = %v", err)
	}
}

func TestOIDCCompletionMapsUnavailableExchangeAndTokenFailures(t *testing.T) { //nolint:funlen // Each failure occurs after state consumption at a distinct provider boundary.
	flow, state := oidcFlowWithPending("")
	if _, err := flow.Complete(t.Context(), "code=approved&state="+state, state, true); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("unconfigured callback provider = %v", err)
	}

	flow, _, setNonce := newOIDCTestFlow(t)
	state, nonce := mustBeginOIDC(t, flow)
	setNonce(nonce)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := flow.Complete(canceled, "code=approved&state="+url.QueryEscape(state), state, true); !errors.Is(err, ErrCodeExchange) {
		t.Fatalf("canceled exchange = %v", err)
	}

	flow, _, setNonce = newOIDCTestFlow(t)
	state, _ = mustBeginOIDC(t, flow)
	setNonce("different")
	if _, err := flow.Complete(t.Context(), "code=approved&state="+url.QueryEscape(state), state, true); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("nonce mismatch = %v", err)
	}

	flow, _, setNonce = newOIDCTestFlow(t)
	flow.config.IdentityClaim = "email"
	location, state, err := flow.Begin(t.Context(), "viewer", "session")
	authorization, parseErr := url.Parse(location)
	if err != nil || parseErr != nil {
		t.Fatalf("custom-claim OIDC begin = %v %v", err, parseErr)
	}
	setNonce(authorization.Query().Get("nonce"))
	if _, err := flow.Complete(t.Context(), "code=approved&state="+url.QueryEscape(state), state, true); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("missing identity claim = %v", err)
	}
}

func farFuture() time.Time {
	return time.Now().Add(time.Hour)
}
