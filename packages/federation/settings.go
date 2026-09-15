package federation

import (
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

const (
	OIDCIssuerKey      = "integrations.oidc.issuer"
	OIDCClientIDKey    = "integrations.oidc.client_id"
	OIDCSecretKey      = "integrations.oidc.client_secret"
	OIDCRedirectKey    = "integrations.oidc.redirect_url"
	OIDCIdentityKey    = "integrations.oidc.identity_claim"
	SAMLMetadataURLKey = "integrations.saml.metadata_url"
	SAMLMetadataXMLKey = "integrations.saml.metadata_xml"
	SAMLIdentityKey    = "integrations.saml.identity_attribute"
)

var (
	oidcRequiredKeys = []string{OIDCIssuerKey, OIDCClientIDKey, OIDCSecretKey, OIDCRedirectKey}
	oidcSettingKeys  = append(append([]string(nil), oidcRequiredKeys...), OIDCIdentityKey)
	samlSettingKeys  = []string{SAMLMetadataURLKey, SAMLMetadataXMLKey, SAMLIdentityKey}
)

// Settings owns atomic persistence mechanics for coupled OIDC and SAML configuration.
type Settings struct {
	dataDir        string
	lock           sync.Locker
	validateValue  func(string, string) error
	read           func(string) (map[string]string, map[string]string, error)
	validateStored func(map[string]string, map[string]string) error
	persist        func(string, map[string]string, map[string]string, bool, bool) error
}

// NewSettings adapts one app's configuration storage to shared federation mutations.
func NewSettings(dataDir string, lock sync.Locker, validateValue func(string, string) error, read func(string) (map[string]string, map[string]string, error), validateStored func(map[string]string, map[string]string) error, persist func(string, map[string]string, map[string]string, bool, bool) error) Settings {
	return Settings{dataDir: dataDir, lock: lock, validateValue: validateValue, read: read, validateStored: validateStored, persist: persist}
}

// SetOIDC changes the complete OpenID Connect client as one validated operation.
func (settings Settings) SetOIDC(config OIDCConfig) error {
	if config.IdentityClaim == "" {
		config.IdentityClaim = "sub"
	}
	values := map[string]string{OIDCIssuerKey: config.Issuer, OIDCClientIDKey: config.ClientID, OIDCSecretKey: config.ClientSecret, OIDCRedirectKey: config.RedirectURL, OIDCIdentityKey: config.IdentityClaim}
	for key, value := range values {
		if err := settings.validateValue(key, value); err != nil {
			return err
		}
	}
	return settings.change(true, "", func(regular, secrets map[string]string) error {
		regular[OIDCIssuerKey], regular[OIDCClientIDKey] = config.Issuer, config.ClientID
		regular[OIDCRedirectKey], regular[OIDCIdentityKey] = config.RedirectURL, config.IdentityClaim
		secrets[OIDCSecretKey] = config.ClientSecret
		return nil
	})
}

// DeleteOIDC removes the complete OpenID Connect client as one operation.
func (settings Settings) DeleteOIDC() error {
	return settings.change(true, "", func(regular, secrets map[string]string) error {
		for _, key := range oidcSettingKeys {
			delete(regular, key)
			delete(secrets, key)
		}
		return nil
	})
}

// SetSAML changes the provider metadata source as one validated operation.
func (settings Settings) SetSAML(config SAMLConfig) error {
	if config.IdentityAttribute == "" {
		config.IdentityAttribute = "NameID"
	}
	values := map[string]string{SAMLMetadataURLKey: config.MetadataURL, SAMLMetadataXMLKey: config.MetadataXML, SAMLIdentityKey: config.IdentityAttribute}
	for key, value := range values {
		if err := settings.validateValue(key, value); err != nil {
			return err
		}
	}
	if config.MetadataURL == "" && config.MetadataXML == "" || config.MetadataURL != "" && config.MetadataXML != "" {
		return errors.New("configure one SAML metadata URL or XML document")
	}
	return settings.change(false, "", func(regular, _ map[string]string) error {
		delete(regular, SAMLMetadataURLKey)
		delete(regular, SAMLMetadataXMLKey)
		if config.MetadataURL != "" {
			regular[SAMLMetadataURLKey] = config.MetadataURL
		} else {
			regular[SAMLMetadataXMLKey] = config.MetadataXML
		}
		regular[SAMLIdentityKey] = config.IdentityAttribute
		return nil
	})
}

// DeleteSAML removes the provider metadata source as one operation.
func (settings Settings) DeleteSAML() error {
	return settings.change(false, "save SAML configuration", func(regular, _ map[string]string) error {
		for _, key := range samlSettingKeys {
			delete(regular, key)
		}
		return nil
	})
}

func (settings Settings) change(writeSecrets bool, persistContext string, update func(map[string]string, map[string]string) error) error {
	settings.lock.Lock()
	defer settings.lock.Unlock()
	regular, secrets, err := settings.read(settings.dataDir)
	if err != nil {
		return err
	}
	if err = update(regular, secrets); err != nil {
		return err
	}
	if err = settings.validateStored(regular, secrets); err != nil {
		return err
	}
	err = settings.persist(settings.dataDir, regular, secrets, true, writeSecrets)
	if err != nil && persistContext != "" {
		return fmt.Errorf("%s: %w", persistContext, err)
	}
	return err
}

// IsOIDCKey reports whether one storage key is part of the coupled OIDC client.
func IsOIDCKey(key string) bool {
	for _, candidate := range oidcSettingKeys {
		if candidate == key {
			return true
		}
	}
	return false
}

// IsSAMLKey reports whether one storage key is part of the coupled SAML provider.
func IsSAMLKey(key string) bool {
	for _, candidate := range samlSettingKeys {
		if candidate == key {
			return true
		}
	}
	return false
}

// ValidateOIDC enforces all-or-none coupled OIDC configuration.
func ValidateOIDC(value func(string) string) error {
	configured := false
	for _, key := range oidcRequiredKeys {
		configured = configured || value(key) != ""
	}
	if !configured {
		return nil
	}
	for _, key := range oidcRequiredKeys {
		if value(key) == "" {
			return errors.New(key + " is required when OpenID Connect is configured")
		}
	}
	return nil
}

// ValidateSAML enforces one metadata source at most.
func ValidateSAML(value func(string) string) error {
	metadataURL, metadataXML := value(SAMLMetadataURLKey), value(SAMLMetadataXMLKey)
	if metadataURL != "" && metadataXML != "" {
		return errors.New("configure one SAML metadata URL or XML document")
	}
	return nil
}

// ValidIdentityURL accepts HTTPS and explicit loopback HTTP without credentials, query, or fragment.
func ValidIdentityURL(raw string) bool {
	if !boundedURLText(raw) {
		return false
	}
	endpoint, err := url.Parse(raw)
	return err == nil && TrustedURL(endpoint)
}

// BoundedIdentityText rejects oversized, malformed, and control-containing identity configuration.
func BoundedIdentityText(raw string, maximum int) bool {
	if len(raw) > maximum || !utf8.ValidString(raw) {
		return false
	}
	for _, character := range raw {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

// ValidIdentityField validates an OIDC claim or SAML attribute name.
func ValidIdentityField(raw string) bool {
	if raw == "" || len(raw) > 256 || strings.TrimSpace(raw) != raw || !utf8.ValidString(raw) {
		return false
	}
	for _, character := range raw {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

// ValidSAMLMetadata verifies metadata structure, a trusted supported endpoint, and a parseable certificate.
func ValidSAMLMetadata(raw string) bool { //nolint:cyclop // Metadata structure, endpoint, and certificate validation stay together.
	if raw == "" {
		return true
	}
	if len(raw) > maxSAMLMetadata || !utf8.ValidString(raw) {
		return false
	}
	metadata, err := samlsp.ParseMetadata([]byte(raw))
	if err != nil || metadata.EntityID == "" || len(metadata.EntityID) > 2048 || len(metadata.IDPSSODescriptors) == 0 {
		return false
	}
	hasEndpoint, hasCertificate := false, false
	for _, descriptor := range metadata.IDPSSODescriptors {
		hasEndpoint = hasEndpoint || validSAMLEndpoints(descriptor.SingleSignOnServices)
		hasCertificate = hasCertificate || validSAMLCertificates(descriptor.KeyDescriptors)
		if hasEndpoint && hasCertificate {
			return true
		}
	}
	return false
}

func validSAMLEndpoints(services []saml.Endpoint) bool {
	for _, service := range services {
		if (service.Binding == saml.HTTPRedirectBinding || service.Binding == saml.HTTPPostBinding) && ValidIdentityURL(service.Location) {
			return true
		}
	}
	return false
}

func validSAMLCertificates(keys []saml.KeyDescriptor) bool {
	for _, key := range keys {
		for _, certificate := range key.KeyInfo.X509Data.X509Certificates {
			der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(certificate.Data), ""))
			if err == nil {
				if _, err = x509.ParseCertificate(der); err == nil {
					return true
				}
			}
		}
	}
	return false
}
