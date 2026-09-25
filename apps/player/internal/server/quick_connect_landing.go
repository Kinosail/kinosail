package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/quickconnect"
)

var connectLandingView = newLocalizedTemplate("connect-landing", `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Connect your TV · Kinosail Player</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=skeleton-3"><script defer src="/static/connect.js?v=1"></script></head><body class="auth"><main><form><img class="brand-mark" src="/static/icon.svg?v=8" alt=""><span class="eyebrow">Kinosail Player</span><h1>Connect your TV</h1><p>Open your signed-in Player app to review and approve the TV.</p><p>Match this code on your TV: <strong>{{.}}</strong></p><a class="button" data-connect-app data-code="{{.}}" hidden>Open Player app</a><a class="button quiet" href="/quick-connect?code={{.}}">Use browser</a><p>No app? Continue in your browser and sign in if needed. Keep your phone on the same network as your Server.</p></form></main></body></html>`)

func (broker *quickConnectBroker) connectPage(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	code, ok := quickconnect.ReadLinkCode(request)
	if !ok {
		localizedError(writer, request, "Scan the QR code on your TV again, or use its six-digit code in Quick Connect.", http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = connectLandingView.Execute(writer, request, code)
}
