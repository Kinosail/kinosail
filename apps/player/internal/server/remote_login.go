package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/quickconnect"
)

var publicLoginView = newLocalizedTemplate("public-login", quickconnect.BrowserHTML)

func (auth *authentication) publicLogin(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_ = publicLoginView.Execute(writer, request, quickconnect.BrowserPage{
		Product: "Kinosail Player", Next: safeLoginReturn(request.URL.Query().Get("next")),
		OIDC: auth.oidc, SAML: auth.saml,
	})
}
