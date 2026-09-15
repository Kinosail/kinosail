package configuration_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/configuration"
)

func TestSAMLMetadataConfigurationRequiresTrustedBoundedURL(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		value string
		valid bool
	}{
		"disabled":          {"", true},
		"HTTPS metadata":    {"https://identity.example/app/metadata", true},
		"local development": {"http://localhost:8080/metadata", true},
		"insecure remote":   {"http://identity.example/metadata", false},
		"credentials":       {"https://owner@identity.example/metadata", false},
		"fragment":          {"https://identity.example/metadata#idp", false},
		"oversized":         {"https://identity.example/" + strings.Repeat("x", 2049), false},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := configuration.Load(t.TempDir(), "", lookup(map[string]string{"KINOSAIL_SAML_METADATA_URL": test.value}))
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v err=%v", test.valid, err)
			}
		})
	}
}

func TestSAMLConfigurationAcceptsOneURLOrDownloadedMetadataDocument(t *testing.T) {
	t.Parallel()
	metadata := testSAMLMetadata(t)
	for name, values := range map[string]map[string]string{
		"metadata URL":      {"KINOSAIL_SAML_METADATA_URL": "https://identity.example/metadata"},
		"metadata XML":      {"KINOSAIL_SAML_METADATA_XML": metadata},
		"POST metadata XML": {"KINOSAIL_SAML_METADATA_XML": strings.ReplaceAll(metadata, "HTTP-Redirect", "HTTP-POST")},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := configuration.Load(t.TempDir(), "", lookup(values)); err != nil {
				t.Fatalf("valid SAML configuration: %v", err)
			}
		})
	}
	for name, values := range map[string]map[string]string{
		"both sources":        {"KINOSAIL_SAML_METADATA_URL": "https://identity.example/metadata", "KINOSAIL_SAML_METADATA_XML": metadata},
		"malformed XML":       {"KINOSAIL_SAML_METADATA_XML": "<EntityDescriptor>"},
		"missing certificate": {"KINOSAIL_SAML_METADATA_XML": `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="id"><IDPSSODescriptor><SingleSignOnService Location="https://identity.example/sso"/></IDPSSODescriptor></EntityDescriptor>`},
		"insecure sign-in":    {"KINOSAIL_SAML_METADATA_XML": strings.ReplaceAll(metadata, "https://identity.example/sso", "http://identity.example/sso")},
		"unsupported binding": {"KINOSAIL_SAML_METADATA_XML": strings.ReplaceAll(metadata, "HTTP-Redirect", "HTTP-Artifact")},
		"spaced identity":     {"KINOSAIL_SAML_METADATA_URL": "https://identity.example/metadata", "KINOSAIL_SAML_IDENTITY_ATTRIBUTE": "object id"},
		"oversized identity":  {"KINOSAIL_SAML_METADATA_URL": "https://identity.example/metadata", "KINOSAIL_SAML_IDENTITY_ATTRIBUTE": strings.Repeat("x", 257)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := configuration.Load(t.TempDir(), "", lookup(values)); err == nil {
				t.Fatal("invalid SAML configuration was accepted")
			}
		})
	}
}

func TestStoredSAMLConfigurationRejectsConflictingSources(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	data, err := json.Marshal(map[string]string{
		"integrations.saml.metadata_url": "https://identity.example/metadata",
		"integrations.saml.metadata_xml": testSAMLMetadata(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "configuration.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := configuration.Set(directory, "server.name", "Kinosail"); err == nil {
		t.Fatal("conflicting stored SAML sources were accepted")
	}
}

func TestSAMLConfigurationChangesMetadataSourceAtomically(t *testing.T) { //nolint:cyclop // One configuration scenario covers valid and conflicting sources.
	t.Parallel()
	directory := t.TempDir()
	metadataURL, metadataXML := "https://identity.example/metadata", testSAMLMetadata(t)
	if err := configuration.SetSAML(directory, metadataURL, "", "objectGUID"); err != nil {
		t.Fatal(err)
	}
	if err := configuration.SetSAML(directory, "", metadataXML, "objectGUID"); err != nil {
		t.Fatal(err)
	}
	loaded, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("integrations.saml.metadata_url") != "" || loaded.String("integrations.saml.metadata_xml") != metadataXML || loaded.String("integrations.saml.identity_attribute") != "objectGUID" {
		t.Fatalf("switched SAML metadata source: url=%q xml=%d err=%v", loaded.String("integrations.saml.metadata_url"), len(loaded.String("integrations.saml.metadata_xml")), err)
	}
	if err := configuration.SetSAML(directory, metadataURL, metadataXML, "objectGUID"); err == nil {
		t.Fatal("conflicting SAML metadata sources were accepted")
	}
	if err := configuration.SetSAML(directory, metadataURL, "", "object id"); err == nil {
		t.Fatal("invalid SAML identity attribute was accepted")
	}
	loaded, err = configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("integrations.saml.metadata_xml") != metadataXML {
		t.Fatal("rejected SAML change modified persisted state")
	}
	if err := configuration.DeleteSAML(directory); err != nil {
		t.Fatal(err)
	}
	loaded, err = configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("integrations.saml.metadata_url") != "" || loaded.String("integrations.saml.metadata_xml") != "" || loaded.String("integrations.saml.identity_attribute") != "NameID" {
		t.Fatalf("deleted SAML configuration remained: %#v err=%v", loaded.Fields(), err)
	}
}

func testSAMLMetadata(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://identity.example"><IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><KeyDescriptor use="signing"><KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><X509Data><X509Certificate>` + base64.StdEncoding.EncodeToString(der) + `</X509Certificate></X509Data></KeyInfo></KeyDescriptor><SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://identity.example/sso"/></IDPSSODescriptor></EntityDescriptor>`
}
