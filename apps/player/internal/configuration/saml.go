package configuration

import (
	"errors"
	"fmt"

	"github.com/MikeO7/kinosail/packages/federation"
)

func isSAMLKey(key string) bool { return federation.IsSAMLKey(key) }

func coupledSettingError(key string) error {
	switch {
	case isOIDCKey(key):
		return errors.New("OpenID Connect settings must be changed together")
	case isSAMLKey(key):
		return errors.New("SAML settings must be changed together")
	case key == scimTokenKey || key == scimExpirationKey:
		return fmt.Errorf("%s and %s must be changed together", scimTokenKey, scimExpirationKey)
	}
	return nil
}

// SetSAML changes the provider metadata source as one validated operation.
func SetSAML(dataDir, metadataURL, metadataXML, identityAttribute string) error {
	return federationSettings(dataDir).SetSAML(federation.SAMLConfig{MetadataURL: metadataURL, MetadataXML: metadataXML, IdentityAttribute: identityAttribute})
}

// DeleteSAML removes the provider metadata source as one operation.
func DeleteSAML(dataDir string) error { return federationSettings(dataDir).DeleteSAML() }
