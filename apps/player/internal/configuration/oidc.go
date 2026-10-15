package configuration

import "github.com/MikeO7/kinosail/packages/federation"

func isOIDCKey(key string) bool { return federation.IsOIDCKey(key) }

func SetOIDC(dataDir, issuer, clientID, secret, redirectURL, identityClaim string) error {
	config := federation.OIDCConfig{Issuer: issuer, ClientID: clientID, ClientSecret: secret, RedirectURL: redirectURL, IdentityClaim: identityClaim}
	return federationSettings(dataDir).SetOIDC(config)
}

func DeleteOIDC(dataDir string) error { return federationSettings(dataDir).DeleteOIDC() }
