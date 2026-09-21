package server

func (auth *authentication) configureFederation(oidcConfig OIDCConfig, samlConfig SAMLConfig) (*oidcLogin, *samlLogin) {
	sso := newOIDC(oidcConfig, auth.profiles)
	saml := newSAML(samlConfig, auth.profiles, sso)
	auth.oidc, auth.saml = sso.Configured(), saml.Configured()
	auth.sso = auth.oidc || auth.saml
	return sso, saml
}
