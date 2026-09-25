package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

const itemDetailsHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><title>{{.Title}} · Kinosail Player</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=skeleton-2"><script defer src="/static/main.kinosail.bundle.js?v=12"></script></head><body class="detail-page"><a class="skip" href="#main">Skip to content</a><main id="main" class="detail-shell last-light"><a class="back" href="/?view=movies" data-browse-back>{{icon "back"}} Library</a><section class="media-hero{{if or .Backdrop .ShowBackdrop}} has-media-backdrop{{end}}">{{if or .Backdrop .ShowBackdrop}}<img class="media-backdrop" src="/backdrop/{{.ID}}" alt="" fetchpriority="high">{{end}}{{if or .Artwork .ShowArtwork}}<img class="hero-poster" src="/art/{{.ID}}" alt="" width="400" height="600">{{else}}<div class="hero-poster art" aria-hidden="true">{{icon "play"}}</div>{{end}}<div class="media-hero-copy"><span class="eyebrow">{{if .Show}}{{.Show}}{{else}}Movie{{end}}</span><h1>{{.Title}}</h1><p class="meta-line">{{.Year}}{{if .Rating}} · {{.Rating}}{{end}}{{if .Genres}} · {{.Genres}}{{end}}</p>{{if .Plot}}<p>{{.Plot}}</p>{{end}}<div class="watch-progress" data-watch-progress="{{.ID}}" hidden><span data-watch-remaining></span><progress max="100" value="0" aria-label="Watch progress"></progress></div><div class="hero-actions"><a class="button hero-action" href="/watch/{{.ID}}">{{icon "play"}}{{if .Watched}}Play again{{else if .Resume}}Resume{{else}}Play{{end}}</a><form action="/item/{{.ID}}/list" method="post"><button class="quiet" name="listed" value="{{if .Listed}}false{{else}}true{{end}}">{{if .Listed}}Remove from{{else}}Add to{{end}} My List</button></form></div>{{if .CanDownload}}<details class="hero-downloads"><summary>Downloads</summary><a class="button quiet" href="/download/{{.ID}}">Download original</a>{{if .OfflineQuality}}<form action="/offline/{{.ID}}" method="post"><button class="quiet" name="quality" value="{{.OfflineQuality}}">Prepare offline copy</button></form><a href="/offline-downloads">Manage offline downloads</a>{{end}}</details>{{end}}{{if .Director}}<p>Directed by {{.Director}}</p>{{end}}{{if .Show}}<a href="/?view=shows&amp;q={{urlquery .Show}}">All seasons and episodes</a>{{end}}</div></section></main></body></html>`

var itemDetailsView = newLocalizedTemplate("item-details", itemDetailsHTML)

func showItemDetails(index *libraryIndex, progress *progressStore, lists *listStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RawQuery != "" {
			localizedError(writer, request, "invalid title request", http.StatusBadRequest)
			return
		}
		item, found := visibleItem(request, index, request.PathValue("id"))
		if !found || item.Kind != "video" {
			localizedNotFound(writer, request)
			return
		}
		state, viewer := progress.Get(request, item.ID), currentViewer(request)
		data := struct {
			library.Item
			Listed, Watched, Resume, CanDownload bool
			OfflineQuality                       string
		}{Item: item, Listed: lists.Has(request, item.ID), Watched: state.Watched, Resume: state.Seconds > 0, CanDownload: viewer.Owner || viewer.Downloads}
		if supportsOffline(item) {
			data.OfflineQuality = optimizedDownloadLabel(item)
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = itemDetailsView.Execute(writer, request, data)
	}
}

func saveDetailsList(index *libraryIndex, lists *listStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		action := catalog.SaveList(request, currentViewer(request).ID, func(id string) (library.Item, bool) {
			item, found := visibleItem(request, index, id)
			return item, found && item.Kind == "video"
		}, lists.SetListed)
		if action.Err == nil && !action.NotFound {
			action.Redirect = "/item/" + request.PathValue("id")
		}
		action.Serve(writer, request, localizedError, localizedNotFound)
	}
}
