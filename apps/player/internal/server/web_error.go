package server

import (
	"net/http"
	"net/url"
	"strings"
)

var requestErrorView = newLocalizedTemplate("request-error", `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Could not complete the request · Kinosail Player</title><script src="/static/theme.js?v=electric-1"></script><script defer src="/static/main.kinosail.bundle.js?v=12"></script><link rel="stylesheet" href="/static/app.css?v=skeleton-3"></head><body class="auth"><a class="skip" href="#main">Skip to content</a><main id="main" class="grant-card"><img class="brand-mark" src="/static/icon.svg?v=8" alt=""><h1>Could not complete the request</h1><p role="alert">{{.Message}}</p><button type="button" data-return-to-form hidden>Return to form</button><a class="button quiet" href="{{.ReturnPath}}">Open previous page</a><a href="/">Library</a></main></body></html>`)

var loginErrorView = newLocalizedTemplate("login-error", strings.NewReplacer(
	`data-login-next="{{.}}"`, `data-login-next="{{.Next}}"`,
	`<label>Name<input`, `<p role="alert">{{.Message}}</p><label>Name<input`,
	`name="name" autocomplete`, `name="name" value="{{.Name}}" maxlength="120" autocomplete`,
).Replace(profileLoginHTML))

func writeWebError(writer http.ResponseWriter, request *http.Request, message string, status int) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	if request.URL.Path == "/login" && request.Method == http.MethodPost {
		name := request.PostForm.Get("name")
		if len(name) > 120 || strings.ContainsAny(name, "\r\n\x00") {
			name = ""
		}
		_ = loginErrorView.Execute(writer, request, struct{ Message, Name, Next string }{message, name, safeLoginReturn(request.URL.Query().Get("next"))})
		return
	}

	_ = requestErrorView.Execute(writer, request, struct{ Message, ReturnPath string }{message, webErrorReturnPath(request)})
}

func webErrorReturnPath(request *http.Request) string {
	back := "/"
	if len(request.Referer()) <= 2048 {
		if referrer, err := url.Parse(request.Referer()); err == nil && referrer.Host == request.Host && referrer.User == nil && (referrer.Scheme == "http" || referrer.Scheme == "https") {
			back = safeLoginReturn(referrer.RequestURI())
		}
	}
	return back
}
