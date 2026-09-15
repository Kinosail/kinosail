package federation

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

func TestSAMLConfigurationRejectsInvalidSourcesBeforeKeyCreation(t *testing.T) {
	t.Parallel()
	validXML := samlMetadataDocument("https://identity.example", "/sso", samlMetadataCertificate(t))
	for name, config := range map[string]SAMLConfig{
		"valid URL":       {MetadataURL: "https://identity.example/metadata", RootURL: "http://localhost"},
		"valid XML":       {MetadataXML: validXML, RootURL: "https://media.example"},
		"both sources":    {MetadataURL: "https://identity.example/metadata", MetadataXML: validXML, RootURL: "http://localhost"},
		"remote HTTP":     {MetadataURL: "http://identity.example/metadata", RootURL: "http://localhost"},
		"root path":       {MetadataXML: validXML, RootURL: "https://media.example/path"},
		"large attribute": {MetadataXML: validXML, RootURL: "http://localhost", IdentityAttribute: strings.Repeat("x", 257)},
		"large XML":       {MetadataXML: strings.Repeat("x", maxSAMLMetadata+1), RootURL: "http://localhost"},
		"malformed XML":   {MetadataXML: "<EntityDescriptor>", RootURL: "http://localhost"},
		"untrusted XML":   {MetadataXML: strings.Replace(validXML, "https://identity.example/sso", "http://identity.example/sso", 1), RootURL: "http://localhost"},
	} {
		want := strings.HasPrefix(name, "valid")
		t.Run(name, func(t *testing.T) {
			assertSAMLConfiguration(t, config, want)
		})
	}
	if (*SAML)(nil).Configured() {
		t.Fatal("nil SAML flow was configured")
	}
}

func TestSAMLRejectsInvalidExternalMetadataBeforeKeyCreation(t *testing.T) {
	t.Parallel()
	metadataServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("<EntityDescriptor>"))
	}))
	t.Cleanup(metadataServer.Close)
	directory := t.TempDir()
	flow := NewSAML(SAMLConfig{MetadataURL: metadataServer.URL, RootURL: "http://localhost", DataDir: directory})
	if _, err := flow.Begin(t.Context(), "viewer"); err == nil {
		t.Fatal("invalid external metadata was accepted")
	}
	if _, err := os.Stat(filepath.Join(directory, "saml_sp.pem")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid external metadata created key: %v", err)
	}
}

func TestSAMLRejectsInvalidDataDirectoryBeforeKeyCreation(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	flow := NewSAML(SAMLConfig{MetadataURL: "https://identity.example/metadata", RootURL: "http://localhost", DataDir: directory + "\x00bad"})
	if flow.Configured() {
		t.Fatal("invalid data directory was accepted")
	}
	if _, err := flow.Metadata(t.Context()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("metadata with invalid data directory = %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "saml_sp.pem")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid data directory created key: %v", err)
	}
}

func assertSAMLConfiguration(t *testing.T, config SAMLConfig, want bool) {
	t.Helper()
	directory := t.TempDir()
	config.DataDir = directory
	flow := NewSAML(config)
	if flow.Configured() != want {
		t.Fatalf("Configured() = %v", !want)
	}
	if want {
		return
	}
	if _, err := flow.Begin(t.Context(), ""); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("invalid begin = %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "saml_sp.pem")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid configuration created key: %v", err)
	}
}

func TestSAMLMetadataKeyIsStableAndPrivate(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	config := SAMLConfig{RootURL: "http://localhost", DataDir: directory}
	first, err := NewSAML(config).Metadata(t.Context())
	second, secondErr := NewSAML(config).Metadata(t.Context())
	firstDocument, firstParseErr := samlsp.ParseMetadata(first)
	secondDocument, secondParseErr := samlsp.ParseMetadata(second)
	firstCertificate := firstDocument.SPSSODescriptors[0].KeyDescriptors[0].KeyInfo.X509Data.X509Certificates[0].Data
	secondCertificate := secondDocument.SPSSODescriptors[0].KeyDescriptors[0].KeyInfo.X509Data.X509Certificates[0].Data
	info, statErr := os.Stat(filepath.Join(directory, "saml_sp.pem"))
	if err != nil || secondErr != nil || firstParseErr != nil {
		t.Fatalf("stable private key errors = %v %v %v %v %v", err, secondErr, firstParseErr, secondParseErr, statErr)
	}
	if secondParseErr != nil || statErr != nil {
		t.Fatalf("reloaded private key errors = %v %v", secondParseErr, statErr)
	}
	if info.Mode().Perm() != 0o600 || firstCertificate != secondCertificate {
		t.Fatalf("private key mode=%v stable=%v", info.Mode().Perm(), firstCertificate == secondCertificate)
	}
}

func TestSAMLRejectsUnsafeKeyFiles(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if _, _, err := loadOrCreateSAMLKeyPair(directory); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "saml_sp.pem")
	if err := os.Chmod(path, 0o644); err != nil { //nolint:gosec // This test creates an unsafe key file.
		t.Fatal(err)
	}
	if _, _, err := loadOrCreateSAMLKeyPair(directory); err == nil {
		t.Fatal("public SAML key was accepted")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "target.pem")
	if err := os.Rename(path, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadOrCreateSAMLKeyPair(directory); err == nil {
		t.Fatal("symlinked SAML key was accepted")
	}
}

