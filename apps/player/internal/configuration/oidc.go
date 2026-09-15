package configuration

import "github.com/MikeO7/kinosail/packages/federation"

func isOIDCKey(key string) bool { return federation.IsOIDCKey(key) }

// SetOIDC changes the complete OpenID Connect client as one validated operation.
func SetOIDC(dataDir, issuer, clientID, secret, redirectURL, identityClaim string) error {
	config := federation.OIDCConfig{Issuer: issuer, ClientID: clientID, ClientSecret: secret, RedirectURL: redirectURL, IdentityClaim: identityClaim}
	return federationSettings(dataDir).SetOIDC(config)
}

// DeleteOIDC removes the complete OpenID Connect client as one operation.
func DeleteOIDC(dataDir string) error { return federationSettings(dataDir).DeleteOIDC() }
