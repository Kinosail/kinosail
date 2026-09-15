package server_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
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

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

func TestSAMLPublishesMetadataAndStartsProviderLogin(t *testing.T) { //nolint:cyclop,gocognit,funlen // One protocol scenario covers metadata, redirect, POST, and restart behavior.
	t.Parallel()
	metadata := samlIDPMetadata(t, "")
	idp := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/metadata":
			writer.Header().Set("Content-Type", "application/samlmetadata+xml")
			_, _ = writer.Write([]byte(strings.ReplaceAll(metadata, "IDP_ORIGIN", "http://"+request.Host))) //nolint:gosec // The host comes from the local test server.
		case "/sso":
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer idp.Close()

	dataDir := t.TempDir()
	handler := server.New(server.Config{DataDir: dataDir, RequireAuth: true, SAML: server.SAMLConfig{MetadataURL: idp.URL + "/metadata", RootURL: "http://localhost", DataDir: dataDir}})
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil))
	if !strings.Contains(login.Body.String(), `href="/api/v1/session/saml"`) || strings.Contains(login.Body.String(), `href="/api/v1/session/oidc"`) {
		t.Fatalf("SAML login choices = %q", login.Body.String())
	}
	both := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, OIDC: server.OIDCConfig{Issuer: "https://identity.example", ClientID: "kinosail", ClientSecret: "secret", RedirectURL: "http://localhost/login/oidc/callback"}, SAML: server.SAMLConfig{MetadataURL: idp.URL + "/metadata", RootURL: "http://localhost"}})
	bothLogin := httptest.NewRecorder()
	both.ServeHTTP(bothLogin, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil))
	if !strings.Contains(bothLogin.Body.String(), `href="/api/v1/session/saml"`) || !strings.Contains(bothLogin.Body.String(), `href="/api/v1/session/oidc"`) {
		t.Fatalf("federated login choices = %q", bothLogin.Body.String())
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login/saml/metadata", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `entityID="http://localhost/login/saml/metadata"`) || !strings.Contains(response.Body.String(), `Location="http://localhost/login/saml/acs"`) {
		t.Fatalf("SAML metadata = %d %q", response.Code, response.Body.String())
	}
	restarted := server.New(server.Config{DataDir: dataDir, RequireAuth: true, SAML: server.SAMLConfig{RootURL: "http://localhost", DataDir: dataDir}})
	restartedMetadata := httptest.NewRecorder()
	restarted.ServeHTTP(restartedMetadata, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login/saml/metadata", nil))
	firstDocument, firstErr := samlsp.ParseMetadata(response.Body.Bytes())
	restartedDocument, restartedErr := samlsp.ParseMetadata(restartedMetadata.Body.Bytes())
	firstKey := firstDocument.SPSSODescriptors[0].KeyDescriptors[0].KeyInfo.X509Data.X509Certificates[0].Data
	restartedKey := restartedDocument.SPSSODescriptors[0].KeyDescriptors[0].KeyInfo.X509Data.X509Certificates[0].Data
	if restartedMetadata.Code != http.StatusOK || firstErr != nil || restartedErr != nil || firstKey != restartedKey {
		t.Fatalf("stable SAML metadata after restart = %d firstErr=%v restartedErr=%v equalKey=%v", restartedMetadata.Code, firstErr, restartedErr, firstKey == restartedKey)
	}
	if info, err := os.Stat(filepath.Join(dataDir, "saml_sp.pem")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("SAML key permissions = %v err=%v", info, err)
	}

	start := httptest.NewRecorder()
	handler.ServeHTTP(start, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/session/saml", nil))
	location, err := url.Parse(start.Header().Get("Location"))
	if err != nil || start.Code != http.StatusSeeOther || location.Host != strings.TrimPrefix(idp.URL, "http://") || location.Path != "/sso" || location.Query().Get("SAMLRequest") == "" {
		t.Fatalf("SAML start = %d location=%q err=%v", start.Code, start.Header().Get("Location"), err)
	}
	metadataRequest, err := http.NewRequestWithContext(t.Context(), http.MethodGet, idp.URL+"/metadata", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadataResponse, err := idp.Client().Do(metadataRequest)
	if err != nil {
		t.Fatal(err)
	}
	metadataXML, _ := io.ReadAll(metadataResponse.Body)
	_ = metadataResponse.Body.Close()
	xmlHandler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, SAML: server.SAMLConfig{MetadataXML: string(metadataXML), RootURL: "http://localhost"}})
	xmlStart := httptest.NewRecorder()
	xmlHandler.ServeHTTP(xmlStart, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/session/saml", nil))
	if xmlStart.Code != http.StatusSeeOther || !strings.Contains(xmlStart.Header().Get("Location"), "/sso?") {
		t.Fatalf("downloaded SAML metadata start = %d location=%q", xmlStart.Code, xmlStart.Header().Get("Location"))
	}
	postMetadata := strings.ReplaceAll(string(metadataXML), saml.HTTPRedirectBinding, saml.HTTPPostBinding)
	postHandler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, SAML: server.SAMLConfig{MetadataXML: postMetadata, RootURL: "http://localhost"}})
	postStart := httptest.NewRecorder()
	postHandler.ServeHTTP(postStart, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/session/saml", nil))
	if postStart.Code != http.StatusOK || !strings.Contains(postStart.Body.String(), `method="post"`) || !strings.Contains(postStart.Body.String(), `name="SAMLRequest"`) || !strings.Contains(postStart.Header().Get("Content-Security-Policy"), "form-action "+idp.URL+"/sso") {
		t.Fatalf("SAML POST start = %d csp=%q body=%q", postStart.Code, postStart.Header().Get("Content-Security-Policy"), postStart.Body.String())
	}
}

