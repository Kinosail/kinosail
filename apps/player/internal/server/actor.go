package server

import (
	"encoding/json"
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalogapi"
)

const actorHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>{{.Name}} · Kinosail</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="detail-page"><main class="detail-shell"><a class="back" href="/">{{icon "back"}} Library</a><section class="media-hero"><div class="hero-poster art">{{if .Image}}<img class="poster" src="{{.Image}}" alt="" loading="eager">{{else}}{{icon "play"}}{{end}}</div><div class="media-hero-copy"><span class="eyebrow">Cast</span><h1>{{.Name}}</h1><p>In your library</p></div></section>{{if .Movies}}<section class="detail-section"><header><h2>Movies</h2></header><div class="grid">{{range .Movies}}{{template "credit" .}}{{end}}</div></section>{{end}}{{if .Shows}}<section class="detail-section"><header><h2>Shows</h2></header><div class="grid">{{range .Shows}}{{template "credit" .}}{{end}}</div></section>{{end}}{{if not (or .Movies .Shows)}}<p>No matching titles are available in your library.</p>{{end}}</main></body></html>{{define "credit"}}<a class="card" href="{{.URL}}">{{if .Artwork}}<img class="poster" src="{{.Artwork}}" alt="" loading="lazy">{{else}}<div class="poster art" aria-hidden="true">{{icon "play"}}</div>{{end}}<h3>{{.Title}}</h3>{{if .Year}}<small>{{.Year}}</small>{{end}}{{if .Role}}<p>{{.Role}}</p>{{end}}</a>{{end}}`

var actorView = newLocalizedTemplate("actor", actorHTML)

func browseActor(index *libraryIndex, api bool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		name, err := catalogapi.ActorQuery(request.URL.RawQuery)
		if err != nil {
			localizedError(writer, request, "actor is invalid", http.StatusBadRequest)
			return
		}
		items, _ := visibleLibrary(request, index)
		page, err := catalogapi.ActorLibrary(items, name)
		if err != nil {
			localizedError(writer, request, "actor is invalid", http.StatusBadRequest)
			return
		}
		if api {
			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(page)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := actorView.Execute(writer, request, page); err != nil {
			localizedError(writer, request, "actor could not be displayed", http.StatusInternalServerError)
		}
	}
}
