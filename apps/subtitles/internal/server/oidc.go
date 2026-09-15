package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/federation"
)

// OIDCConfig configures optional OpenID Connect authorization-code login.
type (
	OIDCConfig   = federation.OIDCConfig
	oidcIdentity = federation.Identity
	oidcLogin    = federation.OIDCHTTP[viewerProfile]
)

const oidcMFAHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Two-factor authentication · Kinosail Subtitles</title><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="auth"><main><form method="post"><h1>Two-factor authentication</h1><input type="hidden" name="challenge" value="{{.}}"><label>Authentication or recovery code<input autofocus required name="code" autocomplete="one-time-code"></label><button>Finish signing in</button></form></main></body></html>`

var oidcMFAView = newLocalizedTemplate("oidc-mfa", oidcMFAHTML)

func newOIDC(config OIDCConfig, profiles *profileStore) *oidcLogin {
	return federation.NewOIDCHTTP(config, profiles.federatedProfiles(), federationWebHooks(profiles), func(writer http.ResponseWriter, request *http.Request, challenge string) error {
		return oidcMFAView.Execute(writer, request, challenge)
	})
}