func TestSAMLValidatesSignedResponseAndLinksProvisionedSubject(t *testing.T) { //nolint:cyclop,funlen // One protocol scenario covers signed response validation and provisioning.
	t.Parallel()
	key, certificate := samlTestKeyPair(t)
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
	defer idpServer.Close()
	idpOrigin, _ := url.Parse(idpServer.URL)
	idp.MetadataURL, idp.SSOURL, idp.LoginURL = *idpOrigin.ResolveReference(&url.URL{Path: "/metadata"}), *idpOrigin.ResolveReference(&url.URL{Path: "/sso"}), *idpOrigin.ResolveReference(&url.URL{Path: "/login"})

	dataDir := t.TempDir()
	handler := server.New(server.Config{DataDir: dataDir, RequireAuth: true, SAML: server.SAMLConfig{MetadataURL: idp.MetadataURL.String(), RootURL: "http://localhost", IdentityAttribute: "objectGUID"}, SCIM: server.SCIMConfig{Token: scimTestToken, TokenExpiresAt: time.Now().Add(time.Hour)}})
	setup := httptest.NewRecorder()
	handler.ServeHTTP(setup, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/setup", strings.NewReader(`{"name":"Owner","password":"owner-password"}`)))
	if setup.Code != http.StatusCreated {
		t.Fatalf("SAML setup = %d %q", setup.Code, setup.Body.String())
	}
	created := scimCall(t, handler, scimTestToken, http.MethodPost, "/scim/v2/Users", map[string]any{"schemas": []string{scimUserSchema}, "userName": "saml@example.com", "externalId": "directory-1"})
	if created.Code != http.StatusCreated {
		t.Fatalf("SAML SCIM fixture = %d %q", created.Code, created.Body.String())
	}
	metadata := httptest.NewRecorder()
	handler.ServeHTTP(metadata, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login/saml/metadata", nil))
	var err error
	provider.metadata, err = samlsp.ParseMetadata(metadata.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	start := httptest.NewRecorder()
	handler.ServeHTTP(start, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/session/saml", nil))
	idpRequest, err := http.NewRequestWithContext(t.Context(), http.MethodGet, start.Header().Get("Location"), nil)
	if err != nil {
		t.Fatal(err)
	}
	idpResponse, err := idpServer.Client().Do(idpRequest)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, _ := io.ReadAll(idpResponse.Body)
	_ = idpResponse.Body.Close()
	response := regexp.MustCompile(`name="SAMLResponse" value="([^"]+)"`).FindSubmatch(responseBody)
	relay := regexp.MustCompile(`name="RelayState" value="([^"]*)"`).FindSubmatch(responseBody)
	if len(response) != 2 || len(relay) != 2 {
		t.Fatalf("IdP response form = %q", responseBody)
	}
	form := url.Values{"SAMLResponse": {html.UnescapeString(string(response[1]))}, "RelayState": {html.UnescapeString(string(relay[1]))}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login/saml/acs", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	callback := httptest.NewRecorder()
	handler.ServeHTTP(callback, request)
	if callback.Code != http.StatusSeeOther || callback.Header().Get("Location") != "/" || len(callback.Result().Cookies()) == 0 || !strings.Contains(string(storedState(t, dataDir, "profiles.json")), `"samlSubject":"directory-1"`) {
		t.Fatalf("SAML callback = %d location=%q cookies=%v body=%q", callback.Code, callback.Header().Get("Location"), callback.Result().Cookies(), callback.Body.String())
	}
}

func TestSAMLRejectsAmbiguousCallbackBeforeProviderWork(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, SAML: server.SAMLConfig{MetadataURL: "https://identity.example/metadata", RootURL: "http://localhost"}})
	for name, request := range map[string]*http.Request{
		"wrong media type": httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login/saml/acs", strings.NewReader("SAMLResponse=value")),
		"duplicate response": func() *http.Request {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login/saml/acs", strings.NewReader("SAMLResponse=one&SAMLResponse=two"))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			return request
		}(),
		"query parameters": func() *http.Request {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login/saml/acs?unexpected=true", strings.NewReader("SAMLResponse=value"))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			return request
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("ambiguous callback = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

type samlTestServiceProvider struct{ metadata *saml.EntityDescriptor }

func (provider *samlTestServiceProvider) GetServiceProvider(_ *http.Request, _ string) (*saml.EntityDescriptor, error) {
	return provider.metadata, nil
}

type samlTestSessionProvider struct{}

func (samlTestSessionProvider) GetSession(_ http.ResponseWriter, _ *http.Request, _ *saml.IdpAuthnRequest) *saml.Session {
	return &saml.Session{ID: "session-1", CreateTime: time.Now(), ExpireTime: time.Now().Add(time.Hour), NameID: "subject-1", NameIDFormat: string(saml.PersistentNameIDFormat), UserName: "saml@example.com", CustomAttributes: []saml.Attribute{{Name: "objectGUID", Values: []saml.AttributeValue{{Type: "xs:string", Value: "directory-1"}}}}}
}

func samlIDPMetadata(t *testing.T, entityID string) string {
	t.Helper()
	_, certificate := samlTestKeyPair(t)
	if entityID == "" {
		entityID = "IDP_ORIGIN/metadata"
	}
	return fmt.Sprintf(`<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="%s"><IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><KeyDescriptor use="signing"><KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><X509Data><X509Certificate>%s</X509Certificate></X509Data></KeyInfo></KeyDescriptor><SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="IDP_ORIGIN/sso"/></IDPSSODescriptor></EntityDescriptor>`, entityID, base64.StdEncoding.EncodeToString(certificate.Raw))
}

func samlTestKeyPair(t *testing.T) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test Identity Provider"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
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
