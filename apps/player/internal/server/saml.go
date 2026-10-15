package server

import "github.com/MikeO7/kinosail/packages/federation"

type (
	SAMLConfig = federation.SAMLConfig
	samlLogin  = federation.SAMLHTTP[viewerProfile]
)

func newSAML(config SAMLConfig, profiles *profileStore, mfa ...*oidcLogin) *samlLogin {
	return federation.NewSAMLHTTP(config, profiles.federatedProfiles(), federationWebHooks(profiles), mfa...)
}