func TestSAMLFlowStartsAndCompletesSignedResponse(t *testing.T) { //nolint:cyclop,funlen // One lifecycle proves provider, key, state, and assertion behavior.
	t.Parallel()
	key, certificate := samlKeyPair(t)
	provider := &samlTestServiceProvider{}
	idp := &saml.IdentityProvider{Key: key, Signer: key, Certificate: certificate, ServiceProviderProvider: provider, SessionProvider: samlTestSessionProvider{}}
	idpServer := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/metadata":
			idp.ServeMetadata(writer, request)
		case "/sso":
			idp.ServeSSO(writer, request)
		default:
			http.NotFound(writer, request)
		}
	}))
	idpServer.Start()
	t.Cleanup(idpServer.Close)
	idpOrigin, _ := url.Parse(idpServer.URL)
	idp.MetadataURL = *idpOrigin.ResolveReference(&url.URL{Path: "/metadata"})
	idp.SSOURL = *idpOrigin.ResolveReference(&url.URL{Path: "/sso"})
	idp.LoginURL = *idpOrigin.ResolveReference(&url.URL{Path: "/login"})
	flow := NewSAML(SAMLConfig{MetadataURL: idp.MetadataURL.String(), RootURL: "http://localhost", DataDir: t.TempDir(), IdentityAttribute: "objectGUID"})
	metadata, err := flow.Metadata(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	provider.metadata, err = samlsp.ParseMetadata(metadata)
	if err != nil {
		t.Fatal(err)
	}
	flow.sp.HTTPClient = idpServer.Client()
	start, err := flow.Begin(t.Context(), "viewer")
	if err != nil || start.RedirectURL == "" {
		t.Fatalf("SAML begin = %#v %v", start, err)
	}
	idpRequest, err := http.NewRequestWithContext(t.Context(), http.MethodGet, start.RedirectURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	idpResponse, err := idpServer.Client().Do(idpRequest)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(idpResponse.Body)
	_ = idpResponse.Body.Close()
	response := regexp.MustCompile(`name="SAMLResponse" value="([^"]+)"`).FindSubmatch(body)
	relay := regexp.MustCompile(`name="RelayState" value="([^"]*)"`).FindSubmatch(body)
	if len(response) != 2 || len(relay) != 2 {
		t.Fatalf("identity provider response = %q", body)
	}
	form := url.Values{"SAMLResponse": {html.UnescapeString(string(response[1]))}, "RelayState": {html.UnescapeString(string(relay[1]))}}
	flow.config.IdentityAttribute = "missing"
	invalidIdentity := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login/saml/acs", strings.NewReader(form.Encode()))
	invalidIdentity.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := flow.Complete(httptest.NewRecorder(), invalidIdentity); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("missing signed identity = %v", err)
	}
	flow.config.IdentityAttribute = "objectGUID"
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login/saml/acs", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	values := []webProfile{{ID: "viewer"}}
	effects := &webEffects{}
	login := &SAMLHTTP[webProfile]{flow: flow, profiles: webProfiles(&values), hooks: webHooks(effects)}
	callback := httptest.NewRecorder()
	login.callback(callback, request)
	if callback.Code != http.StatusSeeOther || values[0].SAML.Subject != "directory-1" {
		t.Fatalf("SAML complete = %d %#v", callback.Code, values[0].SAML)
	}
	replay := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login/saml/acs", strings.NewReader(form.Encode()))
	replay.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err = flow.Complete(httptest.NewRecorder(), replay); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("SAML replay = %v", err)
	}
}

func TestSAMLCallbackRejectsInputBeforeProviderEffects(t *testing.T) {
	t.Parallel()
	for name, request := range map[string]*http.Request{
		"wrong type":    httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login/saml/acs", strings.NewReader("SAMLResponse=value&RelayState=relay")),
		"query":         samlRequest(t, "/login/saml/acs?unexpected=true", "SAMLResponse=value&RelayState=relay"),
		"duplicate":     samlRequest(t, "/login/saml/acs", "SAMLResponse=one&SAMLResponse=two&RelayState=relay"),
		"missing relay": samlRequest(t, "/login/saml/acs", "SAMLResponse=value"),
		"unknown":       samlRequest(t, "/login/saml/acs", "SAMLResponse=value&RelayState=relay&extra=true"),
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			flow := NewSAML(SAMLConfig{MetadataURL: "https://127.0.0.1:1/metadata", RootURL: "http://localhost", DataDir: directory})
			if _, err := flow.Complete(httptest.NewRecorder(), request); !errors.Is(err, ErrInvalidCallback) {
				t.Fatalf("invalid callback = %v", err)
			}
			if _, err := os.Stat(filepath.Join(directory, "saml_sp.pem")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid callback created key: %v", err)
			}
		})
	}
}

func samlRequest(t *testing.T, path, body string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func samlMetadataCertificate(t *testing.T) string {
	t.Helper()
	_, certificate := samlKeyPair(t)
	return base64.StdEncoding.EncodeToString(certificate.Raw)
}

func samlMetadataDocument(origin, path, certificate string) string {
	return fmt.Sprintf(`<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="%s"><IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><KeyDescriptor use="signing"><KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><X509Data><X509Certificate>%s</X509Certificate></X509Data></KeyInfo></KeyDescriptor><SingleSignOnService Binding="%s" Location="%s%s"/></IDPSSODescriptor></EntityDescriptor>`, origin, certificate, saml.HTTPRedirectBinding, origin, path)
}

func samlKeyPair(t *testing.T) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Identity Provider"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return key, certificate
}
