package server

import "net/http"

const subtitleErrorHTML = `<!doctype html>
<html lang="{{.Language}}" dir="{{.Direction}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#0b0d0b"><title>{{.Title}} · Kinosail Subtitles</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"></head>
<body class="library-page subtitle-app"><a class="skip" href="#main">Skip to content</a><header class="app-header"><a class="brand-lockup" href="/"><img class="brand-icon" src="/static/icon.svg?v=11" alt=""><h1>Kinosail Subtitles</h1></a></header>
<main id="main" class="subtitle-main" tabindex="-1"><section class="subtitle-empty"><span class="eyebrow">Kinosail Subtitles</span><h2>{{.Title}}</h2><p>{{.Message}}</p><a class="mode" href="/">Return to Subtitles</a></section></main></body></html>`

var subtitleErrorView = newLocalizedTemplate("subtitle-error", subtitleErrorHTML)

type subtitleErrorData struct {
	Language, Direction, Title, Message string
}

func localizedPageError(writer http.ResponseWriter, request *http.Request, title, message string, status int) {
	tag := preferredLanguage(request)
	setLanguageHeaders(writer, tag)
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	_ = subtitleErrorView.Execute(writer, request, subtitleErrorData{Language: tag, Direction: localeCatalog.Direction(tag), Title: localeCatalog.Localize(tag, title), Message: localeCatalog.Localize(tag, message)})
}
