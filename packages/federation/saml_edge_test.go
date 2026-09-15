package federation

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/crewjam/saml"
)

func TestSAMLProviderRefreshRetainsLastValidDocument(t *testing.T) {
	t.Parallel()
	certificate := samlMetadataCertificate(t)
	var document atomic.Value
	var unavailable atomic.Bool
	metadataServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if unavailable.Load() {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = writer.Write([]byte(document.Load().(string)))
	}))
	t.Cleanup(metadataServer.Close)
	document.Store(samlMetadataDocument(metadataServer.URL, "/one", certificate))
	flow := NewSAML(SAMLConfig{MetadataURL: metadataServer.URL, RootURL: "http://localhost", DataDir: t.TempDir()})
	assertSAMLProviderEndpoint(t, flow, metadataServer.URL+"/one")
	document.Store(samlMetadataDocument(metadataServer.URL, "/two", certificate))
	flow.metadataLoadedAt = time.Now().Add(-metadataRefreshInterval)
	assertSAMLProviderEndpoint(t, flow, metadataServer.URL+"/two")
	unavailable.Store(true)
	flow.metadataLoadedAt = time.Now().Add(-metadataRefreshInterval)
	assertSAMLProviderEndpoint(t, flow, metadataServer.URL+"/two")
}

func assertSAMLProviderEndpoint(t *testing.T, flow *SAML, want string) {
	t.Helper()
	provider, err := flow.serviceProvider(t.Context(), true, true)
	_, endpoint, endpointErr := samlProviderEndpoint(provider.IDPMetadata)
	if err != nil || endpointErr != nil {
		t.Fatalf("SAML metadata = %q %v %v", endpoint, err, endpointErr)
	}
	if endpoint != want {
		t.Fatalf("SAML endpoint = %q want %q", endpoint, want)
	}
}

func TestSAMLPendingStoreIsBounded(t *testing.T) {
	t.Parallel()
	metadata := samlMetadataDocument("https://identity.example", "/sso", samlMetadataCertificate(t))
	flow := NewSAML(SAMLConfig{MetadataXML: metadata, RootURL: "http://localhost"})
	for index := range maxPendingStates {
		flow.pending[fmt.Sprintf("request-%d", index)] = samlTransaction{expires: time.Now().Add(time.Minute)}
	}
	if _, err := flow.Begin(t.Context(), "viewer"); !errors.Is(err, ErrTooManyPending) {
		t.Fatalf("SAML pending limit = %v", err)
	}
}

func TestSAMLRedirectPolicy(t *testing.T) {
	t.Parallel()
	client := samlHTTPClient()
	trusted, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://identity.example/metadata", nil)
	untrusted, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://identity.example/metadata", nil)
	if client.Timeout != 10*time.Second {
		t.Fatal("SAML redirect policy is invalid")
	}
	if client.CheckRedirect(trusted, nil) != nil || client.CheckRedirect(untrusted, nil) == nil {
		t.Fatal("SAML redirect trust is invalid")
	}
}

func TestSAMLIdentitySelectionRejectsAmbiguity(t *testing.T) {
	t.Parallel()
	assertion := &saml.Assertion{Subject: &saml.Subject{NameID: &saml.NameID{Value: "subject"}}, AttributeStatements: []saml.AttributeStatement{{Attributes: []saml.Attribute{
		{Name: "objectGUID", Values: []saml.AttributeValue{{Value: "directory"}}},
		{Name: "ambiguous", Values: []saml.AttributeValue{{Value: "one"}, {Value: "two"}}},
	}}}}
	for attribute, want := range map[string]string{"NameID": "subject", "objectGUID": "directory"} {
		value, err := samlIdentityValue(assertion, attribute)
		if err != nil || value != want {
			t.Fatalf("identity %q = %q %v", attribute, value, err)
		}
	}
	for _, attribute := range []string{"missing", "ambiguous"} {
		if _, err := samlIdentityValue(assertion, attribute); !errors.Is(err, ErrTokenInvalid) {
			t.Fatalf("identity %q = %v", attribute, err)
		}
	}
	if _, _, err := samlProviderEndpoint(nil); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("nil metadata endpoint = %v", err)
	}
	assertion.Subject.SubjectConfirmations = []saml.SubjectConfirmation{{}}
	if samlAssertionRequestID(assertion) != "" {
		t.Fatal("empty request ID was accepted")
	}
}
