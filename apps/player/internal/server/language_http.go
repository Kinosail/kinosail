package server

import "net/http"

const languageHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#0b0d0b"><link rel="icon" href="/static/icon.svg?v=8"><title>Language · Kinosail Player</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="settings-page"><main class="settings-shell"><a class="back" href="/">{{icon "back"}} Kinosail</a><header class="settings-intro"><span class="eyebrow">Language</span><h1>Language</h1><p>Choose the language for this browser.</p></header><section class="wide"><h2>Language</h2>{{languagePicker}}</section></main></body></html>`

var languageView = newLocalizedTemplate("language", languageHTML)

func showLanguage(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := languageView.Execute(writer, request, nil); err != nil {
		localizedError(writer, request, "language page failed", http.StatusInternalServerError)
	}
}
