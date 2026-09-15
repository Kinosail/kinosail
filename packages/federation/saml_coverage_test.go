package federation

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

func TestSAMLCoreRejectsProviderCallbackRootAndKeyEdges(t *testing.T) { //nolint:cyclop,funlen // Distinct trust-boundary failures remain side-effect free.
	certificate := samlMetadataCertificate(t)
	metadataXML := samlMetadataDocument("https://identity.example", "/sso", certificate)
	flow := NewSAML(SAMLConfig{MetadataXML: metadataXML, RootURL: "http://localhost", DataDir: t.TempDir()})
	flow.sp = &saml.ServiceProvider{IDPMetadata: &saml.EntityDescriptor{}}
	if _, err := flow.Begin(t.Context(), "viewer"); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("invalid provider endpoint = %v", err)
	}

	request := samlRequest(t, "/login/saml/acs", "SAMLResponse=value&RelayState=relay")
	if _, err := NewSAML(SAMLConfig{}).Complete(httptest.NewRecorder(), request); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("unavailable callback provider = %v", err)
	}
	flow = NewSAML(SAMLConfig{MetadataXML: metadataXML, RootURL: "http://localhost", DataDir: t.TempDir()})
	flow.pending["request"] = samlTransaction{expires: time.Now().Add(time.Minute)}
	request = samlRequest(t, "/login/saml/acs", "SAMLResponse=value&RelayState=relay")
	if _, err := flow.Complete(httptest.NewRecorder(), request); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("invalid signed response = %v", err)
	}
	if NewSAML(SAMLConfig{RootURL: "https://media.example/" + strings.Repeat("x", 2049)}).rootConfigured() {
		t.Fatal("oversized SAML root was accepted")
	}

	dataFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(dataFile, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSAML(SAMLConfig{RootURL: "http://localhost", DataDir: dataFile}).Metadata(t.Context()); err == nil {
		t.Fatal("regular file data directory was accepted")
	}
	assertion := &saml.Assertion{Subject: &saml.Subject{NameID: &saml.NameID{Value: "bad\nsubject"}}}
	if _, err := samlIdentityValue(assertion, "NameID"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("invalid NameID = %v", err)
	}
	flow.pending["expired"] = samlTransaction{expires: time.Now().Add(-time.Minute)}
	if _, found := flow.takeTransaction("expired"); found {
		t.Fatal("expired SAML transaction was accepted")
	}
	if _, err := samlCallbackResult(&saml.Assertion{}, "subject", samlTransaction{}, false); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("missing SAML transaction = %v", err)
	}
	if _, err := marshalSAMLMetadataWith(&saml.EntityDescriptor{}, func(*saml.EntityDescriptor) ([]byte, error) { return nil, errors.New("marshal failed") }); err == nil {
		t.Fatal("invalid XML metadata was marshaled")
	}
}

func TestSAMLKeyLoaderRejectsMalformedAndNonRSAKeys(t *testing.T) {
	malformed := t.TempDir()
	if err := os.WriteFile(filepath.Join(malformed, "saml_sp.pem"), []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadOrCreateSAMLKeyPair(malformed); err == nil {
		t.Fatal("malformed SAML key was accepted")
	}

	nonRSA := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "EC"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	privateDER, privateErr := x509.MarshalECPrivateKey(key)
	if err != nil || privateErr != nil {
		t.Fatalf("create EC fixture = %v %v", err, privateErr)
	}
	data := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateDER})...)
	if err := os.WriteFile(filepath.Join(nonRSA, "saml_sp.pem"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadOrCreateSAMLKeyPair(nonRSA); err == nil {
		t.Fatal("non-RSA SAML key was accepted")
	}
}

func TestSAMLKeyGenerationAndPersistenceErrors(t *testing.T) { //nolint:cyclop // One matrix covers independent cryptographic failure seams.
	sentinel := errors.New("sentinel")
	generateError := func() (*rsa.PrivateKey, *x509.Certificate, error) { return nil, nil, sentinel }
	if _, _, err := loadOrCreateSAMLKeyPairWith(t.TempDir(), generateError, func(string, []byte) error { return nil }); !errors.Is(err, sentinel) {
		t.Fatalf("key generation error = %v", err)
	}
	key, certificate, err := generateSAMLKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if !certificate.NotBefore.Before(time.Now()) || certificate.NotAfter.Before(time.Now().AddDate(4, 11, 0)) {
		t.Fatalf("certificate validity = %v through %v", certificate.NotBefore, certificate.NotAfter)
	}
	wantUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	if certificate.KeyUsage != wantUsage || len(certificate.ExtKeyUsage) != 1 || certificate.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth {
		t.Fatalf("certificate usage = %v %#v", certificate.KeyUsage, certificate.ExtKeyUsage)
	}
	generate := func() (*rsa.PrivateKey, *x509.Certificate, error) { return key, certificate, nil }
	if _, _, err := loadOrCreateSAMLKeyPairWith(t.TempDir(), generate, func(string, []byte) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("key persistence error = %v", err)
	}
	failedGenerate := func(io.Reader, int) (*rsa.PrivateKey, error) { return nil, sentinel }
	if _, _, err := generateSAMLKeyPairWith(errorReader{sentinel}, failedGenerate, createSAMLCertificate); !errors.Is(err, sentinel) {
		t.Fatalf("RSA generation error = %v", err)
	}
	useKey := func(io.Reader, int) (*rsa.PrivateKey, error) { return key, nil }
	failedCreate := func(io.Reader, *x509.Certificate, *rsa.PrivateKey) ([]byte, error) { return nil, sentinel }
	if _, _, err := generateSAMLKeyPairWith(errorReader{sentinel}, useKey, failedCreate); !errors.Is(err, sentinel) {
		t.Fatalf("certificate generation error = %v", err)
	}
}

type errorReader struct{ err error }

func (reader errorReader) Read([]byte) (int, error) { return 0, reader.err }

func TestSAMLBeginMapsAuthenticationAndRedirectErrors(t *testing.T) {
	certificate := samlMetadataCertificate(t)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	for name, binding := range map[string]string{"post": saml.HTTPPostBinding, "redirect": saml.HTTPRedirectBinding} {
		t.Run(name, func(t *testing.T) {
			metadataXML := strings.Replace(samlMetadataDocument("https://identity.example", "/sso", certificate), saml.HTTPRedirectBinding, binding, 1)
			metadata, err := samlsp.ParseMetadata([]byte(metadataXML))
			if err != nil {
				t.Fatal(err)
			}
			flow := NewSAML(SAMLConfig{MetadataXML: metadataXML, RootURL: "http://localhost", DataDir: t.TempDir()})
			flow.sp = &saml.ServiceProvider{IDPMetadata: metadata, Key: key, Certificate: &x509.Certificate{}, SignatureMethod: "invalid"}
			if _, err := flow.Begin(t.Context(), "viewer"); !errors.Is(err, ErrProviderUnavailable) {
				t.Fatalf("%s authentication error = %v", name, err)
			}
		})
	}
}
