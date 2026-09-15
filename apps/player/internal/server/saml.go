package server

import "github.com/MikeO7/kinosail/packages/federation"

// SAMLConfig configures optional SAML 2.0 Web Browser SSO.
type (
	SAMLConfig = federation.SAMLConfig
	samlLogin  = federation.SAMLHTTP[viewerProfile]
)

func newSAML(config SAMLConfig, profiles *profileStore, mfa ...*oidcLogin) *samlLogin {
	return federation.NewSAMLHTTP(config, profiles.federatedProfiles(), federationWebHooks(profiles), mfa...)
}
